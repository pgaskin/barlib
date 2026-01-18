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
	Group    []int // [cpu]group (0 to skip)
	Interval time.Duration
}

func (c CPU) Run(i barlib.Instance) error {
	i.Tick(c.Interval)
	var ngrp int
	for _, g := range c.Group {
		if g > 0 {
			ngrp = max(ngrp, g)
		}
	}
	var (
		expanded bool
		prevAgg  []cpuTime
		prev     []cpuTime
	)
	if ngrp != 0 {
		prevAgg = make([]cpuTime, ngrp)
	} else {
		prevAgg = make([]cpuTime, 1)
	}
	for ticker, isEvent := i.Tick(c.Interval), false; ; {
		if !i.IsStopped() {
			var (
				agg, cpus, err = getCPUTime()
				usage          []float64
			)
			if err == nil {
				var grp []cpuTime
				if ngrp != 0 {
					grp = make([]cpuTime, ngrp)
					for i, t := range cpus {
						if i < len(c.Group) {
							if g := c.Group[i]; g > 0 {
								grp[g-1] = grp[g-1].Add(t)
							}
						}
					}
				}
				if expanded {
					if len(prev) == len(cpus) {
						usage = make([]float64, len(cpus))
						for i, cur := range cpus {
							usage[i] = cur.Usage(prev[i])
						}
					}
				} else if ngrp != 0 {
					usage = make([]float64, ngrp)
					for i, cur := range grp {
						usage[i] = cur.Usage(prevAgg[i])
					}
				} else {
					usage = []float64{agg.Usage(prevAgg[0])}
				}
				if ngrp != 0 {
					prevAgg = grp
				} else {
					prevAgg[0] = agg
				}
				prev = cpus
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
			case <-ticker:
			case <-i.Stopped():
			case event := <-i.Event():
				switch event.Button {
				default:
					continue
				case 1:
					isEvent = true
				case 2:
					if niri {
						nirimsg("action", "spawn", "--", "foot", "--app-id=htop", "--title=htop", "htop", "--sort-key=PERCENT_CPU")
					} else {
						i3msg(`exec --no-startup-id xfce4-terminal --hide-scrollbar --hide-menubar --dynamic-title-mode none --title htop -e 'htop --sort-key=PERCENT_CPU'`)
					}
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

// https://stackoverflow.com/a/23376195
func (t cpuTime) Usage(p cpuTime) float64 {
	var (
		idle1 = p.Idle + p.IOWait
		idle2 = t.Idle + t.IOWait
		rest1 = p.User + p.Nice + p.System + p.IRQ + p.SoftIRQ + p.Steal
		rest2 = t.User + t.Nice + t.System + t.IRQ + t.SoftIRQ + t.Steal
		tot1  = idle1 + rest1
		tot2  = idle2 + rest2
		totD  = tot2 - tot1
		idleD = idle2 - idle1
	)
	return (float64(totD) - float64(idleD)) / float64(totD)
}

func (t cpuTime) Add(o cpuTime) cpuTime {
	t.User += o.User
	t.Nice += o.Nice
	t.System += o.System
	t.Idle += o.Idle
	t.IOWait += o.IOWait
	t.IRQ += o.IRQ
	t.SoftIRQ += o.SoftIRQ
	t.Steal += o.Steal
	t.Guest += o.Guest
	t.GuestNice += o.GuestNice
	return t
}

func getCPUTime() (cpuTime, []cpuTime, error) {
	buf, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuTime{}, nil, err
	}
	var time []cpuTime
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
						return cpuTime{}, nil, fmt.Errorf("invalid cpu index %q", v)
					} else {
						n = int(v + 1)
					}
				}
				if exp := len(time); exp != n {
					return cpuTime{}, nil, fmt.Errorf("expected cpu %d, got %d", exp, n)
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
				return cpuTime{}, nil, err
			} else {
				*out = v
			}
		}
		time = append(time, t)
	}
	if len(time) < 2 {
		return cpuTime{}, nil, fmt.Errorf("expected at least one CPU")
	}
	return time[0], time[1:], sc.Err()
}
