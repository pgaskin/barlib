package barlib_test

import (
	"fmt"
	"strconv"
	"time"

	"github.com/BurntSushi/xgb/xproto"
	"github.com/pgaskin/barlib"
	"github.com/pgaskin/barlib/barproto"
)

type Example struct {
	Text string
	Rate time.Duration
}

func (c Example) Run(i barlib.Instance) error {
	var (
		count  int64
		paused bool
	)
	for ticker, isEvent := i.Tick(c.Rate), false; ; {
		i.Update(isEvent, func(render barlib.Renderer) {
			var color uint32
			if paused {
				color = 0xFFFF00FF
			} else {
				color = 0x00FF00FF
			}
			render(barproto.Block{
				Instance:  "text",
				FullText:  c.Text + " ",
				Separator: false,
				Color:     color,
			})
			switch {
			case count < 0:
				color = 0x000088FF
			case count > 0:
				color = 0x008800FF
			default:
				color = 0x880000FF
			}
			render(barproto.Block{
				Instance:       "count",
				FullText:       strconv.FormatInt(count, 10),
				MinWidthString: "0000",
				Align:          "center",
				Separator:      true,
				Background:     color,
				BorderLeft:     4,
				BorderRight:    4,
				Border:         color,
			})
		})

		select {
		case <-i.Stopped():
			i.Debug("stopped=%t", i.IsStopped())
		case <-ticker:
			if !paused {
				count += 1
			}
		case event := <-i.Event():
			switch event.Instance {
			case "text":
				switch event.Button {
				case 1:
					paused = !paused
				case 2:
					return fmt.Errorf("fake error")
				case 3:
					count = 0
				case 4:
					i.Tick(c.Rate)
				case 5:
					i.Tick(c.Rate / 2)
				}
			case "count":
				switch event.Button {
				case 1:
					if event.Modifiers&xproto.ModMaskShift != 0 {
						count--
					} else {
						count++
					}
				case 2:
					return fmt.Errorf("fake error")
				case 3:
					count = 0
				case 4:
					count++
				case 5:
					count--
				}
			}
			isEvent = true
		}
	}
}

func ExampleMain() {
	barlib.Main(time.Second/4,
		Example{
			Text: "A",
			Rate: time.Second,
		},
		Example{
			Text: "B",
			Rate: time.Second,
		},
		Example{
			Text: "C",
			Rate: time.Second / 2,
		},
	)
}
