// Package barlib is a simple but flexible library which allows you to implement
// efficient, fast, responsive, and error-tolerant i3status replacements in Go.
package barlib

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/pgaskin/barlib/barproto"
)

// Module is a single immediate-mode status bar module with its own main loop
// and state. The struct implementing Module should contain stateless read-only
// configuration, and all state should be contained within Run.
type Module interface {
	// Run contains the main loop for the module, running indefinitely and
	// returning an error if a fatal error occurs.
	Run(Instance) error
}

// ModuleFunc wraps a function in a Module.
type ModuleFunc func(Instance) error

func (fn ModuleFunc) Run(instance Instance) error {
	return fn(instance)
}

// Instance provides per-instance functions to interact with the bar.
type Instance interface {
	// Tick returns a divided ticker channel, which tries to synchronize ticks
	// for all instances to reduce bar updates (the bar waits for a short period
	// of time after each tick before re-rendering the bar -- this is currently
	// 25ms). The provided duration is not exact must be a multiple of the bar's
	// base tick rate. If zero, the ticker does not tick. The returned channel
	// has a buffer size of 1 (i.e., missed ticks will trigger as soon as
	// possible, but only one missed tick at a time).
	Tick(interval time.Duration) <-chan uint64

	// TickReset updates the interval for a divided ticker channel.
	TickReset(s <-chan uint64, interval time.Duration)

	// Update builds and submits an update for the bar. The renderer must only
	// be used within the function. If now is true, the new bar will be drawn
	// immediately instead of attempting to coalesce draws. The Block.Name field
	// is used internally and will be overridden.
	Update(now bool, fn func(render Renderer))

	// IsStopped checks whether the bar is currently stopped. This is just a
	// hint, and doesn't need to be followed.
	IsStopped() bool

	// Event gets the event channel. Up to 16 events are buffered.
	Event() <-chan barproto.Event

	// Stopped gets a channel which notifies when IsStopped changes. The buffer
	// size is 1 since the actual value is read from IsStopped.
	Stopped() <-chan struct{}

	// Debug writes debug logs.
	Debug(format string, a ...any)
}

// Renderer renders raw blocks while also providing high-level wrappers for
// common block layouts.
type Renderer func(barproto.Block)

// Err renders an error message block.
func (r Renderer) Err(err error) {
	var s string
	if err != nil {
		s = err.Error()
	} else {
		s = "<nil>"
	}
	r(barproto.Block{
		FullText:     " error: " + s + " ",
		ShortText:    "ERR",
		Urgent:       true,
		Separator:    true,
		Background:   0xFF0000FF,
		BorderTop:    -1,
		BorderLeft:   -1,
		BorderBottom: -1,
		BorderRight:  -1,
	})
}

type instanceImpl struct {
	name       string
	invalidate func(now bool)
	ticker     *tickDivider

	// notify
	eventCh   chan barproto.Event
	stoppedCh chan struct{}
	tickersCh chan struct{}

	// stopped state
	stopped atomic.Bool

	// last renderer output
	buf1m sync.Mutex
	buf1b []byte

	// renderer output
	buf2m sync.Mutex
	buf2b []byte
}

func instantiate(m Module, name string, ticker *tickDivider, invalidate func(now bool)) *instanceImpl {
	instance := &instanceImpl{
		name:       name,
		invalidate: invalidate,
		ticker:     ticker,
		eventCh:    make(chan barproto.Event, 16),
		stoppedCh:  make(chan struct{}, 1),
		tickersCh:  make(chan struct{}),
	}
	go func() {
		for {
			err := func() (err error) {
				defer func() {
					if p := recover(); p != nil {
						err = fmt.Errorf("panic: %v", p)
					}
				}()
				return m.Run(instance)
			}()
			if err == nil {
				break
			}
			// stop the tickers
			close(instance.tickersCh)
			instance.tickersCh = make(chan struct{})
			// drain events
			for {
				select {
				case <-instance.eventCh:
					continue
				default:
				}
				break
			}
			// show the error
			instance.Update(true, func(r Renderer) {
				r.Err(fmt.Errorf("fatal: %w", err))
			})
			// wait for a click before recreating the instance
			<-instance.eventCh
		}
	}()
	return instance
}

func (i *instanceImpl) Tick(interval time.Duration) <-chan uint64 {
	return i.ticker.Tick(i.tickersCh, interval)
}

func (i *instanceImpl) TickReset(s <-chan uint64, interval time.Duration) {
	i.ticker.Reset(s, interval)
}

