// Package barproto implements the i3bar protocol.
//
// https://i3wm.org/docs/i3bar-protocol.html
package barproto

import (
	"slices"
	"strconv"
	"syscall"

	"github.com/BurntSushi/xgb/xproto"
	"github.com/tidwall/gjson"
)

const Version = 1 // i3 v4.3+, last tested on v4.23

// Init represents an i3bar initialization message.
type Init struct {
	StopSignal  syscall.Signal
	ContSignal  syscall.Signal
	ClickEvents bool
}

func (x Init) MarshalJSON() ([]byte, error) {
	return x.AppendJSON(nil), nil
}

func (x Init) AppendJSON(s []byte) []byte {
	s = append(s, `{"version":`...)
	s = strconv.AppendInt(s, int64(Version), 10)
	if v := x.StopSignal; v != 0 {
		s = append(s, `,"stop_signal":`...)
		s = strconv.AppendInt(s, int64(v), 10)
	}
	if v := x.ContSignal; v != 0 {
		s = append(s, `,"cont_signal":`...)
		s = strconv.AppendInt(s, int64(v), 10)
	}
	if x.ClickEvents {
		s = append(s, `,"click_events":true`...)
	}
	s = append(s, '}')
	return s
}

// Event represents an i3bar event.
type Event struct {
	Name      string
	Instance  string
	Button    int // xproto.ButtonIndex*
	Modifiers int // xproto.ModMask*
	X         int
	Y         int
	RelativeX int
	RelativeY int
	OutputX   int
	OutputY   int
	Width     int
	Height    int
}

// FromJSON parses b without any error checking.
func (e *Event) FromJSON(b []byte) {
	var event Event
	gjson.ParseBytes(b).ForEach(func(key, value gjson.Result) bool {
		switch key.Str {
		case "name":
			event.Name = value.Str
		case "instance":
			event.Instance = value.Str
		case "button":
			event.Button = int(value.Int())
		case "modifiers":
			// https://github.com/i3/i3/blob/69f68dcd74df1ef306c3459558363d48fdda87d2/i3bar/src/child.c#L850 (send_block_clicked)
			// xmodmap
			value.ForEach(func(_, value gjson.Result) bool {
				switch value.Str {
				case "Shift":
					event.Modifiers |= xproto.ModMaskShift
				case "Control":
					event.Modifiers |= xproto.ModMaskControl
				case "Mod1": // Alt
					event.Modifiers |= xproto.ModMask1
				case "Mod2":
					event.Modifiers |= xproto.ModMask2
				case "Mod3":
					event.Modifiers |= xproto.ModMask3
				case "Mod4": // Super
					event.Modifiers |= xproto.ModMask4
				case "Mod5":
					event.Modifiers |= xproto.ModMask5
				}
				return true
			})
		case "x":
			event.X = int(value.Int())
		case "y":
			event.Y = int(value.Int())
		case "relative_x":
			event.RelativeX = int(value.Int())
		case "relative_y":
			event.RelativeY = int(value.Int())
		case "output_x":
			event.OutputX = int(value.Int())
		case "output_y":
			event.OutputY = int(value.Int())
		case "width":
			event.Width = int(value.Int())
		case "height":
			event.Height = int(value.Int())
		}
		return true
	})
	*e = event
}

// Block represents an i3bar block.
type Block struct {
	Name                string // optional, passed as-is for events
	Instance            string // optional, passed as-is for events
	FullText            string // text
	ShortText           string // optional
	Color               uint32 // 0xRRGGBBAA (AA should be 0xFF for solid colors) (0x00000000 is treated as i3bar's default)
	Background          uint32 // ^
	Border              uint32 // ^
	BorderTop           int    // pixels (0 is treated as i3bar's default of 1, set to -1 to disable the border)
	BorderRight         int    // ^
	BorderBottom        int    // ^
	BorderLeft          int    // ^
	MinWidth            int    // pixels (0 is none)
	MinWidthString      string // overrides MinWidth with the width of the specified text if not empty
	Align               string // left|center|right, used if smaller than MinWidth
	Urgent              bool   // used by i3bar
	Separator           bool   // whether to draw a separator line after the block
	SeparatorBlockWidth int    // pixels, should be odd since line is in the middle (if Separator is true, 0 is treated as the i3bar's value, otherwise -1 is)
	Pango               bool   // whether to use pango markup
}

func (b Block) MarshalJSON() ([]byte, error) {
	return b.AppendJSON(nil), nil
}

