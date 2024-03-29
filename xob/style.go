package xob

type Anchor int

const (
	AnchorTopLeft Anchor = iota
	AnchorTopCenter
	AnchorTopRight
	AnchorMiddleLeft
	AnchorMiddleCenter
	AnchorMiddleRight
	AnchorBottomLeft
	AnchorBottomCenter
	AnchorBottomRight
)

type style struct {
	X           dim
	Y           dim
	Length      dim
	Thickness   int
	Border      int
	Padding     int
	Outline     int
	Orientation Orientation
	Overflow    OverflowMode
	Color       styleColor
}

type styleColor struct {
	Normal      colorspec
	Overflow    colorspec
	Alt         colorspec
	AltOverflow colorspec
}

func (s style) WithX(rel float64, abs int) style {
	s.X = dim{rel, abs}
	return s
}

func (s style) WithY(rel float64, abs int) style {
	s.Y = dim{rel, abs}
	return s
}

func (s style) WithLength(rel float64, abs int) style {
	s.Length = dim{rel, abs}
	return s
}

func (s style) WithThickness(thickness int) style {
	s.Thickness = thickness
	return s
}

func (s style) WithBorder(border int) style {
	s.Border = border
	return s
}

func (s style) WithPadding(padding int) style {
	s.Padding = padding
	return s
}

func (s style) WithOutline(outline int) style {
	s.Outline = outline
	return s
}

func (s style) WithOrientation(orientation Orientation) style {
	s.Orientation = orientation
	return s
}

func (s style) WithColorNormal(fg, bg, border string) style {
	s.Color.Normal = colorspec{fg, bg, border}
	return s
}

func (s style) WithColorOverflow(fg, bg, border string) style {
	s.Color.Overflow = colorspec{fg, bg, border}
	return s
}

func (s style) WithColorAlt(fg, bg, border string) style {
	s.Color.Alt = colorspec{fg, bg, border}
	return s
}

func (s style) WithColorAltOverflow(fg, bg, border string) style {
	s.Color.AltOverflow = colorspec{fg, bg, border}
	return s
}

func (s style) Anchor(anchor Anchor, offset int) style {
	switch anchor {
	case AnchorTopLeft:
		s.X = dim{0.0, offset}
		s.Y = dim{0.0, offset}
	case AnchorTopCenter:
		s.X = dim{0.5, 0}
		s.Y = dim{0.0, offset}
	case AnchorTopRight:
		s.X = dim{1.0, -offset}
		s.Y = dim{0.0, offset}
	case AnchorMiddleLeft:
		s.X = dim{0.0, offset}
		s.Y = dim{0.5, 0}
	case AnchorMiddleCenter:
		s.X = dim{0.5, 0}
		s.Y = dim{0.5, 0}
	case AnchorMiddleRight:
		s.X = dim{1.0, -offset}
		s.Y = dim{0.5, 0}
	case AnchorBottomLeft:
		s.X = dim{0.0, offset}
		s.Y = dim{1.0, -offset}
	case AnchorBottomCenter:
		s.X = dim{0.5, 0}
		s.Y = dim{1.0, -offset}
	case AnchorBottomRight:
		s.X = dim{1.0, -offset}
		s.Y = dim{1.0, -offset}
	}
	return s
}

func DefaultStyle() style {
	return style{
		X: dim{
			Rel: 1.0,
			Abs: -48,
		},
		Y: dim{
			Rel: 0.5,
			Abs: 0,
		},
		Length: dim{
			Rel: 0.3,
			Abs: 0,
		},
		Thickness:   24,
		Border:      4,
		Padding:     3,
		Outline:     3,
		Orientation: OrientationVertical,
		Color: styleColor{
			Normal: colorspec{
				FG:     "#ffffff",
				BG:     "#000000",
				Border: "#ffffff",
			},
			Overflow: colorspec{
				FG:     "#ff0000",
				BG:     "#000000",
				Border: "#ff0000",
			},
			Alt: colorspec{
				FG:     "#555555",
				BG:     "#000000",
				Border: "#555555",
			},
			AltOverflow: colorspec{
				FG:     "#550000",
				BG:     "#000000",
				Border: "#550000",
			},
		},
	}
}
func I3Style() style {
	return style{
		X: dim{
			Rel: 1.0,
			Abs: -48,
		},
		Y: dim{
			Rel: 0.5,
			Abs: 0,
		},
		Length: dim{
			Rel: 0.3,
			Abs: 0,
		},
		Thickness:   24,
		Border:      1,
		Padding:     0,
		Outline:     0,
		Orientation: OrientationVertical,
		Color: styleColor{
			Normal: colorspec{
				FG:     "#285577",
				BG:     "#222222",
				Border: "#4c7899",
			},
			Alt: colorspec{
				FG:     "#333333",
				BG:     "#222222",
				Border: "#444444",
			},
			Overflow: colorspec{
				FG:     "#900000",
				BG:     "#222222",
				Border: "#933333",
			},
			AltOverflow: colorspec{
				FG:     "#900000",
				BG:     "#222222",
				Border: "#444444",
			},
		},
	}
}