func (i *instanceImpl) Update(now bool, fn func(Renderer)) {
	i.buf2m.Lock()
	defer i.buf2m.Unlock()

	i.buf2b = i.buf2b[:0]
	fn(Renderer(func(b barproto.Block) {
		b.Name = i.name
		i.buf2b = b.AppendJSON(append(i.buf2b, ','))
	}))

	i.buf1m.Lock()
	defer i.buf1m.Unlock()

	i.buf1b, i.buf2b = i.buf2b, i.buf1b

	if !bytes.Equal(i.buf1b, i.buf2b) {
		i.invalidate(now)
	}
}

func (i *instanceImpl) IsStopped() bool {
	return i.stopped.Load()
}

func (i *instanceImpl) Event() <-chan barproto.Event {
	return i.eventCh
}

func (i *instanceImpl) Stopped() <-chan struct{} {
	return i.stoppedCh
}

func (i *instanceImpl) Debug(format string, a ...any) {
	fmt.Fprintln(os.Stderr, "debug: "+i.name+": "+fmt.Sprintf(format, a...))
}

func (i *instanceImpl) SendEvent(event barproto.Event) {
	if event.Name == i.name {
		select {
		case i.eventCh <- event:
		default:
		}
	}
}

func (i *instanceImpl) SendStopped(stopped bool) {
	i.stopped.Store(stopped)
	select {
	case i.stoppedCh <- struct{}{}:
	default:
	}
}

func (i *instanceImpl) WriteTo(w io.Writer, comma bool) bool {
	i.buf1m.Lock()
	defer i.buf1m.Unlock()

	if len(i.buf1b) <= 1 {
		return comma
	}

	var err error
	if comma {
		_, err = w.Write(i.buf1b)
	} else {
		_, err = w.Write(i.buf1b[1:])
	}
	if err != nil {
		panic(err)
	}
	return true
}

const tickDividerStrict = true

type tickDivider struct {
	b time.Duration                   // base interval
	c chan struct{}                   // cancel channel
	s map[chan<- uint64]uint64        // map of sub-tickers to multiple of base interval
	r map[<-chan uint64]chan<- uint64 // map of sub-tickers to themselves
	m sync.Mutex                      // lock for sub-ticker map
}

func newTickDivider(base time.Duration) *tickDivider {
	c := make(chan struct{})
	d := &tickDivider{
		b: base,
		c: c,
		s: make(map[chan<- uint64]uint64),
		r: make(map[<-chan uint64]chan<- uint64),
	}
	go func() {
		t := time.NewTicker(base)
		defer t.Stop()
		var n uint64
		for {
			select {
			case <-t.C:
				d.m.Lock()
				for s, i := range d.s {
					if i != 0 && n%i == 0 {
						select {
						case s <- n:
						default:
							// tick missed
						}
					}
				}
				d.m.Unlock()
				n++
			case <-c:
				d.m.Lock()
				for s1, s2 := range d.r {
					delete(d.r, s1)
					delete(d.s, s2)
				}
				d.s = nil
				d.m.Unlock()
				return
			}
		}
	}()
	return d
}

func (d *tickDivider) Base() time.Duration {
	return d.b
}

func (d *tickDivider) Tick(cancel <-chan struct{}, interval time.Duration) <-chan uint64 {
	n := d.interval(interval)
	s := make(chan uint64, 1)
	d.m.Lock()
	if d.s != nil {
		d.s[s] = n
		d.r[s] = s
		if cancel != nil {
			go func() {
				<-cancel
				d.m.Lock()
				delete(d.s, s)
				delete(d.r, s)
				d.m.Unlock()
			}()
		}
	}
	d.m.Unlock()
	return s
}

func (d *tickDivider) Reset(s <-chan uint64, interval time.Duration) {
	n := d.interval(interval)
	d.m.Lock()
	if d.s != nil {
		if s, ok := d.r[s]; ok {
			d.s[s] = n
		}
	}
	d.m.Unlock()
}

func (d *tickDivider) interval(interval time.Duration) uint64 {
	if interval < 0 {
		panic(fmt.Errorf("tick interval %s is negative", interval))
	}
	if interval%d.b != 0 {
		if tickDividerStrict {
			panic(fmt.Errorf("tick interval %s is not a multiple of base %s", interval, d.b))
		}
	}
	return uint64((interval + d.b/2) / d.b)
}

func (d *tickDivider) Stop() {
	close(d.c)
}

