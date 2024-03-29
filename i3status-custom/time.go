// # time
//
// Renders the current time at the configured interval using the specified
// stdlib layout.
package main

import (
	"time"

	"github.com/pgaskin/barlib"
	"github.com/pgaskin/barlib/barproto"
)

type Time struct {
	LayoutFull  string
	LayoutShort string
	Interval    time.Duration
	Color       uint32
}

func (c Time) Run(i barlib.Instance) error {
	for ticker := i.Tick(c.Interval); ; {
		if !i.IsStopped() {
			now := time.Now()
			i.Update(false, func(render barlib.Renderer) {
				render(barproto.Block{
					FullText:  now.Format(c.LayoutFull),
					ShortText: now.Format(c.LayoutShort),
					Color:     c.Color,
					Separator: true,
				})
			})
		}
		select {
		case <-ticker:
		case <-i.Stopped():
		}
	}
}
