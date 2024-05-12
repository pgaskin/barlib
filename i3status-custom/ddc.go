// # ddc
//
// Controls monitor brightness/contrast using DDC-CI.
package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"time"

	"github.com/pgaskin/barlib"
	"github.com/pgaskin/barlib/barproto"
	"github.com/pgaskin/barlib/ddc"
)

// also see:
// - https://poynton.ca/notes/brightness_and_contrast/
// - https://www.rtings.com/monitor/tests/picture-quality/contrast-ratio

type DDC struct {
	Interval   time.Duration // interval to sync current value or probe for monitor
	ID         string        // see [ddc.FindMonitor]
	HideIfGone bool          // instead of showing an error
	Brightness bool          // whether to show brightness (if present)
	Contrast   bool          // whether to show contrast (if present)
	Presets    [][2]uint16   // brightness/contrast presets to toggle through on click
}

func (c DDC) Run(i barlib.Instance) error {
	var (
		ci             *ddc.CI // controller, nil if closed
		i2c            int     // i2c bus for monitor
		unauthorized   bool    // is unauthorized to access i2c bus?
		hasBr, hasCn   bool    // is present?
		brCur, cnCur   uint16  // current value (if present)
		brMax, cnMax   uint16  // maximum value (if present)
		brSkip, cnSkip bool    // whether to skip the next update
	)
	defer func() {
		if ci != nil {
			ci.Close()
		}
	}()
	for ticker, isEvent := i.Tick(c.Interval), false; ; {
		if !i.IsStopped() {
			if ci != nil && c.Brightness {
				if brSkip {
					brSkip = false
				} else {
					var err error
					brCur, brMax, err = ci.GetVCP(ddc.VCP_Brightness)
					if err == nil {
						hasBr = true
					} else if errors.Is(err, ddc.ErrUnsupportedVCP) {
						hasBr = false
					} else if errors.Is(err, ddc.ErrDeviceGone) {
						ci.Close()
						ci = nil
					} else if errors.Is(err, ddc.ErrNoReply) && hasBr {
						// ignore, probably asleep, and we already know it works
					} else {
						return fmt.Errorf("get brightness: %w", err)
					}
				}
			}
			if ci != nil && c.Contrast {
				if cnSkip {
					cnSkip = false
				} else {
					var err error
					cnCur, cnMax, err = ci.GetVCP(ddc.VCP_Contrast)
					if err == nil {
						hasCn = true
					} else if errors.Is(err, ddc.ErrUnsupportedVCP) {
						hasCn = false
					} else if errors.Is(err, ddc.ErrDeviceGone) {
						ci.Close()
						ci = nil
					} else if errors.Is(err, ddc.ErrNoReply) && hasCn {
						// ignore, probably asleep, and we already know it works
					} else {
						return fmt.Errorf("get contrast: %w", err)
					}
				}
			}
			if ci == nil {
				i2c, unauthorized = 0, false
				hasBr, hasCn = false, false

				card, err := ddc.FindMonitor(c.ID)
				if err != nil {
					return fmt.Errorf("enumerate monitors: %w", err)
				}
				if card != "" {
					i2cs, err := ddc.FindI2C(card)
					if err == nil && len(i2cs) != 1 {
						err = fmt.Errorf("expected exactly 1 bus, got %d", len(i2cs))
					}
					if err != nil {
						return fmt.Errorf("find ddc i2c bus: %w", err)
					}
					i2c = i2cs[0]

					ci, err = ddc.Open(i2c)
					if errors.Is(err, fs.ErrPermission) {
						unauthorized = true
					} else if err != nil {
						return fmt.Errorf("open ddc i2c bus: %w", err)
					} else {
						continue // immediately do another update
					}
				} else if !c.HideIfGone {
					return fmt.Errorf("monitor %q not found", c.ID)
				}
			}
		}
		if ci != nil {
			i.Update(isEvent, func(render barlib.Renderer) {
				if hasBr {
					render(barproto.Block{
						Instance: "br",
						FullText: fmt.Sprintf("%02.0f", float64(brCur)/float64(brMax)*100),
					})
				}
				if hasBr && hasCn {
					render(barproto.Block{
						FullText: "/",
					})
				}
				if hasCn {
					render(barproto.Block{
						Instance: "cn",
						FullText: fmt.Sprintf("%02.0f", float64(cnCur)/float64(cnMax)*100),
					})
				}
				if hasBr || hasCn {
					render(barproto.Block{
						FullText:  "%",
						Separator: true,
					})
				}
			})
		} else if unauthorized {
			i.Update(isEvent, func(render barlib.Renderer) {
				render(barproto.Block{
					FullText:  c.ID,
					Color:     0xFF0000FF,
					Separator: true,
				})
			})
		}
		for isEvent = false; ; {
			select {
			case <-ticker:
			case <-i.Stopped():
			case event := <-i.Event():
				var (
					next = event.Button == 1
					prev = event.Button == 3
					incr = event.Button == 4
					decr = event.Button == 5
				)
				var (
					brNew, brSet = brCur, false
					cnNew, cnSet = cnCur, false
				)
				switch {
				case unauthorized:
					if err := exec.Command("pkexec", "setfacl", "-m", "u:"+strconv.Itoa(os.Getuid())+":rw", "/dev/i2c-"+strconv.Itoa(i2c)).Run(); err != nil {
						return fmt.Errorf("get permissions to access i2c %d: %w", i2c, err)
					}
					isEvent = true
				case next, prev:
					if ci != nil && len(c.Presets) != 0 {
						// if preset matches a known one, go to the next/prev one for left/right button
						// otherwise, go to the closet one
						idx := slices.IndexFunc(c.Presets, func(preset [2]uint16) bool {
							return (!hasBr || preset[0] == brCur) && (!hasCn || preset[1] == cnCur)
						})
						if idx == -1 {
							dist := make([]int, len(c.Presets))
							for i, preset := range c.Presets {
								if hasBr {
									dist[i] += int(max(preset[0], brCur) - min(preset[0], brCur))
								}
								if hasCn {
									dist[i] += int(max(preset[1], cnCur) - min(preset[1], cnCur))
								}
							}
							idx = slices.Index(dist, slices.Min(dist))
						} else {
							switch {
							case next:
								if idx++; idx >= len(c.Presets) {
									idx = 0
								}
							case prev:
								if idx--; idx < 0 {
									idx = len(c.Presets) - 1
								}
							}
						}
						if hasBr {
							brNew, brSet = c.Presets[idx][0], true
						}
						if hasCn {
							cnNew, cnSet = c.Presets[idx][1], true
						}
					}
				case incr, decr:
					var (
						br = event.Instance == "br"
						cn = event.Instance == "cn"
					)
					if ci != nil && (br || cn) {
						var cur, max uint16
						switch {
						case br:
							cur, max = brCur, brMax
						case cn:
							cur, max = cnCur, cnMax
						}
						switch {
						case incr:
							switch {
							case cur < 20:
								cur += 1
							case cur < 50:
								cur += 2
							default:
								cur += 5
							}
							if cur > max {
								cur = max
							}
						case decr:
							switch {
							case cur > 50:
								cur -= 5
							case cur > 20:
								cur -= 2
							default:
								cur -= 1
							}
							if cur > max {
								cur = 0
							}
						}
						switch {
						case br:
							brNew, brSet = cur, true
						case cn:
							cnNew, cnSet = cur, true
						}
					}
				}
				if ci != nil && hasBr && brSet {
					if brNew > brMax {
						return fmt.Errorf("brightness out of range")
					}
					if err := ci.SetVCP(ddc.VCP_Brightness, brNew); err != nil {
						if errors.Is(err, ddc.ErrDeviceGone) {
							ci.Close()
							ci = nil
						} else {
							return fmt.Errorf("set brightness to %d/%d: %w", brNew, brMax, err)
						}
					}
					brCur, brSkip, isEvent = brNew, true, true
				}
				if ci != nil && hasCn && cnSet {
					if cnNew > cnMax {
						return fmt.Errorf("contrast out of range")
					}
					if err := ci.SetVCP(ddc.VCP_Contrast, cnNew); err != nil {
						if errors.Is(err, ddc.ErrDeviceGone) {
							ci.Close()
							ci = nil
						} else {
							return fmt.Errorf("set contrast to %d/%d: %w", cnNew, cnMax, err)
						}
					}
					cnCur, cnSkip, isEvent = cnNew, true, true
				}
			}
			break
		}
	}
}
