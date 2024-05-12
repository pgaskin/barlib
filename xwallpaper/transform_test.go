package xwallpaper

import (
	"fmt"
	"image"
	"strconv"
	"testing"

	"golang.org/x/image/math/f64"
)

func TestTransform(t *testing.T) {
	var (
		pt   = image.Pt
		rect = image.Rect
		s2d  = func(sx, sy, tx, ty float64) f64.Aff3 {
			return f64.Aff3{
				sx, 0, tx,
				0, sy, ty,
			}
		}
		none = image.Rectangle{}
		s2ds = func(s2d f64.Aff3, sr image.Rectangle) string {
			var b []byte
			if sr != none {
				b = append(b, "crop("...)
				b = strconv.AppendInt(b, int64(sr.Dx()), 10)
				b = append(b, 'x')
				b = strconv.AppendInt(b, int64(sr.Dy()), 10)
				if sr.Min.X >= 0 {
					b = append(b, '+')
				}
				b = strconv.AppendInt(b, int64(sr.Min.X), 10)
				if sr.Min.Y >= 0 {
					b = append(b, '+')
				}
				b = strconv.AppendInt(b, int64(sr.Min.Y), 10)
				b = append(b, ')')
			}
			if scaleX, scaleY := s2d[0], s2d[4]; scaleX != 1 || scaleY != 1 {
				if b != nil {
					b = append(b, ' ')
				}
				b = append(b, "scale("...)
				b = strconv.AppendFloat(b, scaleX, 'f', -1, 64)
				b = append(b, ',')
				b = strconv.AppendFloat(b, scaleY, 'f', -1, 64)
				b = append(b, ')')
			}
			if transX, transY := s2d[2], s2d[5]; transX != 0 || transY != 0 {
				if b != nil {
					b = append(b, ' ')
				}
				b = append(b, "translate("...)
				b = strconv.AppendFloat(b, transX, 'f', -1, 64)
				b = append(b, ',')
				b = strconv.AppendFloat(b, transY, 'f', -1, 64)
				b = append(b, ')')
			}
			return string(b)
		}
	)
	for _, tc := range []struct {
		dr   image.Point
		sr   image.Point
		tr   image.Rectangle
		mode Mode
		s2d  f64.Aff3
		cr   image.Rectangle
	}{
		{pt(1000, 500), pt(250, 200), none, Center, s2d(1, 1, 375, 150), rect(0, 0, 250, 200)},
		{pt(1000, 500), pt(200, 250), none, Center, s2d(1, 1, 400, 125), rect(0, 0, 200, 250)},
		{pt(1000, 500), pt(250, 200), none, Stretch, s2d(4, 2.5, 0, 0), rect(0, 0, 250, 200)},
		{pt(1000, 500), pt(200, 250), none, Stretch, s2d(5, 2, 0, 0), rect(0, 0, 200, 250)},
		{pt(1000, 500), pt(250, 200), none, Zoom, s2d(4, 4, 0, -150), rect(0, 0, 250, 200)},
		{pt(1000, 500), pt(200, 250), none, Zoom, s2d(5, 5, 0, -375), rect(0, 0, 200, 250)},
		{pt(1000, 500), pt(250, 200), none, Maximize, s2d(2.5, 2.5, 187.5, 0), rect(0, 0, 250, 200)},
		{pt(1000, 500), pt(200, 250), none, Maximize, s2d(2, 2, 300, 0), rect(0, 0, 200, 250)},
		{pt(1000, 500), pt(250, 200), none, Focus, s2d(2.5, 2.5, 187.5, 0), rect(-75, 0, -75+400, 0+200)},
		{pt(1000, 500), pt(200, 250), none, Focus, s2d(2, 2, 300, 0), rect(-150, 0, -150+500, 0+250)},

		{pt(1000, 500), pt(250, 200), rect(10, 10, 30, 50), Center, s2d(1, 1, 480, 220), rect(10, 10, 10+20, 10+40)},
		{pt(1000, 500), pt(200, 250), rect(10, 10, 30, 50), Center, s2d(1, 1, 480, 220), rect(10, 10, 10+20, 10+40)},
		{pt(1000, 500), pt(250, 200), rect(10, 10, 30, 50), Stretch, s2d(50, 12.5, -500, -125), rect(10, 10, 10+20, 10+40)},
		{pt(1000, 500), pt(200, 250), rect(10, 10, 30, 50), Stretch, s2d(50, 12.5, -500, -125), rect(10, 10, 10+20, 10+40)},
		{pt(1000, 500), pt(250, 200), rect(10, 10, 30, 50), Zoom, s2d(50, 50, -500, -1250), rect(10, 10, 10+20, 10+40)},
		{pt(1000, 500), pt(200, 250), rect(10, 10, 30, 50), Zoom, s2d(50, 50, -500, -1250), rect(10, 10, 10+20, 10+40)},
		{pt(1000, 500), pt(250, 200), rect(10, 10, 30, 50), Maximize, s2d(12.5, 12.5, 250, -125), rect(10, 10, 10+20, 10+40)},
		{pt(1000, 500), pt(200, 250), rect(10, 10, 30, 50), Maximize, s2d(12.5, 12.5, 250, -125), rect(10, 10, 10+20, 10+40)},
		{pt(1000, 500), pt(250, 200), rect(10, 10, 30, 50), Focus, s2d(4, 4, 0, 0), rect(0, 0, 250, 125)},
		{pt(1000, 500), pt(200, 250), rect(10, 10, 30, 50), Focus, s2d(5, 5, 0, 0), rect(0, 0, 200, 100)},

		// TODO: more cases
		// TODO: write output images (use a radial gradient) for verification?
	} {
		var td string
		td = fmt.Sprintf("%12s -> %-12s %10s", tc.sr, tc.dr, tc.mode)
		if tc.tr != none {
			td = fmt.Sprintf("%-26s %s", fmt.Sprintf("(focus %s)", tc.dr), td)
		} else {
			td = fmt.Sprintf("%-26s %s", "", td)
		}

		if tc.tr == none {
			tc.tr = image.Rectangle{pt(0, 0), tc.sr}
		}
		s2d, cr := transform(image.Rectangle{pt(0, 0), tc.dr}, image.Rectangle{pt(0, 0), tc.sr}, tc.tr, tc.mode)

		if tc.cr == (image.Rectangle{pt(0, 0), tc.sr}) {
			tc.cr = none
		}
		if cr == (image.Rectangle{pt(0, 0), tc.sr}) {
			cr = none
		}
		if s2d != tc.s2d || cr != tc.cr {
			t.Errorf("%s   ::   %-40s   !=   %-40s", td, s2ds(s2d, cr), s2ds(tc.s2d, tc.cr))
		}
	}
}
