// # pulseaudio
//
// Controls sink/source volume/mute/default using the PulseAudio native API. Has
// reasonable thresholds for the volume step. Starts pavucontrol with the
// sink/source tab selected on middle-click.
package main

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"time"

	"github.com/pgaskin/barlib"
	"github.com/pgaskin/barlib/barproto"
	"github.com/pgaskin/barlib/pulseaudio"
)

type PulseAudio struct {
	ShowSink   bool
	ShowSource bool
}

func (c PulseAudio) Run(i barlib.Instance) error {
	cl, err := pulseaudio.NewClient()
	if err != nil {
		return err
	}
	defer cl.Close()

	ch, err := cl.UpdatesByType(pulseaudio.SUBSCRIPTION_MASK_SERVER | pulseaudio.SUBSCRIPTION_MASK_SINK | pulseaudio.SUBSCRIPTION_MASK_SOURCE)
	if err != nil {
		return err
	}
	var (
		snkExp, srcExp bool
		snkSel, srcSel string
	)
	for {
		if !cl.Connected() {
			return fmt.Errorf("disconnected")
		}
		var (
			snkDef, srcDef string
			snkVol, srcVol []int
			snk            []pulseaudio.Sink
			src            []pulseaudio.Source
			snkIdx         = -1
			srcIdx         = -1
		)
		if inf, err := cl.ServerInfo(); err != nil {
			return err
		} else {
			if c.ShowSink {
				if snk, err = cl.Sinks(); err != nil {
					return err
				} else {
					if snkSel != "" {
						snkIdx = slices.IndexFunc(snk, func(snk pulseaudio.Sink) bool {
							return snk.Name == snkSel
						})
					}
					if snkIdx == -1 {
						snkIdx = slices.IndexFunc(snk, func(snk pulseaudio.Sink) bool {
							return snk.Name == inf.DefaultSink
						})
					}
					if snkIdx == -1 {
						snkIdx = slices.IndexFunc(snk, func(snk pulseaudio.Sink) bool {
							return true
						})
					}
					if snkIdx == -1 && len(snk) != 0 {
						snkIdx = 0
					}
					snkVol = make([]int, len(snk))
					for i, s := range snk {
						snkVol[i] = int(math.Round(float64(s.GetVolume()) * 100))
					}
					snkDef = inf.DefaultSink
				}
			}
			if c.ShowSource {
				if src, err = cl.Sources(); err != nil {
					return err
				} else {
					if srcSel != "" {
						srcIdx = slices.IndexFunc(src, func(src pulseaudio.Source) bool {
							return src.Name == srcSel
						})
					}
					if srcIdx == -1 {
						srcIdx = slices.IndexFunc(src, func(src pulseaudio.Source) bool {
							return src.Name == inf.DefaultSource
						})
					}
					if srcIdx == -1 {
						srcIdx = slices.IndexFunc(src, func(src pulseaudio.Source) bool {
							return src.MonitorSourceName == ""
						})
					}
					srcVol = make([]int, len(src))
					for i, s := range src {
						srcVol[i] = int(math.Round(float64(s.GetVolume()) * 100))
					}
					srcDef = inf.DefaultSource
				}
			}
		}
	render:
		i.Update(true, func(render barlib.Renderer) {
			if snkIdx != -1 {
				s := snk[snkIdx]
				{
					block := barproto.Block{
						Instance: "snk_ic",
						FullText: " ",
					}
					if s.Muted {
						block.Color = 0xFFFF00FF
					} else {
						block.Color = 0x00FF00FF
					}
					render(block)
				}
				if snkExp {
					block := barproto.Block{
						Instance: "snk_sel",
						FullText: s.Description + " ",
					}
					if snkDef == s.Name {
						block.Color = 0x00FF00FF
					} else {
						block.Color = 0xFFFF00FF
					}
					render(block)
				}
				{
					block := barproto.Block{
						Instance:  "snk_vol",
						FullText:  strconv.Itoa(snkVol[snkIdx]),
						Separator: true,
					}
					if s.Muted {
						block.FullText += "-"
						block.Color = 0xFFFF00FF
					} else {
						block.FullText += "%"
						block.Color = 0x00FF00FF
					}
					render(block)
				}
			}
			if srcIdx != -1 {
				s := src[srcIdx]
				{
					block := barproto.Block{
						Instance:  "src_ic",
						FullText:  " ",
						Separator: false,
					}
					if s.Muted {
						block.Color = 0xFFFF00FF
					} else {
						block.Color = 0x00FF00FF
					}
					render(block)
				}
				if srcExp {
					block := barproto.Block{
						Instance: "src_sel",
						FullText: s.Description + " ",
					}
					if srcDef == s.Name {
						block.Color = 0x00FF00FF
					} else {
						block.Color = 0xFFFF00FF
					}
					render(block)
				}
				{
					block := barproto.Block{
						Instance:  "src_vol",
						FullText:  strconv.Itoa(srcVol[srcIdx]),
						Separator: true,
					}
					if s.Muted {
						block.FullText += "-"
						block.Color = 0xFFFF00FF
					} else {
						block.FullText += "%"
						block.Color = 0x00FF00FF
					}
					render(block)
				}
			}
		})
		for {
			select {
			case <-ch:
				time.Sleep(time.Millisecond * 8) // debounce
				for {
					select {
					case <-ch:
						continue
					default:
					}
					break
				}
			case event := <-i.Event():
				if !cl.Connected() {
					return fmt.Errorf("disconnected")
				}
				var err error
				switch event.Instance {
				case "snk_ic", "snk_vol", "snk_sel":
					if snkIdx != -1 {
						s := snk[snkIdx]
						switch event.Button {
						case 1:
							if event.Instance == "snk_sel" {
								err = cl.SetDefaultSink(s.Name)
							} else {
								err = cl.SetSinkMute(s.Name, !s.Muted)
							}
						case 2:
							go i3msg(`exec --no-startup-id pavucontrol --tab=3`)
						case 3:
							if snkExp = !snkExp; !snkExp {
								snkSel = ""
							}
							goto render // re-render without getting new data
						case 4:
							if event.Instance == "snk_sel" {
								if snkIdx++; snkIdx >= len(snk) {
									snkIdx = 0
								}
								snkSel = snk[snkIdx].Name
								goto render // re-render without getting new data
							} else {
								err = cl.SetSinkVolume(s.Name, min(max(float32(snkVol[snkIdx]+1)/100, 0), 1.25))
							}
						case 5:
							if event.Instance == "snk_sel" {
								if snkIdx--; snkIdx < 0 {
									snkIdx = len(snk) - 1
								}
								snkSel = snk[snkIdx].Name
								goto render // re-render without getting new data
							} else {
								err = cl.SetSinkVolume(s.Name, min(max(float32(snkVol[snkIdx]-1)/100, 0), 1.25))
							}
						}
					}
				case "src_ic", "src_vol", "src_sel":
					if srcIdx != -1 {
						s := src[srcIdx]
						switch event.Button {
						case 1:
							if event.Instance == "src_sel" {
								err = cl.SetDefaultSource(s.Name)
							} else {
								err = cl.SetSourceMute(s.Name, !s.Muted)
							}
						case 2:
							go i3msg(`exec --no-startup-id pavucontrol --tab=4`)
						case 3:
							if srcExp = !srcExp; !srcExp {
								srcSel = ""
							}
							goto render // re-render without getting new data
						case 4:
							if event.Instance == "src_sel" {
								srcDefIsMonitor := src[srcIdx].MonitorSourceName != ""
								for {
									if srcIdx++; snkIdx >= len(src) {
										srcIdx = 0
									}
									if srcDefIsMonitor || src[srcIdx].MonitorSourceName == "" {
										break
									}
								}
								srcSel = src[srcIdx].Name
								goto render // re-render without getting new data
							} else {
								err = cl.SetSourceVolume(s.Name, min(max(float32(srcVol[srcIdx]+1)/100, 0), 1.25))
							}
						case 5:
							if event.Instance == "src_sel" {
								srcDefIsMonitor := src[srcIdx].MonitorSourceName != ""
								for {
									if srcIdx--; srcIdx < 0 {
										srcIdx = len(src) - 1
									}
									if srcDefIsMonitor || src[srcIdx].MonitorSourceName == "" {
										break
									}
								}
								srcSel = src[srcIdx].Name
								goto render // re-render without getting new data
							} else {
								err = cl.SetSourceVolume(s.Name, min(max(float32(srcVol[srcIdx]-1)/100, 0), 1.25))
							}
						}
					}
				}
				if err != nil {
					return err
				}
				continue
			}
			break
		}
	}
}
