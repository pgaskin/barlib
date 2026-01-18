//go:build unix

package redshift

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"codeberg.org/tesselslate/wl"
	"github.com/pgaskin/barlib/wayland"
	"github.com/pgaskin/barlib/wayland/zwlr"
	"golang.org/x/sys/unix"
)

// TODO: debug log output?

// ColorRampWayland connects to the specified wayland display to set the white
// point for all outputs. If update returns an error, it is fatal. Only one
// application may manage the gamma ramps at a time (otherwise it will fail
// silently).
//
//	update, close, errCh, err := redshift.ColorRampWayland("")
//	if err != nil {
//		panic(err)
//	}
//	defer close()
//
//	go func() {
//		if err := <-errCh; err != nil {
//			panic(err)
//		}
//	}()
//
//	for {
//		var temp int
//		if _, err := fmt.Scanf("%d", &temp); err != nil {
//			if err == io.EOF {
//				break
//			}
//			panic(err)
//		}
//		wr, wg, wb, ok := redshift.WhitePoint(temp)
//		if !ok {
//			fmt.Println("bad")
//			continue
//		}
//		update(wr, wg, wb)
//	}
func ColorRampWayland(display string) (update func(whiteR, whiteG, whiteB float32) error, closeConn func(), errCh <-chan error, err error) {
	conn, err := wayland.Connect(display)
	if err != nil {
		return nil, nil, nil, err
	}

	gc := wlGammaControlNew(conn)

	ch := make(chan error)
	go func() {
		ch <- conn.Closed()
	}()
	update = func(whiteR, whiteG, whiteB float32) error {
		return conn.Enqueue(func() error {
			return gc.SetWhitePointLocked(whiteR, whiteG, whiteB)
		})
	}
	closeConn = func() {
		conn.Close()
	}
	return update, closeConn, ch, nil
}

type wlGammaControl struct {
	conn    *wayland.Connection
	manager *zwlr.GammaControlManagerV1
	outputs map[uint32]*wlGammaControlOutput

	wr, wg, wb float32
	wok        bool
}

func wlGammaControlNew(conn *wayland.Connection) *wlGammaControl {
	ctx := &wlGammaControl{
		conn:    conn,
		outputs: make(map[uint32]*wlGammaControlOutput),
	}
	conn.Registry(wl.RegistryListener{
		Global:       ctx.registryGlobal,
		GlobalRemove: ctx.registryGlobalRemove,
	})
	return ctx
}

func (ctx *wlGammaControl) SetWhitePointLocked(wr, wg, wb float32) error {
	ctx.wr, ctx.wg, ctx.wb, ctx.wok = wr, wg, wb, true
	return ctx.applyLocked()
}

func (ctx *wlGammaControl) registryGlobal(data any, self wl.Registry, name uint32, iface string, version uint32) error {
	return ctx.conn.Do(func() error {
		switch iface {
		case zwlr.GammaControlManagerV1Interface.Name:
			//fmt.Println("got gcm")
			ctx.manager = new(zwlr.GammaControlManagerV1(self.Bind(name, &zwlr.GammaControlManagerV1Interface, version)))

		case wl.OutputInterface.Name:
			// defer it to ensure we've had the chance to initialize gcm first
			go ctx.conn.Enqueue(func() error {
				//fmt.Println("got output", name)
				if ctx.manager == nil {
					return errors.New("no gamma control manager")
				}
				octx := wlGammaControlOutputNew(ctx.conn, wl.Output(self.Bind(name, &wl.OutputInterface, version)), *ctx.manager)
				if ctx.wok {
					if err := octx.SetWhitePointLocked(ctx.wr, ctx.wg, ctx.wb); err != nil {
						octx.DestroyLocked()
						return err
					}
				}
				ctx.outputs[name] = octx
				return nil
			})
		}
		return nil
	})
}

func (ctx *wlGammaControl) registryGlobalRemove(data any, self wl.Registry, name uint32) error {
	return ctx.conn.Do(func() error {
		if octx, ok := ctx.outputs[name]; ok {
			// fmt.Println("removing output", name)
			octx.DestroyLocked()
			delete(ctx.outputs, name)
		}
		return nil
	})
}
func (ctx *wlGammaControl) applyLocked() error {
	if ctx.wok {
		for _, octx := range ctx.outputs {
			if err := octx.SetWhitePointLocked(ctx.wr, ctx.wg, ctx.wb); err != nil {
				return err
			}
		}
	}
	return nil
}

