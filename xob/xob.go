// Package xob is a pure-go package for drawing X11 overlay bars.
//
// Inspired by github.com/florentc/xob@d6ca69d6a45a9c1ac1c99e00357d0df32f956f19.
package xob

import (
	"fmt"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
)

type OverflowMode int

const (
	OverflowModeHidden OverflowMode = iota
	OverflowModeProportional
)

type Orientation int

const (
	OrientationHorizontal Orientation = iota
	OrientationVertical
)

type ShowMode int

const (
	ShowModeNormal ShowMode = iota
	ShowModeAlternative
)

type Bar struct {
	x        xContext
	color    colorContext
	geometry geometryContext
}

type colorspec struct {
	FG     string
	BG     string
	Border string
	// alpha is not used
}

type dim struct {
	Rel float64
	Abs int
}

type xContext struct {
	Display *xgb.Conn
	Screen  *xproto.ScreenInfo
	Window  xproto.Window
	Mapped  bool
	GC      []xproto.Gcontext
}

type gcColorset struct {
	FG     xproto.Gcontext
	BG     xproto.Gcontext
	Border xproto.Gcontext
}

type colorContext struct {
	Normal      gcColorset
	Overflow    gcColorset
	Alt         gcColorset
	AltOverflow gcColorset
}

type geometryContext struct {
	Outline     int
	Border      int
	Padding     int
	Length      int
	Thickness   int
	Orientation Orientation
}

func (g geometryContext) SizeX() int {
	if g.Orientation == OrientationHorizontal {
		return g.Length
	}
	return g.Thickness
}

func (g geometryContext) SizeY() int {
	if g.Orientation == OrientationHorizontal {
		return g.Thickness
	}
	return g.Length
}

func NewBar(conf style) (*Bar, error) {
	var err error
	var dc Bar

	if dc.x.Display, err = xgb.NewConn(); err != nil {
		return nil, err
	}

	dc.x.Screen = xproto.Setup(dc.x.Display).DefaultScreen(dc.x.Display)

	dc.geometry.Outline = conf.Outline
	dc.geometry.Border = conf.Border
	dc.geometry.Padding = conf.Padding
	dc.geometry.Thickness = conf.Thickness
	dc.geometry.Orientation = conf.Orientation

	fatLayer := dc.geometry.Padding + dc.geometry.Border + dc.geometry.Outline

	var availableLength int
	if dc.geometry.Orientation == OrientationHorizontal {
		availableLength = int(dc.x.Screen.WidthInPixels)
	} else {
		availableLength = int(dc.x.Screen.HeightInPixels)
	}

	dc.geometry.Length = clamp(
		int(float64(availableLength)*conf.Length.Rel)+conf.Length.Abs,
		0,
		availableLength-2*fatLayer,
	)

	topLeftX := clamp(
		int(float64(dc.x.Screen.WidthInPixels)*conf.X.Rel)-(dc.geometry.SizeX()+2*fatLayer)/2,
		0,
		int(dc.x.Screen.WidthInPixels)-(dc.geometry.SizeX()+2*fatLayer),
	) + conf.X.Abs

	topLeftY := clamp(
		int(float64(dc.x.Screen.HeightInPixels)*conf.Y.Rel)-(dc.geometry.SizeY()+2*fatLayer)/2,
		0,
		int(dc.x.Screen.HeightInPixels)-(dc.geometry.SizeY()+2*fatLayer),
	) + conf.Y.Abs

	if dc.x.Window, err = xproto.NewWindowId(dc.x.Display); err != nil {
		dc.Close()
		return nil, err
	}

	if err = xproto.CreateWindowChecked(
		dc.x.Display,
		dc.x.Screen.RootDepth, dc.x.Window, dc.x.Screen.Root,
		int16(topLeftX), int16(topLeftY),
		uint16(dc.geometry.SizeX()+2*fatLayer), uint16(dc.geometry.SizeY()+2*fatLayer),
		0,
		xproto.WindowClassInputOutput, dc.x.Screen.RootVisual,
		xproto.CwBackPixel|xproto.CwBorderPixel|xproto.CwOverrideRedirect|xproto.CwColormap, []uint32{dc.x.Screen.BlackPixel, dc.x.Screen.BlackPixel, 1, uint32(dc.x.Screen.DefaultColormap)},
	).Check(); err != nil {
		dc.Close()
		return nil, err
	}

	if err = xproto.ChangePropertyChecked(dc.x.Display, xproto.PropModeReplace, dc.x.Window, xproto.AtomWmName, xproto.AtomString, 8, 3, []byte{'x', 'o', 'b'}).Check(); err != nil {
		dc.Close()
		return nil, err
	}

	if err = xproto.ChangePropertyChecked(dc.x.Display, xproto.PropModeReplace, dc.x.Window, xproto.AtomWmClass, xproto.AtomString, 8, 3, []byte{'x', 'o', 'b'}).Check(); err != nil {
		dc.Close()
		return nil, err
	}

	// custom (not in original xob): also make it always-on-top
	{
		ts, err := xproto.InternAtom(dc.x.Display, false, uint16(len("_NET_WM_STATE")), "_NET_WM_STATE").Reply()
		if err != nil {
			return nil, err
		}

		tsa, err := xproto.InternAtom(dc.x.Display, false, uint16(len("_NET_WM_STATE_ABOVE")), "_NET_WM_STATE_ABOVE").Reply()
		if err != nil {
			return nil, err
		}

		if err := xproto.SendEventChecked(dc.x.Display, false, dc.x.Screen.Root, xproto.EventMaskSubstructureNotify|xproto.EventMaskSubstructureRedirect, string(xproto.ClientMessageEvent{
			Type:   ts.Atom,
			Window: dc.x.Window,
			Format: 32,
			Data: xproto.ClientMessageDataUnionData32New([]uint32{
				1, // _NET_WM_STATE_ADD
				uint32(tsa.Atom),
				0,
				0,
				0,
				0,
			}),
		}.Bytes())).Check(); err != nil {
			return nil, err
		}
	}

	for _, v := range []struct {
		x *gcColorset
		y *colorspec
	}{
		{&dc.color.Normal, &conf.Color.Normal},
		{&dc.color.Overflow, &conf.Color.Overflow},
		{&dc.color.Alt, &conf.Color.Alt},
		{&dc.color.AltOverflow, &conf.Color.AltOverflow},
	} {
		if v.x.FG, err = gcFromString(dc.x, v.y.FG); err != nil {
			dc.Close()
			return nil, err
		} else {
			dc.x.GC = append(dc.x.GC, v.x.FG)
		}
		if v.x.BG, err = gcFromString(dc.x, v.y.BG); err != nil {
			dc.Close()
			return nil, err
		} else {
			dc.x.GC = append(dc.x.GC, v.x.BG)
		}
		if v.x.Border, err = gcFromString(dc.x, v.y.Border); err != nil {
			dc.Close()
			return nil, err
		} else {
			dc.x.GC = append(dc.x.GC, v.x.Border)
		}
	}

	return &dc, nil
}