// Main runs the status bar with the provided modules.
//
// Do not use the Block/Event Name field from the modules; this is used
// internally to differentiate between instantiated modules for events. Use the
// Event Instance field for handling click events on different blocks
// differently.
func Main(tickRate time.Duration, modules ...Module) {
	const (
		restartEnv  = "BARLIB_RESTARTED=1"
		stopSignal  = syscall.SIGUSR1
		contSignal  = syscall.SIGUSR2
		updateDelay = time.Millisecond * 25
	)
	var (
		ticker          = newTickDivider(tickRate)
		delayer         *time.Timer
		instances       = make([]*instanceImpl, len(modules))
		invalidateCh    = make(chan struct{}, 1)
		invalidateNowCh = make(chan struct{}, 1)
	)
	go func() {
		exe, err := os.Executable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "watcher: failed to watch own binary: get own path: %v", err)
			return
		}

		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			fmt.Fprintf(os.Stderr, "watcher: failed to watch own binary: create watcher: %v", err)
			return
		}
		defer watcher.Close()

		if err := watcher.Add(exe); err != nil {
			fmt.Fprintf(os.Stderr, "watcher: failed to watch own binary: update watcher: %v", err)
			return
		}
		for {
			select {
			case event, ok := <-watcher.Events:
				if ok && event.Has(fsnotify.Chmod) {
					// go build chmods it at the end of the build
					fmt.Fprintf(os.Stderr, "watcher: got chmod, restarting in 500ms\n")
					time.Sleep(time.Millisecond * 500)
					if err := syscall.Exec(exe, os.Args, append(os.Environ(), restartEnv)); err != nil {
						fmt.Fprintf(os.Stderr, "watcher: restart failed: %v\n", err)
					}
				}
			case err, ok := <-watcher.Errors:
				if ok {
					fmt.Fprintf(os.Stderr, "watcher: warning: %v", err)
				}
			}
		}
	}()
	for i, module := range modules {
		instances[i] = instantiate(module, strconv.Itoa(i), ticker, func(now bool) {
			if now {
				select {
				case invalidateNowCh <- struct{}{}:
				default:
				}
			} else {
				select {
				case invalidateCh <- struct{}{}:
				default:
				}
			}
		})
	}
	go func() {
		sc := bufio.NewScanner(os.Stdin)
		for sc.Scan() {
			buf := sc.Bytes()
			if len(buf) == 0 {
				continue
			}
			if buf[0] == '[' || buf[0] == ',' {
				buf = buf[1:]
			}
			if len(buf) == 0 || buf[0] != '{' || buf[len(buf)-1] != '}' {
				if sc.Text() != "[" {
					fmt.Fprintf(os.Stderr, "warning: invalid event line %q", sc.Text())
				}
				continue
			}
			var event barproto.Event
			event.FromJSON(buf)
			for _, instance := range instances {
				instance.SendEvent(event)
			}
		}
		panic(fmt.Errorf("stdin failed (error: %w)", sc.Err()))
	}()
	go func() {
		sigCh := make(chan os.Signal, 2)
		signal.Notify(sigCh, stopSignal, contSignal)
		for sig := range sigCh {
			switch sig {
			case stopSignal:
				for _, instance := range instances {
					instance.SendStopped(true)
				}
			case contSignal:
				for _, instance := range instances {
					instance.SendStopped(false)
				}
			}
		}
	}()
	if !slices.Contains(os.Environ(), restartEnv) {
		os.Stdout.Write(append(barproto.Init{
			StopSignal:  stopSignal,
			ContSignal:  contSignal,
			ClickEvents: true,
		}.AppendJSON(nil), "\n[[]\n"...))
	}
	for render := false; ; {
		if render {
			select {
			case <-invalidateCh:
				continue
			default:
			}
			select {
			case <-invalidateNowCh:
				continue
			default:
			}
			render = false

			os.Stdout.WriteString(",[")
			var comma bool
			for _, instance := range instances {
				comma = instance.WriteTo(os.Stdout, comma)
			}
			os.Stdout.WriteString("]\n")
		}
		select {
		case <-invalidateNowCh:
			render = true
			continue
		case <-invalidateCh:
			render = true
		}
		if delayer == nil {
			delayer = time.NewTimer(updateDelay)
		} else {
			delayer.Reset(updateDelay)
		}
		select {
		case <-delayer.C:
		case <-invalidateNowCh:
			render = true
		}
	}
}