type wlGammaControlOutput struct {
	conn    *wayland.Connection
	output  wl.Output
	control *zwlr.GammaControlV1

	ramp       *wlGammaControlRamp
	wr, wg, wb float32
	wok        bool
}

func wlGammaControlOutputNew(conn *wayland.Connection, output wl.Output, manager zwlr.GammaControlManagerV1) *wlGammaControlOutput {
	octx := &wlGammaControlOutput{
		conn:    conn,
		output:  output,
		control: new(manager.GetGammaControl(output)),
	}
	octx.control.SetListener(zwlr.GammaControlV1Listener{
		GammaSize: octx.gammaControlGammaSize,
		Failed:    octx.gammaControlFailed,
	}, nil)
	return octx
}

func (octx *wlGammaControlOutput) SetWhitePointLocked(wr, wg, wb float32) error {
	octx.wr, octx.wg, octx.wb, octx.wok = wr, wg, wb, true
	return octx.applyLocked()
}

func (octx *wlGammaControlOutput) DestroyLocked() {
	if octx.control != nil {
		octx.control.Destroy()
	}
	*octx = wlGammaControlOutput{}
}

func (octx *wlGammaControlOutput) gammaControlGammaSize(data any, self zwlr.GammaControlV1, size uint32) error {
	return octx.conn.Do(func() (err error) {
		//fmt.Println("got gamma ramp size for output", size)
		octx.ramp = nil
		if size == 0 {
			return nil
		}
		octx.ramp, err = wlGammaControlRampNew(int(size))
		if err != nil {
			return fmt.Errorf("create gamma ramp: %w", err)
		}
		return octx.applyLocked()
	})
}

func (octx *wlGammaControlOutput) gammaControlFailed(data any, self zwlr.GammaControlV1) error {
	return octx.conn.Do(func() error {
		//fmt.Println("gamma control failed (is something else already controlling it for this output or was the output removed?)")
		octx.control.Destroy()
		octx.control = nil
		return nil
	})
}

func (octx *wlGammaControlOutput) applyLocked() error {
	if !octx.wok || octx.ramp == nil || octx.control == nil {
		return nil
	}
	if err := octx.ramp.Set(octx.wr, octx.wg, octx.wb); err != nil {
		return fmt.Errorf("set gamma ramp: %w", err)
	}
	octx.ramp.Apply(*octx.control)
	return nil
}

type wlGammaControlRamp struct {
	_    noCopy
	fd   int
	size int
}

func wlGammaControlRampNew(size int) (*wlGammaControlRamp, error) {
	if size < 1 {
		return nil, fmt.Errorf("invalid size")
	}
	fd, err := unix.Open("/dev/shm", unix.O_TMPFILE|unix.O_RDWR|unix.O_EXCL|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("allocate shared memory: %w", err)
	}
	if err := unix.Ftruncate(fd, int64(size)*3*2); err != nil { // [3*size]uint16
		unix.Close(fd)
		return nil, fmt.Errorf("allocate shared memory: %w", err)
	}
	r := &wlGammaControlRamp{
		fd:   fd,
		size: size,
	}
	runtime.SetFinalizer(r, func(r *wlGammaControlRamp) {
		unix.Close(r.fd)
	})
	return r, nil
}

func (r *wlGammaControlRamp) Apply(control zwlr.GammaControlV1) error {
	if _, err := unix.Seek(r.fd, 0, unix.SEEK_SET); err != nil {
		return fmt.Errorf("seek: %w", err)
	}
	control.SetGamma(r.fd) // note: if this fails, zwlr.GammaControlV1Listener.Failed will be called asynchronously
	return nil
}

func (r *wlGammaControlRamp) Set(wr, wg, wb float32) error {
	rr, rg, rb := ColorRamp[uint16](wr, wg, wb, r.size)
	_, err := unix.Pwritev(r.fd, [][]byte{
		unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(rr))), r.size*2),
		unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(rg))), r.size*2),
		unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(rb))), r.size*2),
	}, 0)
	return err
}

type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}
