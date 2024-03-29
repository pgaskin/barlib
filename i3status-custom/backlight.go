// # backlight
//
// Reads backlight values from sysfs, and sets them using systemd-logind over
// DBus.
package main

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"path/filepath"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/pgaskin/barlib"
	"github.com/pgaskin/barlib/barproto"
)

type Backlight struct {
	Interval  time.Duration
	Subsystem string
	Name      string
	Separator bool
}

func (c Backlight) Run(i barlib.Instance) error {
	i.Tick(c.Interval)
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	var (
		setErr       error
		blCur, blMax uint32
	)
	for isEvent := false; ; {
		if !i.IsStopped() {
			blMax, err = readFileUint[uint32](filepath.Join("/sys/class", c.Subsystem, c.Name, "max_brightness"))
			if err == nil {
				blCur, err = readFileUint[uint32](filepath.Join("/sys/class", c.Subsystem, c.Name, "brightness"))
			}
			i.Update(isEvent, func(render barlib.Renderer) {
				if err != nil {
					if errors.Is(err, fs.ErrNotExist) {
						render(barproto.Block{
							FullText:            "?",
							Color:               0xFF0000FF,
							Separator:           c.Separator,
							SeparatorBlockWidth: -1,
						})
					} else {
						render.Err(setErr)
					}
					return
				}
				if setErr != nil {
					render.Err(setErr)
					return
				}
				render(barproto.Block{
					FullText:            fmt.Sprintf("%.0f%%", float64(blCur)/float64(blMax)*100),
					Separator:           c.Separator,
					SeparatorBlockWidth: -1,
				})
			})
		}
		for isEvent = false; ; {
			select {
			case <-i.Ticked():
			case <-i.Stopped():
			case event := <-i.Event():
				blPct := int(math.Round(float64(blCur) / float64(blMax) * 100))
				blNew := blPct
				switch event.Button {
				default:
					continue
				case 4:
					isEvent = true
					if blNew < 45 {
						blNew += 1
					} else {
						blNew += 5
					}
				case 5:
					isEvent = true
					if blNew > 45 {
						blNew -= 5
					} else {
						blNew -= 1
					}
				}
				if blNew != blPct {
					blCur = uint32(min(max(float64(blNew)/100, 0), 1) * float64(blMax))
					setErr = conn.Object("org.freedesktop.login1", "/org/freedesktop/login1/session/self").Call("org.freedesktop.login1.Session.SetBrightness", 0, c.Subsystem, c.Name, blCur).Err
				}
			}
			break
		}
	}
}
