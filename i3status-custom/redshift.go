// # redshift
//
// Like redshift, but deeply integrated into the status bar. Supports setting a
// custom color temperature by scrolling and enabling/disabling/resetting by
// clicking. Uses X11 to set color temperature directly. Automatically updates
// when outputs are changed.
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/randr"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/nathan-osman/go-sunrise"
	"github.com/pgaskin/barlib"
	"github.com/pgaskin/barlib/barproto"
	"github.com/pgaskin/barlib/redshift"
)

type Redshift struct {
	Latitude         float64
	Longitude        float64
	ElevationDay     float64 // solar elevation in degrees for transition to daytime
	ElevationNight   float64 // solar elevation in degrees for transition to night
	TemperatureDay   float64
	TemperatureNight float64
}

func (c Redshift) Run(i barlib.Instance) error {
	if c.ElevationNight >= c.ElevationDay {
		return fmt.Errorf("night elevation must be smaller than day")
	}
	if c.TemperatureDay == 0 {
		c.TemperatureDay = 6500
	}
	if c.TemperatureNight == 0 {
		c.TemperatureNight = 4500
	}
	var (
		update func(float32, float32, float32) error
		ch     <-chan error
	)
	if disp := os.Getenv("WAYLAND_DISPLAY"); disp != "" {
		var close func()
		var err error
		update, close, ch, err = redshift.ColorRampWayland(disp)
		if err != nil {
			return err
		}
		defer close()
	} else {
		conn, err := xgb.NewConn()
		if err != nil {
			return err
		}
		defer conn.Close()

		if err := randr.Init(conn); err != nil {
			return err
		}
		for _, root := range xproto.Setup(conn).Roots {
			if err := randr.SelectInputChecked(conn, root.Root, randr.NotifyMaskCrtcChange).Check(); err != nil {
				return err
			}
		}

		errCh := make(chan error)
		go func() {
			for {
				_, err = conn.WaitForEvent()
				if err != nil {
					errCh <- err
					return
				}
			}
		}()

		update = func(wr, wg, wb float32) error {
			return redshift.ColorRampX11(conn, wr, wg, wb)
		}
		ch = errCh
	}
	var (
		disabled    bool
		override    bool
		temperature int
	)
	for ticker, isEvent := i.Tick(time.Second*15), false; ; {
		if !override {
			elevation := sunrise.Elevation(c.Latitude, c.Longitude, time.Now())

			var progress float64
			switch {
			case elevation < c.ElevationNight:
				progress = 0
			case elevation >= c.ElevationDay:
				progress = 1
			default:
				progress = (c.ElevationNight - elevation) / (c.ElevationNight - c.ElevationDay)
			}
			temperature = int((1-progress)*float64(c.TemperatureNight) + progress*float64(c.TemperatureDay))
		}
		temperature = min(max(temperature, 1000), 25000)

		var wr, wg, wb float32
		if disabled {
			wr, wg, wb = 1, 1, 1
		} else {
			wr, wg, wb, _ = redshift.WhitePoint(temperature)
		}
		if err := update(wr, wg, wb); err != nil {
			return err
		}

		i.Update(isEvent, func(render barlib.Renderer) {
			block := barproto.Block{
				Separator:           true,
				SeparatorBlockWidth: -1,
			}
			if disabled {
				block.FullText = "----K"
			} else {
				block.FullText = strconv.Itoa(temperature) + "K"
			}
			if disabled {
				block.Color = 0xFFFF00FF
			} else if override {
				block.Color = 0x00FF00FF
			}
			render(block)
		})

		for isEvent = false; ; {
			select {
			case err := <-ch:
				return err
			case <-ticker:
			case <-i.Stopped():
			case event := <-i.Event():
				switch event.Button {
				default:
					continue
				case 1:
					if !disabled && override {
						override = false
					} else {
						disabled = !disabled
					}
				case 4:
					disabled = false
					override = true
					temperature += 50
				case 5:
					disabled = false
					override = true
					temperature -= 50
				}
			}
			break
		}
	}
}
