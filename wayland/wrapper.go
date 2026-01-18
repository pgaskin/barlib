package wayland

import (
	"os"

	"codeberg.org/tesselslate/wl"
)

// The wl library isn't really documented, so I kind of had to reverse-engineer
// the intended usage of the api by comparing it to the C wayland bindings and
// by reading the code.
//
// [wl.Object] is a handle (it contains a pointer to the actual object data), a
// zero handle WILL cause a nil-deref when using any of the members, and some of
// the methods require a pointer receiver, so it's better to just store pointers
// to them (the api design leaves something to be desired...).
//
// It looks like it's safe to call flush and methods on objects while dispatch
// is running, but not concurrently with other things which touch the write
// queue (it could be safe if the queue was made goroutine-safe with atomics),
// so we need a mutex for all of our own callbacks which could touch the write
// queue (i.e., if they call flush or if they call methods on objects).

// Connection attempts to wrap the main loop of the wl library in a slightly
// more goroutine-safe way. All methods on objects (including within callbacks
// even though those are run within the main loop) must be called within
// [Connection.Do], which waits on any other callbacks and blocks the main loop,
// or [Connection.Enqueue], which executes on the main goroutine after all other
// events have been processed. All errors returned are treated as fatal and will
// close the conection.
//
// This wouldn't have been necessary if the wl library provided a way to
// interrupt [wl.Display.Dispatch], but I don't feel like forking it yet.
type Connection struct {
	done      chan struct{}
	closed    chan struct{}
	closedErr error
	mu        chan struct{} // protects the write queue on dpy (chan instead of plain mutex so we can wait on closed too)
	display   *wl.Display
}

func Connect(name string) (*Connection, error) {
	display, err := wl.NewDisplay(name)
	if err != nil {
		return nil, err
	}

	c := &Connection{
		done:      make(chan struct{}),
		closed:    make(chan struct{}),
		closedErr: nil,
		mu:        make(chan struct{}, 1),
		display:   display,
	}
	go c.run()

	c.mu <- struct{}{}

	return c, nil
}

func (c *Connection) run() {
	defer close(c.done)
	for {
		// flush any queued messages
		if err := c.Do(func() error {
			return nil
		}); err != nil {
			return // Do will have already called closeWithError
		}
		// read and displatch messages
		if err := c.display.Dispatch(); err != nil {
			c.closeWithError(err)
			return
		}
	}
}

func (c *Connection) Registry(cb wl.RegistryListener) error {
	return c.Do(func() error {
		registry := c.display.GetRegistry()
		registry.SetListener(cb, nil)
		return nil
	})
}

// Do runs the provided function while blocking the main loop and any other
// calls to [Connection.Do]. It is not re-entrant and must not be called within
// another call to [Connection.Do] or [Connection.Enqueue]. If an error is
// returned, it is fatal and the connection will be closed.
func (c *Connection) Do(fn func() error) error {
	select {
	case <-c.closed:
		if err := c.closedErr; err != nil {
			return err
		}
		return os.ErrClosed
	case <-c.mu: // lock
	}
	if err := fn(); err != nil {
		c.closeWithErrorLocked(err)
		return err
	}
	if err := c.display.Flush(); err != nil {
		c.closeWithErrorLocked(err)
		return err
	}
	c.mu <- struct{}{} // unlock
	return nil
}

// Enqueue waits for all events to be processed, then executes the provided
// function on the main loop, blocking it. If an error is returned, it is fatal
// and the connection will be closed.
func (c *Connection) Enqueue(fn func() error) error {
	done := make(chan struct{})

	if err := c.Do(func() error {
		// we use an async callback to ensure we've already processed all events so far
		cb := c.display.Sync()
		cb.SetListener(wl.CallbackListener{
			Done: func(data any, self wl.Callback, callbackData uint32) error {
				defer close(done)
				return c.Do(fn)
			},
		}, nil)
		return nil
	}); err != nil {
		return err
	}

	<-done
	return nil
}

// Close closes the connection if it is not already closed, interrupting any
// operations, and waits for any pending callbacks to complete and the main loop
// to return.
func (c *Connection) Close() {
	c.closeWithError(nil)
	<-c.done
}

func (c *Connection) closeWithError(err error) {
	select {
	case <-c.closed:
		return
	case <-c.mu: // lock
		// note: don't unlock it again after so the closed chan is always selected
	}
	c.closeWithErrorLocked(err)
}

// closeWithErrorLocked closes the display if not already closed, setting the
// sticky error to err or a generic error message.
func (c *Connection) closeWithErrorLocked(err error) {
	select {
	case <-c.closed:
		return
	default:
	}
	defer func() {
		c.closedErr = err
		close(c.closed)
	}()
	c.display.Close()
}

// Closed returns when the connection is closed. If the connection was not
// closed by [Connection.Close], the fatal error is returned.
func (c *Connection) Closed() error {
	<-c.closed
	return c.closedErr
}
