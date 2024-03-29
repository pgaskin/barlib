// # disk
//
// Shows the disk usage of the specified mountpoint. Updates using polling at
// the configured interval. Starts gnome-disks on middle-click.
package main

import (
	"errors"
	"io/fs"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/pgaskin/barlib"
	"github.com/pgaskin/barlib/barproto"
	"golang.org/x/sys/unix"
)

type Disk struct {
	Interval       time.Duration
	Threshold      uint64
	ThresholdColor uint32
	Mountpoint     string
}

func (c Disk) Run(i barlib.Instance) error {
	i.Tick(c.Interval)
	var expanded bool
	for isEvent := false; ; {
		if !i.IsStopped() {
			var stat unix.Statfs_t
			err := unix.Statfs(c.Mountpoint, &stat)
			i.Update(isEvent, func(render barlib.Renderer) {
				if err != nil {
					if errors.Is(err, fs.ErrNotExist) {
						render(barproto.Block{
							FullText:  "?",
							Color:     0xFF0000FF,
							Separator: true,
						})
					} else {
						render.Err(err)
					}
					return
				}
				available := stat.Bavail * uint64(stat.Bsize)
				block := barproto.Block{
					FullText:  humanize.IBytes(available),
					Separator: true,
				}
				if expanded {
					block.FullText += " / " + humanize.IBytes(stat.Blocks*uint64(stat.Bsize))
				}
				if available <= c.Threshold {
					block.Color = c.ThresholdColor
				}
				render(block)
			})
		}
		for isEvent = false; ; {
			select {
			case <-i.Ticked():
			case <-i.Stopped():
			case event := <-i.Event():
				switch event.Button {
				default:
					continue
				case 1:
					isEvent = true
				case 2:
					go i3msg(`exec --no-startup-id gnome-disks`)
					continue
				case 3:
					expanded = !expanded
					isEvent = true
				}
			}
			break
		}
	}
}
