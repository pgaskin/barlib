// # redshift
//
// Like redshift, but deeply integrated into the status bar. Supports setting a
// custom color temperature by scrolling and enabling/disabling/resetting by
// clicking. Uses X11 to set color temperature directly. Automatically updates
// when outputs are changed.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/pgaskin/barlib"
	"github.com/pgaskin/barlib/barproto"
	"github.com/pgaskin/barlib/redshift"
)

type Redshift struct {
	Latitude         float64
	Longitude        float64
	ElevationDay     float64 // solar elevation in degrees for transition to daytime
	ElevationNight   float64 // solar elevation in degrees for transition to night
	TemperatureDay   redshift.Temperature
	TemperatureNight redshift.Temperature
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
	m, fatal, err := redshift.New(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	})))
	if err != nil {
		return err
	}
	defer m.Close()
	var (
		disabled    bool
		override    bool
		temperature redshift.Temperature
	)
	for ticker, isEvent := i.Tick(time.Second*15), false; ; {
		if !override {
			temperature = redshift.Solar(time.Now(), c.Latitude, c.Longitude, c.ElevationNight, c.ElevationDay, c.TemperatureNight, c.TemperatureDay)
		}

		var white redshift.WhitePoint
		if disabled {
			white = redshift.WhitePoint{1, 1, 1}
		} else {
			white, _ = redshift.GetWhitePoint(temperature)
		}
		m.Set(white)

		i.Update(isEvent, func(render barlib.Renderer) {
			block := barproto.Block{
				Separator:           true,
				SeparatorBlockWidth: -1,
			}
			if disabled {
				block.FullText = "----K"
			} else {
				block.FullText = strconv.Itoa(int(temperature)) + "K"
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
			case err := <-fatal:
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
