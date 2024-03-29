// # wifi
//
// Polls netlink at the configured interval for wifi information. Shows network
// info and throughput.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/mdlayher/wifi"
	"github.com/pgaskin/barlib"
	"github.com/pgaskin/barlib/barproto"
)

type WiFi struct {
	Interval       time.Duration
	Threshold      int
	ThresholdColor uint32
}

func (c WiFi) Run(i barlib.Instance) error {
	cl, err := wifi.New()
	if err != nil {
		return err
	}
	type State struct {
		SSID         string
		BSSID        net.HardwareAddr
		Frequency    int
		Signal       int
		Tx           int
		Rx           int
		ThroughputAt time.Time
		ThroughputTx float64
		ThroughputRx float64
	}
	var (
		throughput bool
		state      = map[int]State{}
	)
	for ticker, isEvent := i.Tick(c.Interval), false; ; {
		ifaces, err := cl.Interfaces()
		if !i.IsStopped() || throughput {
			for _, iface := range ifaces {
				if iface.Type == wifi.InterfaceTypeStation {
					var bss *wifi.BSS
					if bss, err = cl.BSS(iface); err == nil {
						prev := state[iface.Index]
						update := State{
							SSID:      bss.SSID,
							BSSID:     bss.BSSID,
							Frequency: iface.Frequency,
						}
						var stas []*wifi.StationInfo
						if stas, err = cl.StationInfo(iface); err == nil {
							for _, sta := range stas {
								if bytes.Equal(sta.HardwareAddr, bss.BSSID) {
									update.Signal = sta.Signal
									update.Tx = sta.TransmittedBytes
									update.Rx = sta.ReceivedBytes
									update.ThroughputAt = time.Now()
									if bytes.Equal(prev.BSSID, update.BSSID) && update.Tx >= prev.Tx && update.Rx >= prev.Rx {
										update.ThroughputTx = float64(update.Tx-prev.Tx) / update.ThroughputAt.Sub(prev.ThroughputAt).Seconds()
										update.ThroughputRx = float64(update.Rx-prev.Rx) / update.ThroughputAt.Sub(prev.ThroughputAt).Seconds()
									}
									break
								}
							}
						}
						state[iface.Index] = update
					} else {
						delete(state, iface.Index)
					}
				} else {
					delete(state, iface.Index)
				}
				if err != nil {
					break
				}
			}
		}
		if !i.IsStopped() {
			i.Update(isEvent, func(render barlib.Renderer) {
				if errors.Is(err, fs.ErrNotExist) {
					return
				}
				if err != nil {
					render.Err(err)
				}
				for _, iface := range ifaces {
					if cur, ok := state[iface.Index]; ok {
						if throughput {
							render(barproto.Block{
								FullText:  fmt.Sprintf("%s/s↑ %s/s↓", humanize.IBytes(uint64(cur.ThroughputTx)), humanize.IBytes(uint64(cur.ThroughputRx))),
								Separator: true,
							})
						} else {
							if cur.SSID == "" {
								cur.SSID = "?"
							}
							render(barproto.Block{
								FullText:  fmt.Sprintf("%s %.1fG %ddBm", cur.SSID, float64(cur.Frequency)/1000, cur.Signal),
								Color:     0x00FF00FF,
								Separator: true,
							})
						}
					}
				}
			})
		}
		for isEvent = false; ; {
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
					go i3msg(`exec --no-startup-id nm-connection-editor`)
					continue
				case 3:
					throughput = !throughput
					isEvent = true
				}
			}
			break
		}
	}
}
