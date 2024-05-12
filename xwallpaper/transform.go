package xwallpaper

import (
	"image"
	"strconv"

	"github.com/bamiaux/rez"
	"golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
)

// Mode specifies an algorithm for cropping, panning, and scaling the image.
type Mode int

const (
	// Tile crops the image to the area of interest then repeats it to cover the
	// output starting from the top-left corner of the screen.
	Tile Mode = iota

	// Center crops the image to the area of interest then centers it on the
	// output. If it is larger than the output, parts of it will be cut off.
	Center

	// Stretch crops the image to the area of interest then stretches it to
	// cover the output, ignoring the aspect ratio.
	Stretch

	// Zoom crops the image to the area of interest then zooms the image in or
	// out until the shortest dimension matches the output. If the aspect ratio
	// doesn't match the output, parts of it will be cut off.
	Zoom

	// Maximize crops the image to the area of interest then zooms the image in
	// or out as necessary to ensure the entire image is visible, centering it
	// if it is smaller.
	Maximize

	// Focus zooms the image out to ensure the entire area of interest is
	// visible, then zooms in if necessary to eliminate black areas. Unlike
	// [Maximize], the parts of the image outside of the area of interest are
	// not cropped.
	Focus
)

func (m Mode) String() string {
	switch m {
	case Tile:
		return "tile"
	case Center:
		return "center"
	case Stretch:
		return "stretch"
	case Zoom:
		return "zoom"
	case Maximize:
		return "maximize"
	case Focus:
		return "focus"
	default:
		return strconv.Itoa(int(m))
	}
}

// Draw transforms src using mode with trim as the area of interest, drawing it
// onto dst using the provided filter.
//
// If trim is the zero rectangle, it is set to the bounds of src.
func Draw(dst draw.Image, src image.Image, trim image.Rectangle, mode Mode, filter draw.Transformer) {
	var (
		dr = dst.Bounds()
		sr = src.Bounds()
		tr = trim
	)

	// use input dimensions as trim box if none specified, otherwise clamp it to input dimensions
	if tr == (image.Rectangle{}) {
		tr = sr
	} else {
		tr = tr.Intersect(sr)
	}

	// if tiling, we don't transform it
	if mode == Tile {
		tile(dst, dr, src, tr, draw.Over)
		return
	}

	// compute the transformation and new source rectangle
	s2d, tr := transform(dr, sr, tr, mode)

	// if we're only translating, we don't need a high-quality kernel
	if scaleX, scaleY := s2d[0], s2d[4]; scaleX == 1 && scaleY == 1 {
		filter = draw.NearestNeighbor
	}

	// transform the image
	filter.Transform(dst, s2d, src, tr, draw.Over, nil)
}

// DrawRez is like Draw, but uses a [rez.Filter] for better performance.
func DrawRez(dst draw.Image, src image.Image, trim image.Rectangle, mode Mode, filter rez.Filter) {
	Draw(dst, src, trim, mode, rezFilter{filter})
}

func tile(dst draw.Image, dr image.Rectangle, src image.Image, tr image.Rectangle, op draw.Op) {
	tr = tr.Canon()
	dr = dr.Canon()

	// tile onto dst until dr is covered
	for x := dr.Min.X; x < dr.Max.X; x += tr.Dx() {
		for y := dr.Min.Y; y < dr.Max.Y; y += tr.Dy() {
			draw.Draw(dst, tr.Add(image.Pt(x, y)), src, tr.Min, op)
		}
	}
}