func (dc *Bar) Show(value int, cap int, overflowMode OverflowMode, showMode ShowMode) error {
	if !dc.x.Mapped {
		if err := xproto.MapWindowChecked(dc.x.Display, dc.x.Window).Check(); err != nil {
			return err
		}
		dc.x.Mapped = true
		if err := xproto.ConfigureWindowChecked(dc.x.Display, dc.x.Window, xproto.ConfigWindowStackMode, []uint32{xproto.StackModeAbove}).Check(); err != nil {
			return err
		}
	}

	var colorset, colorsetOverflowProportional gcColorset
	switch showMode {
	case ShowModeNormal:
		colorsetOverflowProportional = dc.color.Normal
		if value <= cap {
			colorset = dc.color.Normal
		} else {
			colorset = dc.color.Overflow
		}
	case ShowModeAlternative:
		colorsetOverflowProportional = dc.color.Alt
		if value <= cap {
			colorset = dc.color.Alt
		} else {
			colorset = dc.color.AltOverflow
		}
	}

	if err := drawEmpty(dc.x, dc.geometry, colorset); err != nil {
		return err
	}
	if err := drawContent(dc.x, dc.geometry, clamp(value, 0, cap)*dc.geometry.Length/cap, colorset.FG); err != nil {
		return err
	}
	if value > cap && overflowMode == OverflowModeProportional && cap*dc.geometry.Length/value > dc.geometry.Padding {
		if err := drawContent(dc.x, dc.geometry, cap*dc.geometry.Length/value, colorsetOverflowProportional.FG); err != nil {
			return err
		}
		if err := drawSeparator(dc.x, dc.geometry, cap*dc.geometry.Length/value, colorsetOverflowProportional.BG); err != nil {
			return err
		}
	}

	dc.x.Display.Sync()

	return nil
}

func (dc *Bar) Hide() error {
	if dc.x.Display == nil {
		return nil
	}
	if dc.x.Window != 0 {
		if err := xproto.UnmapWindowChecked(dc.x.Display, dc.x.Window).Check(); err != nil {
			return err
		}
	}
	dc.x.Mapped = false
	return nil
}

func (dc *Bar) Close() {
	if dc.x.Display == nil {
		return
	}
	if dc.x.Window != 0 {
		_ = dc.Hide()
		_ = xproto.DestroyWindowChecked(dc.x.Display, dc.x.Window).Check()
	}
	for _, gc := range dc.x.GC {
		_ = xproto.FreeGCChecked(dc.x.Display, gc).Check()
	}
	dc.x.Display.Close()
}

