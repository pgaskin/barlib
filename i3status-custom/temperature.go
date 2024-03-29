// # temperature
//
// Polls sysfs for the hwmon temperature of the configured chip and sensor
// index.
package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/pgaskin/barlib"
	"github.com/pgaskin/barlib/barproto"
)

type Temperature struct {
	Interval time.Duration
	Chip     string
	Index    int
}

func (c Temperature) Run(i barlib.Instance) error {
	var path string
	for ticker, isEvent := i.Tick(c.Interval), false; ; {
		if !i.IsStopped() {
			if path == "" {
				if ds, err := os.ReadDir("/sys/class/hwmon"); err == nil {
					for _, d := range ds {
						if b, err := os.ReadFile(filepath.Join("/sys/class/hwmon", d.Name(), "name")); err == nil {
							if string(bytes.TrimSpace(b)) == c.Chip {
								path = filepath.Join("/sys/class/hwmon", d.Name(), "temp"+strconv.Itoa(c.Index)+"_input")
								if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
									path = ""
								}
							}
						}
					}
				}
			}
			var (
				temp int64
				err  error
			)
			if path != "" {
				temp, err = readFileInt[int64](path)
				if errors.Is(err, fs.ErrNotExist) {
					path = ""
				}
			}
			if path == "" {
				i.Update(isEvent, func(render barlib.Renderer) {
					render(barproto.Block{
						FullText:  "?",
						Color:     0xFF0000FF,
						Separator: true,
					})
				})
			} else {
				i.Update(isEvent, func(render barlib.Renderer) {
					if err != nil {
						render.Err(err)
						return
					}
					render(barproto.Block{
						FullText:  strconv.FormatInt((temp+500)/1000, 10) + "°C",
						Separator: true,
					})
				})
			}
		}
		for isEvent = false; ; {
			select {
			case <-ticker:
			case <-i.Stopped():
			case event := <-i.Event():
				switch event.Button {
				default:
					continue
				case 1:
					isEvent = true
				}
			}
			break
		}
	}
}
