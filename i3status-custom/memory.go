// # memory
//
// Polls memory usage from procfs at the configured interval. Shows used memory
// similar to the "free" command. Starts htop sorted by memory usage on
// middle-click.
package main

import (
	"bufio"
	"bytes"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/pgaskin/barlib"
	"github.com/pgaskin/barlib/barproto"
)

type Memory struct {
	Interval       time.Duration
	Threshold      uint64
	ThresholdColor uint32
}

func (c Memory) Run(i barlib.Instance) error {
	i.Tick(c.Interval)
	var expanded bool
	for isEvent := false; ; {
		if !i.IsStopped() {
			stats, err := getMemInfo()
			i.Update(isEvent, func(render barlib.Renderer) {
				if err != nil {
					render(barproto.Block{
						FullText:  err.Error(),
						Urgent:    true,
						Separator: true,
					})
					return
				}
				used := stats.MemTotal - stats.MemFree - stats.Buffers - stats.Cached
				block := barproto.Block{
					FullText:  humanize.IBytes(used),
					Separator: true,
				}
				if expanded {
					block.FullText += " / " + humanize.IBytes(stats.MemTotal)
				}
				if used+c.Threshold >= stats.MemTotal {
					block.Color = c.ThresholdColor
				}
				render(block)
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
					go i3msg(`exec --no-startup-id xfce4-terminal --hide-scrollbar --hide-menubar --dynamic-title-mode none --title htop -e 'htop --sort-key=PERCENT_MEM'`)
				case 3:
					expanded = !expanded
				}
			}
			break
		}
	}
}

type memInfo struct {
	MemTotal     uint64
	MemFree      uint64
	MemAvailable uint64
	Buffers      uint64
	Cached       uint64
	SwapCached   uint64
	Active       uint64
	Inactive     uint64
	SwapTotal    uint64
	SwapFree     uint64
	Mapped       uint64
	Shmem        uint64
	Slab         uint64
	PageTables   uint64
}

func getMemInfo() (memInfo, error) {
	var info memInfo
	buf, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return info, err
	}
	sc := bufio.NewScanner(bytes.NewReader(buf))
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		var out *uint64
		switch k {
		case "MemTotal":
			out = &info.MemTotal
		case "MemFree":
			out = &info.MemFree
		case "MemAvailable":
			out = &info.MemAvailable
		case "Buffers":
			out = &info.Buffers
		case "Cached":
			out = &info.Cached
		case "SwapCached":
			out = &info.SwapCached
		case "Active":
			out = &info.Active
		case "Inactive":
			out = &info.Inactive
		case "SwapTotal":
			out = &info.SwapTotal
		case "SwapFree":
			out = &info.SwapFree
		case "Mapped":
			out = &info.Mapped
		case "Shmem":
			out = &info.Shmem
		case "Slab":
			out = &info.Slab
		case "PageTables":
			out = &info.PageTables
		default:
			continue
		}
		v, kB := strings.CutSuffix(v, " kB")
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			*out = n
		}
		if kB {
			*out = *out * 1024
		}
	}
	return info, sc.Err()
}
