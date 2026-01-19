//go:build unix

package redshift

import (
	"fmt"
	"log/slog"
	"unsafe"

	"github.com/friedelschoen/wayland"
	"github.com/pgaskin/barlib/wlproto"
	"golang.org/x/sys/unix"
)

// wlManager manages gamma ramps for Wayland displays using
// wlr_gamma_control_unstable_v1. It is safe for concurrent usage.
type wlManager struct {
	conn   *wayland.Conn
	errch  chan error
	logger *slog.Logger

	display  *wlproto.WlDisplay
	registry *wlproto.WlRegistry
	manager  *wlproto.ZwlrGammaControlManagerV1 // may be nil
	outputs  map[uint32]*wlOutputState

	white *WhitePoint
}

type wlOutputState struct {
	object  uint32
	output  *wlproto.WlOutput
	control *wlproto.ZwlrGammaControlV1 // may be nil
	size    uint32                      // may be zero

	white *WhitePoint
}

// NewWayland opens a Wayland connection to the specified display (empty for the
// default), processing events in another goroutine. If a connection error is
// encountered after the initial connection, it may be logged with [log.Printf]
// by the underlying library. If a fatal error occurs, the chan will return it,
// and the connection should be closed as it will no longer be usable. If logger
// is not nil, it is used for debug logs from this package.
func NewWayland(display string, logger *slog.Logger) (Manager, <-chan error, error) {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	conn, err := wayland.Connect(display)
	if err != nil {
		return nil, nil, err
	}

	// note: Connect starts a goroutine to dispatch events, which are all called
	// synchronously, so as long as we do everything in event handlers, we don't
	// need to worry about locking or consistency (and message writes are
	// goroutine-safe since they lock the socket)

	e := make(chan error, 1)
	m := &wlManager{conn: conn, errch: e, logger: logger}

	m.display = wlproto.NewWlDisplay(&wlproto.WlDisplayHandlers{
		OnError: wlHandler(func(e *wlproto.WlDisplayErrorEvent) bool {
			if m.errch != nil {
				defer close(m.errch)
				m.errch <- fmt.Errorf("object %d: error %d: %s", e.ObjectID().ID(), e.Code(), e.Message())
				m.errch = nil
			}
			return true
		}),
		OnDeleteID: wlHandler(func(e *wlproto.WlDisplayDeleteIDEvent) bool {
			return m.conn.UnregisterEvent(e)
		}),
	})
	m.conn.Register(m.display)

	m.manager = nil
	m.outputs = make(map[uint32]*wlOutputState)

	// would use [wayland.Registrar], but it doesn't handle multiple instances
	// of objects and notifying when they're added or removed
	m.registry = m.display.GetRegistry(&wlproto.WlRegistryHandlers{
		OnGlobal: wlHandler(func(e *wlproto.WlRegistryGlobalEvent) bool {
			m.logger.Debug("registry: object added", "object", e.Name(), "interface", e.Interface(), "version", e.Version())

			if e.Interface() == new(wlproto.ZwlrGammaControlManagerV1).Name() {
				if m.manager != nil {
					m.logger.Warn("registry: wtf: got multiple gamma control managers?")
					return true
				}

				m.manager = wlproto.NewZwlrGammaControlManagerV1()
				m.conn.Register(m.manager)
				m.registry.Bind(e.Name(), e.Interface(), e.Version(), m.manager)
				m.logger.Info("registry: bound gamma control manager")

				for _, s := range m.outputs {
					m.bindControl(s)
					m.applyOutput(s)
				}
				return true
			}

			if e.Interface() == new(wlproto.WlOutput).Name() {
				s := new(wlOutputState)
				s.object = e.Name()
				s.output = wlproto.NewWlOutput(&wlproto.WlOutputHandlers{})
				m.conn.Register(s.output)
				m.outputs[e.Name()] = s
				m.registry.Bind(e.Name(), e.Interface(), e.Version(), s.output)
				m.logger.Info("registry: detected new output", "object", e.Name())

				m.bindControl(s)
				m.applyOutput(s)
			}

			return true
		}),
		OnGlobalRemove: wlHandler(func(e *wlproto.WlRegistryGlobalRemoveEvent) bool {
			m.logger.Debug("registry: object removed", "object", e.Name())
			if m.manager != nil && m.manager.ID() == e.Name() {
				m.manager.Destroy()
				m.manager = nil
				m.logger.Warn("registry: lost gamma control manager")
				return true
			}
			if s, ok := m.outputs[e.Name()]; ok {
				if s.control != nil {
					s.control.Destroy()
					s.control = nil
				}
				if s.output != nil {
					s.output.Destroy()
					s.output = nil
				}
				delete(m.outputs, e.Name())
				m.logger.Info("registry: output removed", "object", e.Name())
				return true
			}
			return true
		}),
	})

	m.sync(nil)

	return m, e, nil
}