func (b Block) AppendJSON(s []byte) []byte {
	s = append(s, `{"full_text":`...)
	s = jsonString(s, b.FullText)
	if v := b.ShortText; v != "" {
		s = append(s, `,"short_text":`...)
		s = jsonString(s, v)
	}
	if v := b.Color; v != 0 {
		s = append(s, `,"color":"`...)
		s = hexColor(s, v)
		s = append(s, '"')
	}
	if v := b.Name; v != "" {
		s = append(s, `,"name":`...)
		s = jsonString(s, v)
	}
	if v := b.Instance; v != "" {
		s = append(s, `,"instance":`...)
		s = jsonString(s, v)
	}
	if v := b.Background; v != 0 {
		s = append(s, `,"background":"`...)
		s = hexColor(s, v)
		s = append(s, '"')
	}
	if v := b.Border; v != 0 {
		s = append(s, `,"border":"`...)
		s = hexColor(s, v)
		s = append(s, '"')
	}
	if v := b.BorderTop; v != 0 {
		if v == -1 {
			v = 0
		}
		s = append(s, `,"border_top":`...)
		s = strconv.AppendInt(s, int64(v), 10)
	}
	if v := b.BorderRight; v != 0 {
		if v == -1 {
			v = 0
		}
		s = append(s, `,"border_right":`...)
		s = strconv.AppendInt(s, int64(v), 10)
	}
	if v := b.BorderBottom; v != 0 {
		if v == -1 {
			v = 0
		}
		s = append(s, `,"border_bottom":`...)
		s = strconv.AppendInt(s, int64(v), 10)
	}
	if v := b.BorderLeft; v != 0 {
		if v == -1 {
			v = 0
		}
		s = append(s, `,"border_left":`...)
		s = strconv.AppendInt(s, int64(v), 10)
	}
	if v := b.MinWidthString; v != "" {
		s = append(s, `,"min_width":`...)
		s = jsonString(s, v)
	} else if v := b.MinWidth; v != 0 {
		s = append(s, `,"min_width":`...)
		s = strconv.AppendInt(s, int64(v), 10)
	}
	if v := b.Align; v != "" {
		s = append(s, `,"align":`...)
		s = jsonString(s, v)
	}
	if b.Urgent {
		s = append(s, `,"urgent":true`...)
	}
	if b.Separator {
		s = append(s, `,"separator":true`...)
		if v := b.SeparatorBlockWidth; v > 0 {
			s = append(s, `,"separator_block_width":`...)
			s = strconv.AppendInt(s, int64(v), 10)
		}
	} else {
		s = append(s, `,"separator":false`...)
		if v := b.SeparatorBlockWidth; v >= 0 {
			s = append(s, `,"separator_block_width":`...)
			s = strconv.AppendInt(s, int64(v), 10)
		}
	}
	if b.Pango {
		s = append(s, `,"markup":"pango"`...)
	}
	s = append(s, '}')
	return s
}

func hexColor(b []byte, rrggbbaa uint32) []byte {
	const hex = "0123456789ABCDEF"
	b = slices.Grow(b, 9)
	b = append(b, '#')
	b = append(b, hex[(rrggbbaa>>28)&0xF])
	b = append(b, hex[(rrggbbaa>>24)&0xF])
	b = append(b, hex[(rrggbbaa>>20)&0xF])
	b = append(b, hex[(rrggbbaa>>16)&0xF])
	b = append(b, hex[(rrggbbaa>>12)&0xF])
	b = append(b, hex[(rrggbbaa>>8)&0xF])
	if rrggbbaa&0xFF != 0xFF {
		b = append(b, hex[(rrggbbaa>>4)&0xF])
		b = append(b, hex[(rrggbbaa>>0)&0xF])
	}
	return b
}

func jsonString[T ~[]byte | ~string](b []byte, s T) []byte {
	b = slices.Grow(b, len(s)+2)
	b = append(b, '"')
	x := 0 // note: this won't break utf-8 since we only check for < 0x20
	for i := 0; i < len(s); {
		if c := s[i]; c < 0x20 || c == '\\' || c == '"' {
			b = append(b, s[x:i]...)
			switch c {
			case '\\', '"':
				b = append(b, '\\', c)
			case '\n':
				b = append(b, '\\', 'n')
			case '\r':
				b = append(b, '\\', 'r')
			case '\t':
				b = append(b, '\\', 't')
			default:
				b = append(b, '\\', 'u', '0', '0', "0123456789abcdef"[c>>4], "0123456789abcdef"[c&0xF])
			}
			i++
			x = i
			continue
		}
		i++
	}
	b = append(b, s[x:]...)
	b = append(b, '"')
	return b
}
