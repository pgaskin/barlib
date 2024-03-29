// # cpu
//
// Shows the CPU usage percent over the specified interval, optionally expanding
// to show all CPUs. Reads from procfs. Starts htop sorted by CPU usage on
// middle-click.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pgaskin/barlib"
	"github.com/pgaskin/barlib/barproto"
)

type CPU struct {
	Interval time.Duration
}

func (c CPU) Run(i barlib.Instance) error {
	i.Tick(c.Interval)
	var (
		expanded bool
		prev     []cpuTime
	)
	for isEvent := false; ; {
		if !i.IsStopped() {
			var (
				stats, err = getCPUTime()
				usage      = make([]float64, len(stats))
			)
			if err == nil {
				if !expanded {
					usage = usage[:1]
				}
				if len(prev) == len(stats) {
					for i, cur := range stats {
						if i >= len(usage) {
							break
						}
						// https://stackoverflow.com/a/23376195
						var (
							tmp   = prev[i]
							idle1 = tmp.Idle + tmp.IOWait
							idle2 = cur.Idle + cur.IOWait
							rest1 = tmp.User + tmp.Nice + tmp.System + tmp.IRQ + tmp.SoftIRQ + tmp.Steal
							rest2 = cur.User + cur.Nice + cur.System + cur.IRQ + cur.SoftIRQ + cur.Steal
							tot1  = idle1 + rest1
							tot2  = idle2 + rest2
							totD  = tot2 - tot1
							idleD = idle2 - idle1
						)
						usage[i] = (float64(totD) - float64(idleD)) / float64(totD)
					}
				}
				prev = stats
			}
			i.Update(isEvent, func(render barlib.Renderer) {
				if err != nil {
					render.Err(err)
					return
				}
				b := make([]byte, 0, len(usage)*6-1)
				for i, pct := range usage {
					if i != 0 {
						b = append(b, ' ')
					}
					v := int64(math.Round(pct * 100))
					if v < 10 {
						b = append(b, '0')
					}
					b = strconv.AppendInt(b, v, 10)
					b = append(b, '%')
				}
				render(barproto.Block{
					FullText:  string(b),
					Separator: true,
				})
			})
		}
		for {
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
					go i3msg(`exec --no-startup-id xfce4-terminal --hide-scrollbar --hide-menubar --dynamic-title-mode none --title htop -e 'htop --sort-key=PERCENT_CPU'`)
				case 3:
					expanded = !expanded
				}
			}
			break
		}
	}
}

type cpuTime struct {
	User      uint64
	Nice      uint64
	System    uint64
	Idle      uint64
	IOWait    uint64
	IRQ       uint64
	SoftIRQ   uint64
	Steal     uint64
	Guest     uint64
	GuestNice uint64
}

func getCPUTime() ([]cpuTime, error) {
	var time []cpuTime
	buf, err := os.ReadFile("/proc/stat")
	if err != nil {
		return time, err
	}
	sc := bufio.NewScanner(bytes.NewReader(buf))
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "cpu") {
			break
		}
		var t cpuTime
		for i, v := range strings.Fields(line) {
			var out *uint64
			switch i {
			case 0:
				var n int
				if v := v[3:]; v != "" {
					if v, err := strconv.ParseUint(v, 10, 64); err != nil {
						return nil, fmt.Errorf("invalid cpu index %q", v)
					} else {
						n = int(v + 1)
					}
				}
				if exp := len(time); exp != n {
					return nil, fmt.Errorf("expected cpu %d, got %d", exp, n)
				}
				continue
			case 1:
				out = &t.User
			case 2:
				out = &t.Nice
			case 3:
				out = &t.System
			case 4:
				out = &t.Idle
			case 5:
				out = &t.IOWait
			case 6:
				out = &t.IRQ
			case 7:
				out = &t.SoftIRQ
			case 8:
				out = &t.Steal
			case 9:
				out = &t.Guest
			case 10:
				out = &t.GuestNice
			default:
				continue
			}
			if v, err := strconv.ParseUint(v, 10, 64); err != nil {
				return nil, err
			} else {
				*out = v
			}
		}
		time = append(time, t)
	}
	return time, sc.Err()
}
