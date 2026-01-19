package redshift

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/randr"
	"github.com/BurntSushi/xgb/xproto"
)

// xManager manages gamma ramps for X11 displays using RandR. It is safe for
// concurrent usage.
type xManager struct {
	conn   *xgb.Conn
	errch  chan error
	logger *slog.Logger

	root xproto.Window

	wmu   sync.Mutex
	white *WhitePoint
}

// NewX11 opens a X11 connection to the specified display (empty for the
// default), processing events in another goroutine. If a fatal error occurs,
// the chan will return it, and the connection should be closed as it will no
// longer be usable. If logger is not nil, it is used for debug logs from this
// package.
func NewX11(display string, logger *slog.Logger) (Manager, <-chan error, error) {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	conn, err := xgb.NewConnDisplay(display)
	if err != nil {
		return nil, nil, err
	}

	e := make(chan error, 1)
	m := &xManager{conn: conn, errch: e, logger: logger}

	m.root = xproto.Setup(m.conn).DefaultScreen(conn).Root

	if err := randr.Init(conn); err != nil {
		conn.Close()
		return nil, nil, err
	}

	if err := randr.SelectInputChecked(m.conn, m.root, randr.NotifyMaskCrtcChange).Check(); err != nil {
		conn.Close()
		return nil, nil, err
	}

	go func() {
		for {
			e, err := m.conn.WaitForEvent()
			if err != nil {
				m.errch <- err
				return
			}
			switch e := e.(type) {
			case randr.NotifyEvent:
				if e.SubCode == randr.NotifyCrtcChange {
					m.apply()
				}
			}
		}
	}()

	return m, e, nil
}

func (m *xManager) Close() {
	m.conn.Close()
}

func (m *xManager) Set(white WhitePoint) {
	func() {
		m.wmu.Lock()
		defer m.wmu.Unlock()
		m.white = new(white)
	}()

	m.apply()
}

func (m *xManager) apply() {
	white := func() *WhitePoint {
		m.wmu.Lock()
		defer m.wmu.Unlock()
		return m.white
	}()
	if white == nil {
		return // not ready
	}

	resources, err := randr.GetScreenResourcesCurrent(m.conn, m.root).Reply()
	if err != nil {
		m.logger.Error("x11: randr: failed to get screen resources", "error", err)
		return
	}

	for _, crtc := range resources.Crtcs {
		if err := SetX11(m.conn, crtc, *white); err != nil {
			m.logger.Warn("x11: randr: failed to set color ramp", "crtc", crtc, "error", err)
		}
	}
}

// SetX11 applies a color ramp to the specified CRTC. The RandR
// extension must be initialized.
func SetX11(conn *xgb.Conn, crtc randr.Crtc, white WhitePoint) error {
	gamma, err := randr.GetCrtcGammaSize(conn, crtc).Reply()
	if err != nil {
		return fmt.Errorf("get crtc gamma size: %w", err)
	}
	gr := make([]uint16, gamma.Size)
	gg := make([]uint16, gamma.Size)
	gb := make([]uint16, gamma.Size)
	GammaRamp(gr, gg, gb, white)
	if err := randr.SetCrtcGammaChecked(conn, crtc, gamma.Size, gr, gg, gb).Check(); err != nil {
		return fmt.Errorf("set crtc gamma: %w", err)
	}
	return nil
}
