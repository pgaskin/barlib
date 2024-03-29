// # xrandr
//
// Shows the current XRANDR outputs. When there are exactly two outputs, it lets
// you swap which one is the primary display, and provides seven preset layouts
// using the preferred mode for the output. Starts arandr on middle-click. Not
// tested on hidpi displays not running at 1:1. Does not currently support
// rotation.
package main

import (
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/randr"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/pgaskin/barlib"
	"github.com/pgaskin/barlib/barproto"
)

type XRandR struct {
}

func (c XRandR) Run(i barlib.Instance) error {
	conn, err := xgb.NewConn()
	if err != nil {
		return err
	}
	if err := randr.Init(conn); err != nil {
		return err
	}
	root := xproto.Setup(conn).DefaultScreen(conn).Root
	if err := randr.SelectInputChecked(conn, root, randr.NotifyMaskCrtcChange|randr.NotifyMaskOutputChange).Check(); err != nil {
		return err
	}
	ch := make(chan error, 1)
	go func() {
		var err error
		for err == nil {
			_, err = conn.WaitForEvent()
			ch <- err
		}
	}()
	var (
		layoutSelecting bool
		layoutSelection string
	)
	for isEvent := false; ; {
		// this logic intentionally supports a limited set of two-monitor
		// configurations
		//
		//  - the logic is MUCH simpler
		//  - it's easier to drag stuff around in arandr or use the terminal
		//    than scroll through a long list of textual options)
		//
		//          |-----|
		//          |     |
		//          +-----|-------|
		//          |             |
		//    |-----|   primary   |-----|
		//    |     |             |     |
		//    |-----+-----|-------+-----|
		//          |     |
		//          |-----|
		//
		//  - highest supported resolution/rate for both
		//  - anchor points are denoted by the +
		//  - we don't touch transforms/panning/rotation/etc (if we need
		//    rotation, we could add a button to rotate the primary plus one for
		//    the secondary like the swap button)
		//
		//  - ⇆ (swap which is primary)
		//  - pri
		//  - pri=sec
		//  - pri→sec
		//  - pri←sec
		//  - pri↑sec
		//  - pri↓sec

		resources, err := randr.GetScreenResources(conn, root).Reply()
		if err != nil {
			return err
		}

		primary, err := randr.GetOutputPrimary(conn, root).Reply()
		if err != nil {
			return err
		}

		szRange, err := randr.GetScreenSizeRange(conn, root).Reply()
		if err != nil {
			return err
		}

		// https://tomverbeure.github.io/video_timings_calculator
		// https://glenwing.github.io/docs/VESA-CVT-1.2.pdf
		// https://gitlab.freedesktop.org/xorg/app/xrandr/-/blob/master/xrandr.c
		type outputInfo struct {
			Name   string
			Output randr.Output
			Mode   randr.Mode // the default one
			Width  int16
			Height int16
			Freq   float64
		}
		type crtcInfo struct {
			E bool
			X int16
			Y int16
			R uint16
			M randr.Mode
			O randr.Output
			W int16 // matches the mode
			H int16 // matches the mode
		}
		type layoutInfo struct {
			Label    string
			W, H     int16
			Pri, Sec crtcInfo
		}
		var (
			active  string // layout label
			layouts []layoutInfo
			outputs []outputInfo // first is primary
			apply   func(layoutInfo) error
			swap    func() error // swaps the primary display for the currently active mode
			//rotatePri func() error // rotates the primary display for the currently active mode
			//rotateSec func() error // ^ for the secondary
		)
		for _, output := range resources.Outputs {
			info, err := randr.GetOutputInfo(conn, output, 0).Reply()
			if err != nil {
				return err
			}
			if info.Connection == randr.ConnectionConnected && len(info.Modes) != 0 {
				i := slices.IndexFunc(resources.Modes, func(m randr.ModeInfo) bool {
					// first info.NumPreferred items are the preferred modes
					return m.Id == uint32(info.Modes[0])
				})
				if i == -1 {
					return fmt.Errorf("wtf: cannot find mode %d", info.Modes[0])
				}
				m := resources.Modes[i]
				x := outputInfo{
					Name:   string(info.Name),
					Output: output,
					Mode:   info.Modes[0],
					Width:  int16(m.Width),
					Height: int16(m.Height),
					Freq:   float64(m.DotClock) / float64(m.Htotal) / float64(m.Vtotal),
				}
				if primary.Output == output {
					outputs = slices.Insert(outputs, 0, x)
				} else {
					outputs = append(outputs, x)
				}
				//fmt.Fprintf(os.Stderr, "%#v\n", output)
			}
		}
		if len(outputs) == 2 {
			var (
				pri    = outputs[0]
				sec    = outputs[1]
				priCur crtcInfo
				secCur crtcInfo
			)
			for _, crtc := range resources.Crtcs {
				info, err := randr.GetCrtcInfo(conn, crtc, 0).Reply()
				if err != nil {
					return err
				}
				if slices.Contains(info.Outputs, pri.Output) {
					priCur.E = true
					priCur.X = info.X
					priCur.Y = info.Y
					priCur.R = info.Rotation
					priCur.M = info.Mode
					priCur.O = pri.Output
					priCur.W = int16(info.Width)
					priCur.H = int16(info.Height)
				}
				if slices.Contains(info.Outputs, sec.Output) {
					secCur.E = true
					secCur.X = info.X
					secCur.Y = info.Y
					secCur.R = info.Rotation
					secCur.M = info.Mode
					secCur.O = sec.Output
					secCur.W = int16(info.Width)
					secCur.H = int16(info.Height)
				}
			}
			// TODO: figure out hidpi support if needed
			addLayout := func(label string, priLay, secLay crtcInfo) bool {
				if !priLay.E {
					priLay.X = 0
					priLay.Y = 0
					priLay.R = randr.RotationRotate0
					priLay.M = 0
				}
				if !secLay.E {
					secLay.X = 0
					secLay.Y = 0
					secLay.R = randr.RotationRotate0
					secLay.M = 0
				}

				if priLay.X < 0 {
					secLay.X -= priLay.X
					priLay.X -= priLay.X
				}
				if secLay.X < 0 {
					priLay.X -= secLay.X
					secLay.X -= secLay.X
				}
				if priLay.Y < 0 {
					secLay.Y -= priLay.Y
					priLay.Y -= priLay.Y
				}
				if secLay.Y < 0 {
					priLay.Y -= secLay.Y
					secLay.Y -= secLay.Y
				}

				layout := layoutInfo{
					Label: label,
					W:     max(priLay.X+priLay.W, secLay.X+secLay.W, int16(szRange.MinWidth)),
					H:     max(priLay.Y+priLay.H, secLay.Y+secLay.H, int16(szRange.MinHeight)),
					Pri:   priLay,
					Sec:   secLay,
				}
				if layout.W > int16(szRange.MaxWidth) {
					return false
				}
				if layout.H > int16(szRange.MaxHeight) {
					return false
				}
				layouts = append(layouts, layout)
				return true
			}
			addLayoutSimple := func(label string, priE, secE bool, priX, priY, secX, secY int16) bool {
				var priLay, secLay crtcInfo
				priLay.E, secLay.E = priE, secE
				priLay.X, secLay.X = priX, secX
				priLay.Y, secLay.Y = priY, secY
				priLay.R, secLay.R = randr.RotationRotate0, randr.RotationRotate0
				priLay.M, secLay.M = pri.Mode, sec.Mode
				priLay.O, secLay.O = pri.Output, sec.Output
				priLay.W, secLay.W = pri.Width, sec.Width
				priLay.H, secLay.H = pri.Height, sec.Height
				return addLayout(label, priLay, secLay)
			}
			addLayoutSimple(
				pri.Name,
				true, false,
				0, 0,
				0, 0,
			)
			addLayoutSimple(
				sec.Name,
				false, true,
				0, 0,
				0, 0,
			)
			if pri.Width == sec.Width && pri.Height == sec.Height {
				addLayoutSimple(
					pri.Name+"="+sec.Name,
					true, true,
					0, 0,
					0, 0,
				)
			}
			addLayoutSimple(
				pri.Name+"\uf060"+sec.Name, // left
				true, true,
				pri.Width, -pri.Height,
				0, -sec.Height,
			)
			addLayoutSimple(
				pri.Name+"\uf061"+sec.Name, // right
				true, true,
				0, -pri.Height,
				pri.Width, -sec.Height,
			)
			addLayoutSimple(
				pri.Name+"\uf062"+sec.Name, // up
				true, true,
				0, 0,
				0, -sec.Height,
			)
			addLayoutSimple(
				pri.Name+"\uf063"+sec.Name, // down
				true, true,
				0, 0,
				0, pri.Height,
			)
			for _, layout := range layouts {
				if layout.Pri.E != priCur.E || layout.Sec.E != secCur.E {
					continue
				}
				if layout.Pri.E {
					if layout.Pri.X != priCur.X || layout.Pri.Y != priCur.Y {
						continue
					}
					if layout.Pri.W != priCur.W || layout.Pri.H != priCur.H {
						continue
					}
					if layout.Pri.R != priCur.R {
						continue
					}
				}
				if layout.Sec.E {
					if layout.Sec.X != secCur.X || layout.Sec.Y != secCur.Y {
						continue
					}
					if layout.Sec.W != secCur.W || layout.Sec.H != secCur.H {
						continue
					}
					if layout.Sec.R != secCur.R {
						continue
					}
				}
				active = layout.Label
				break
			}
			apply = func(layout layoutInfo) error {
				// this is much more reliable; it's somewhat complicated, and we only do it when changing, so it's okay that it's a bit inefficient
				cmd := exec.Command("xrandr")
				for _, crtc := range []crtcInfo{layout.Pri, layout.Sec} {
					cmd.Args = append(cmd.Args, "--output", "0x"+strconv.FormatUint(uint64(crtc.O), 16))
					if crtc.E {
						cmd.Args = append(cmd.Args, "--mode", "0x"+strconv.FormatUint(uint64(crtc.M), 16))
						cmd.Args = append(cmd.Args, "--pos", strconv.FormatInt(int64(crtc.X), 10)+"x"+strconv.FormatInt(int64(crtc.Y), 10))
						switch {
						case crtc.R&randr.RotationRotate0 != 0:
							cmd.Args = append(cmd.Args, "--rotate", "normal")
						case crtc.R&randr.RotationRotate90 != 0:
							cmd.Args = append(cmd.Args, "--rotate", "left")
						case crtc.R&randr.RotationRotate180 != 0:
							cmd.Args = append(cmd.Args, "--rotate", "inverted")
						case crtc.R&randr.RotationRotate270 != 0:
							cmd.Args = append(cmd.Args, "--rotate", "right")
						}
						if rx, ry := crtc.R&randr.RotationReflectX != 0, crtc.R&randr.RotationReflectY != 0; !rx && !ry {
							cmd.Args = append(cmd.Args, "--reflect", "normal")
						} else if rx {
							cmd.Args = append(cmd.Args, "--reflect", "x")
						} else if ry {
							cmd.Args = append(cmd.Args, "--reflect", "y")
						} else {
							cmd.Args = append(cmd.Args, "--reflect", "xy")
						}
					} else {
						cmd.Args = append(cmd.Args, "--off")
					}
				}
				if err := cmd.Run(); err != nil {
					return fmt.Errorf("exec %s: %w", cmd.Args, err)
				}
				return nil
			}
			swap = func() error {
				return randr.SetOutputPrimaryChecked(conn, root, sec.Output).Check()
			}
		}

		// if we've scrolled to a layout and it doesn't exist or isn't valid
		// anymore, clear it
		if layoutSelecting {
			if i := slices.IndexFunc(layouts, func(layout layoutInfo) bool {
				return layout.Label == layoutSelection
			}); i == -1 {
				layoutSelecting = false
			}
		}

		// if we haven't explicitly scrolled to a layout, set it to the active
		// one
		if !layoutSelecting {
			if i := slices.IndexFunc(layouts, func(layout layoutInfo) bool {
				return layout.Label == active
			}); i != -1 {
				layoutSelection = layouts[i].Label
			} else if len(layouts) > 0 {
				layoutSelection = layouts[0].Label
			} else {
				layoutSelection = ""
			}
		}

		// if the selected layout is the active one, clear the explicit
		// selection
		if layoutSelection == active {
			layoutSelecting = false
		}

	render:
		i.Update(isEvent, func(render barlib.Renderer) {
			var (
				isActive bool
				isPreset bool
				preset   = "UNK"
				color    = uint32(0x00FF00FF)
			)
			if len(outputs) != 0 {
				var b strings.Builder
				for i, o := range outputs {
					if i != 0 {
						b.WriteRune('•')
					}
					b.WriteString(o.Name)
				}
				preset = b.String()
			}
			for _, layout := range layouts {
				if layout.Label == layoutSelection {
					preset = layout.Label
					isActive = preset == active
					isPreset = true
					break
				}
			}
			if !isPreset {
				color = 0
			}
			if isPreset && !isActive {
				color = 0xFFFF00FF
			}
			if swap != nil {
				render(barproto.Block{
					Instance: "swap",
					FullText: "\uf0ec ",
				})
			}
			render(barproto.Block{
				Instance:  "preset",
				FullText:  preset,
				Color:     color,
				Separator: true,
			})
		})
		for isEvent = false; ; {
			select {
			case err := <-ch:
				if err != nil {
					return err
				}
			case event := <-i.Event():
				switch event.Button {
				default:
					continue
				case 1:
					switch event.Instance {
					case "swap":
						if swap != nil {
							if err := swap(); err != nil {
								return err
							}
						}
					case "preset":
						for _, layout := range layouts {
							if layout.Label == layoutSelection {
								if err := apply(layout); err != nil {
									return err
								}
								layoutSelecting = false
								break
							}
						}
					}
				case 2:
					go i3msg(`exec --no-startup-id arandr`)
				case 4:
					layoutSelecting = true
					if len(layouts) == 0 {
						layoutSelection = ""
					} else if layoutSelection == "" {
						layoutSelection = layouts[0].Label
					} else {
						for i, layout := range layouts {
							if layout.Label == layoutSelection {
								if i++; i > len(layouts) {
									i = 0
								}
								if i == len(layouts) {
									layoutSelection = ""
								} else {
									layoutSelection = layouts[i].Label
								}
								break
							}
						}
					}
					goto render
				case 5:
					layoutSelecting = true
					if len(layouts) == 0 {
						layoutSelection = ""
					} else if layoutSelection == "" {
						layoutSelection = layouts[len(layouts)-1].Label
					} else {
						for i, layout := range layouts {
							if layout.Label == layoutSelection {
								if i--; i < 0 {
									i = len(layouts)
								}
								if i == len(layouts) {
									layoutSelection = ""
								} else {
									layoutSelection = layouts[i].Label
								}
								break
							}
						}
					}
					goto render
				}
			}
			break
		}
	}
}
