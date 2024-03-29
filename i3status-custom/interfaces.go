// # interfaces
//
// Polls netlink at the configured interval for getting interface information.
// Shows filtered interface status, throughput, and IPv4 address. Starts the
// NetworkManager connection editor on middle-click.
package main

import (
	"fmt"
	"net"
	"slices"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/pgaskin/barlib"
	"github.com/pgaskin/barlib/barproto"
	"github.com/vishvananda/netlink"
)

type Interfaces struct {
	Interval time.Duration
	Filter   func(string) bool
	Icon     func(string) rune
}

func (c Interfaces) Run(i barlib.Instance) error {
	i.Tick(c.Interval)
	type State struct {
		View         uint64
		Name         string
		IP           net.IP
		Tx           uint64
		Rx           uint64
		ThroughputAt time.Time
		ThroughputTx float64
		ThroughputRx float64
	}
	state := map[string]State{}
	for isEvent := false; ; {
		var ifaces []string
		links, err := netlink.LinkList()
		if err == nil {
			for _, link := range links {
				attr := link.Attrs()
				if c.Filter != nil && !c.Filter(attr.Name) {
					continue
				}
				if attr.OperState != netlink.OperUp && attr.OperState != netlink.OperUnknown {
					continue
				}
				prev := state[attr.Name]
				update := prev
				update.Name = attr.Name
				update.Tx = attr.Statistics.TxBytes
				update.Rx = attr.Statistics.RxBytes
				update.ThroughputAt = time.Now()
				if update.Tx >= prev.Tx && update.Rx >= prev.Rx {
					update.ThroughputTx = float64(update.Tx-prev.Tx) / update.ThroughputAt.Sub(prev.ThroughputAt).Seconds()
					update.ThroughputRx = float64(update.Rx-prev.Rx) / update.ThroughputAt.Sub(prev.ThroughputAt).Seconds()
				}
				update.IP = nil
				addrs, _ := netlink.AddrList(link, netlink.FAMILY_V4)
				for _, addr := range addrs {
					if addr.Scope == int(netlink.SCOPE_UNIVERSE) {
						update.IP = addr.IP
						break
					}
				}
				state[attr.Name] = update
				ifaces = append(ifaces, attr.Name)
			}
		}
		if !i.IsStopped() {
			slices.Sort(ifaces)
			i.Update(isEvent, func(render barlib.Renderer) {
				if err != nil {
					render.Err(err)
					return
				}
				for _, s := range ifaces {
					s := state[s]
					switch s.View % 3 {
					case 0:
					case 1:
						render(barproto.Block{
							Instance:            s.Name,
							FullText:            fmt.Sprintf(" %s/s↑ %s/s↓", humanize.IBytes(uint64(s.ThroughputTx)), humanize.IBytes(uint64(s.ThroughputRx))),
							Separator:           false,
							SeparatorBlockWidth: 6,
						})
					case 2:
						var ip string
						if s.IP != nil {
							ip = s.IP.String()
						} else {
							ip = "-"
						}
						render(barproto.Block{
							Instance:            s.Name,
							FullText:            ip,
							Separator:           false,
							SeparatorBlockWidth: 6,
						})
					}
					block := barproto.Block{
						Instance:  s.Name,
						FullText:  s.Name,
						Separator: true,
						Color:     0x00FF00FF,
					}
					if c.Icon != nil {
						if x := c.Icon(s.Name); x != 0 {
							block.FullText = string(x)
						}
					}
					render(block)
				}
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
				case 2:
					go i3msg(`exec --no-startup-id nm-connection-editor`)
					continue
				case 3, 4:
					if s, ok := state[event.Instance]; ok {
						isEvent = true
						s.View++
						state[event.Instance] = s
					} else {
						continue
					}
				case 5:
					if s, ok := state[event.Instance]; ok {
						isEvent = true
						s.View--
						state[event.Instance] = s
					} else {
						continue
					}
				}
			}
			break
		}
	}
}