func (m *wlManager) Close() {
	m.conn.Close()
}

// sync waits for all requests to complete and events to be processed, then
// calls fn, if non-nil, in the event goroutine, waiting for it to complete.
func (m *wlManager) sync(fn func()) {
	done := make(chan struct{})
	m.display.Sync(&wlproto.WlCallbackHandlers{
		OnDone: wayland.EventHandlerFunc[*wlproto.WlCallbackDoneEvent](func(_ *wlproto.WlCallbackDoneEvent) bool {
			if fn != nil {
				fn()
			}
			done <- struct{}{}
			return true
		}),
	})
	<-done
}

func (m *wlManager) Set(white WhitePoint) {
	m.sync(func() {
		m.white = new(white)
		for _, s := range m.outputs {
			m.applyOutput(s)
		}
	})
}

func (m *wlManager) bindControl(s *wlOutputState) {
	if m.manager == nil {
		return // not ready yet
	}
	if s.control != nil {
		return // already have it
	}
	s.control = m.manager.GetGammaControl(s.output, &wlproto.ZwlrGammaControlV1Handlers{
		OnGammaSize: wlHandler(func(e *wlproto.ZwlrGammaControlV1GammaSizeEvent) bool {
			m.logger.Info("output: got gamma control", "object", s.object, "size", e.Size())
			s.size = e.Size()
			m.applyOutput(s)
			return true
		}),
		OnFailed: wlHandler(func(e *wlproto.ZwlrGammaControlV1FailedEvent) bool {
			s.control.Destroy()
			s.control = nil
			m.logger.Warn("output: lost gamma control (was the output removed or is another application controlling it?)", "object", s.object)
			return true
		}),
	})
	m.logger.Info("output: getting gamma control", "object", s.object)
}

func (m *wlManager) applyOutput(s *wlOutputState) {
	if m.white == nil || s.control == nil || s.size == 0 {
		return // not ready yet
	}
	if s.white != nil && s.white == m.white {
		return // already attempted to apply this ramp (note: control is exclusive, so nothing could have changed it behind our backs)
	}
	s.white = m.white

	SetWayland(s.control, s.size, *s.white)
}

func SetWayland(control *wlproto.ZwlrGammaControlV1, size uint32, white WhitePoint) {
	g := make([]uint16, size*3)
	GammaRamp(g[size*0:size*1], g[size*1:size*2], g[size*2:size*3], white)

	fd, err := unix.MemfdCreate("gammaramp", unix.MFD_CLOEXEC)
	if err != nil {
		panic(err)
	}
	defer unix.Close(fd) // the SetGamma call blocks on sending the fd, so this is fine

	if _, err := unix.Pwrite(fd, sliceBytes(g), 0); err != nil {
		panic(err)
	}
	control.SetGamma(fd)
}

func sliceBytes[T any](s []T) []byte {
	d := unsafe.SliceData(s)
	e := unsafe.Sizeof(*d)
	return unsafe.Slice((*byte)(unsafe.Pointer(d)), cap(s)*int(e))[:len(s)*int(e)]
}

func wlHandler[T wayland.Event](fn func(T) bool) wayland.EventHandler {
	return wayland.EventHandlerFunc[T](fn)
}