func transform(dr image.Rectangle, sr image.Rectangle, tr image.Rectangle, mode Mode) (f64.Aff3, image.Rectangle) {
	sr = sr.Canon()
	dr = dr.Canon()
	tr = tr.Canon()

	// ensure trim box is at least 1x1
	if tr.Dx() == 0 {
		tr.Max.X++
	}
	if tr.Dy() == 0 {
		tr.Max.Y++
	}

	// save the input/output sizes
	var (
		sw, sh = sr.Dx(), sr.Dy()
		dw, dh = dr.Dx(), dr.Dy()
		tw, th = tr.Dx(), tr.Dy()
	)

	// if mode is focus, adjust trim box accordingly
	if mode == Focus {
		outRatio := float64(dw) / float64(dh)

		// calculate minimum box
		var mw, mh int
		if sw > dw && sh > dh {
			// input image is larger than output; use output dimensions (no zooming)
			mw = dw
			mh = dh
		} else {
			// zoom in, preserve output aspect ratio
			if float64(dw)/float64(sw) < float64(dh)/float64(sh) {
				mw = max(1, int(float64(sh)*outRatio))
				mh = sh
			} else {
				mw = sw
				mh = max(1, int(float64(sw)/outRatio))
			}
		}

		// if trim doesn't fit into minimum box, zoom out to cover (may cause black borders)
		if tw > mw || th > mh {
			// zoom out, preserve output aspect ratio
			if float64(sw)/float64(dw) < float64(sh)/float64(dh) {
				tw = max(1, int(float64(sh)*outRatio))
				th = sh
			} else {
				tw = sw
				th = max(1, int(float64(sw)*outRatio))
			}
		} else {
			tw = mw
			th = mh
		}

		// find offsets, keeping the entire trim box visible even at the cost of black borders
		var (
			tx = max(0, tr.Min.X-(tw-tr.Dx())/2)
			ty = max(0, tr.Min.Y-(th-tr.Dy())/2)
		)
		if tw > sw-tx {
			if tw > sw {
				tx = (sw - tw) / 2
			} else {
				tx = sw - tw
			}
		}
		if th > sh-ty {
			if th > sh {
				ty = (sh - th) / 2
			} else {
				ty = sh - th
			}
		}
		tr = image.Rect(tx, ty, tx+tw, ty+th)

		// now, we just need to scale to ensure our adjusted trim box is visible
		mode = Maximize
	}

	// compute scale
	var (
		scaleX = float64(dw) / float64(tw)
		scaleY = float64(dh) / float64(th)
	)
	switch mode {
	case Center:
		scaleX, scaleY = 1, 1
	case Maximize:
		if scaleX > scaleY {
			scaleX = scaleY
		} else {
			scaleY = scaleX
		}
	case Zoom:
		if scaleX < scaleY {
			scaleX = scaleY
		} else {
			scaleY = scaleX
		}
	case Stretch:
		// default
	}

	// compute translation to center the scaled trim box
	var (
		transX = (float64(dw)-float64(tw)*scaleX)/2 - float64(tr.Min.X)*scaleX
		transY = (float64(dh)-float64(th)*scaleY)/2 - float64(tr.Min.Y)*scaleY
	)

	// build the matrix
	s2d := f64.Aff3{
		scaleX, 0, transX,
		0, scaleY, transY,
	}
	return s2d, tr
}

type rezFilter struct {
	Filter rez.Filter
}

func (t rezFilter) Transform(dst draw.Image, s2d f64.Aff3, src image.Image, sr image.Rectangle, op draw.Op, opts *draw.Options) {
	if opts != nil {
		panic("unsupported transform options")
	}

	// skew
	if s2d[1] != 0 || s2d[3] != 0 {
		panic("unsupported transform")
	}

	// convert + crop
	var (
		r1 = sr.Canon()
		t1 = image.NewRGBA(r1)
	)
	draw.Draw(t1, r1, src, r1.Min, draw.Src)

	// scale
	var (
		r2 = image.Rect(
			int(s2d[2]),
			int(s2d[5]),
			int(s2d[2]+float64(r1.Dx())*s2d[0]),
			int(s2d[5]+float64(r1.Dy())*s2d[4]),
		)
		t2 = image.NewRGBA(r2)
	)
	if err := rez.Convert(t2, t1, t.Filter); err != nil {
		panic(err)
	}

	// composite + convert
	draw.Draw(dst, r2, t2, r2.Min, op)
}