func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func gcFromString(x xContext, color string) (xproto.Gcontext, error) {
	var pixel uint32
	if len(color) != 0 && color[0] == '#' {
		var cr, cg, cb uint8
		switch len(color) {
		case 7:
			if _, err := fmt.Sscanf(color, "#%02x%02x%02x", &cr, &cg, &cb); err != nil {
				return 0, fmt.Errorf("invalid hex color %q: %w", color, err)
			}
		case 4:
			if _, err := fmt.Sscanf(color, "#%02x%02x%02x", &cr, &cg, &cb); err != nil {
				return 0, fmt.Errorf("invalid hex color %q: %w", color, err)
			}
			cr *= 17
			cg *= 17
			cb *= 17
		default:
			return 0, fmt.Errorf("invalid hex color %q: wrong length", color)
		}
		xc, err := xproto.AllocColor(x.Display, x.Screen.DefaultColormap, uint16(cr)*257, uint16(cg)*257, uint16(cb)*257).Reply()
		if err != nil {
			return 0, err
		}
		pixel = xc.Pixel
	} else {
		xl, err := xproto.LookupColor(x.Display, x.Screen.DefaultColormap, uint16(len(color)), color).Reply()
		if err != nil {
			return 0, err
		}
		xc, err := xproto.AllocColor(x.Display, x.Screen.DefaultColormap, xl.ExactRed, xl.ExactGreen, xl.ExactBlue).Reply()
		if err != nil {
			return 0, err
		}
		pixel = xc.Pixel
	}
	xg, err := xproto.NewGcontextId(x.Display)
	if err != nil {
		return 0, err
	}
	if err := xproto.CreateGCChecked(x.Display, xg, xproto.Drawable(x.Window), 0, nil).Check(); err != nil {
		return 0, err
	}
	if err := xproto.ChangeGCChecked(x.Display, xg, xproto.GcForeground, []uint32{pixel}).Check(); err != nil {
		return 0, err
	}
	return xg, nil
}

func drawEmpty(x xContext, g geometryContext, color gcColorset) error {
	if err := xproto.PolyFillRectangleChecked(x.Display, xproto.Drawable(x.Window), color.BG, []xproto.Rectangle{{
		X:      0,
		Y:      0,
		Width:  uint16(2*(g.Outline+g.Border+g.Padding) + g.SizeX()),
		Height: uint16(2*(g.Outline+g.Border+g.Padding) + g.SizeY()),
	}}).Check(); err != nil {
		return err
	}
	if err := xproto.PolyFillRectangleChecked(x.Display, xproto.Drawable(x.Window), color.Border, []xproto.Rectangle{{
		X:      int16(g.Outline),
		Y:      int16(g.Outline),
		Width:  uint16(2*(g.Border+g.Padding) + g.SizeX()),
		Height: uint16(2*(g.Border+g.Padding) + g.SizeY()),
	}}).Check(); err != nil {
		return err
	}
	if err := xproto.PolyFillRectangleChecked(x.Display, xproto.Drawable(x.Window), color.BG, []xproto.Rectangle{{
		X:      int16(g.Outline + g.Border),
		Y:      int16(g.Outline + g.Border),
		Width:  uint16(2*+g.Padding + g.SizeX()),
		Height: uint16(2*+g.Padding + g.SizeY()),
	}}).Check(); err != nil {
		return err
	}
	return nil
}

func drawContent(x xContext, g geometryContext, filledLength int, color xproto.Gcontext) error {
	if g.Orientation == OrientationHorizontal {
		return xproto.PolyFillRectangleChecked(x.Display, xproto.Drawable(x.Window), color, []xproto.Rectangle{{
			X:      int16(g.Outline + g.Border + g.Padding),
			Y:      int16(g.Outline + g.Border + g.Padding),
			Width:  uint16(filledLength),
			Height: uint16(g.Thickness),
		}}).Check()
	} else {
		return xproto.PolyFillRectangleChecked(x.Display, xproto.Drawable(x.Window), color, []xproto.Rectangle{{
			X:      int16(g.Outline + g.Border + g.Padding),
			Y:      int16(g.Outline + g.Border + g.Padding + g.Length - filledLength),
			Width:  uint16(g.Thickness),
			Height: uint16(filledLength),
		}}).Check()
	}
}

func drawSeparator(x xContext, g geometryContext, position int, color xproto.Gcontext) error {
	if g.Orientation == OrientationHorizontal {
		return xproto.PolyFillRectangleChecked(x.Display, xproto.Drawable(x.Window), color, []xproto.Rectangle{{
			X:      int16(g.Outline + g.Border + (g.Padding / 2) + position),
			Y:      int16(g.Outline + g.Border + g.Padding),
			Width:  uint16(g.Padding),
			Height: uint16(g.Thickness),
		}}).Check()
	} else {
		return xproto.PolyFillRectangleChecked(x.Display, xproto.Drawable(x.Window), color, []xproto.Rectangle{{
			X:      int16(g.Outline + g.Border + g.Padding),
			Y:      int16(g.Outline + g.Border + (g.Padding / 2) + g.Length - position),
			Width:  uint16(g.Thickness),
			Height: uint16(g.Padding),
		}}).Check()
	}
}
