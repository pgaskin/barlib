// # cmus
//
// Shows the status of cmus using MPRIS over DBus. Starts it in an
// xfce4-terminal window when middle-clicked, showing and hiding it from the i3
// scratchpad on scroll. Supports metadata, seeking, volume, and more.
package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/pgaskin/barlib"
	"github.com/pgaskin/barlib/barproto"
)

type CMUS struct{}

func (c CMUS) Run(i barlib.Instance) error {
	conn, err := dbus.SessionBus()
	if err != nil {
		return err
	}
	if err := conn.AddMatchSignal(
		dbus.WithMatchSender("org.mpris.MediaPlayer2.cmus"),
		dbus.WithMatchObjectPath("/org/mpris/MediaPlayer2"),
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchMember("PropertiesChanged"),
		dbus.WithMatchArg(0, "org.mpris.MediaPlayer2.Player"),
	); err != nil {
		return err
	}
	if err := conn.AddMatchSignal(
		dbus.WithMatchSender("org.mpris.MediaPlayer2.cmus"),
		dbus.WithMatchObjectPath("/org/mpris/MediaPlayer2"),
		dbus.WithMatchInterface("org.mpris.MediaPlayer2.Player"),
		dbus.WithMatchMember("Seeked"),
	); err != nil {
		return err
	}
	if err := conn.AddMatchSignal(
		dbus.WithMatchSender("org.freedesktop.DBus"),
		dbus.WithMatchObjectPath("/org/freedesktop/DBus"),
		dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
		dbus.WithMatchArg(0, "org.mpris.MediaPlayer2.cmus"),
	); err != nil {
		return err
	}
	ch := make(chan *dbus.Signal, 6)
	{
		var names []string
		if err := conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names); err == nil {
			for _, name := range names {
				if name == "org.mpris.MediaPlayer2.cmus" {
					ch <- &dbus.Signal{
						Sender: "org.freedesktop.DBus",
						Path:   "/org/freedesktop/DBus",
						Name:   "org.freedesktop.DBus.NameOwnerChanged",
						Body: []interface{}{
							string("org.mpris.MediaPlayer2.cmus"),
							string(""),
							string("dummy"),
						},
					}
				}
			}
		}
	}
	conn.Signal(ch)
	type State struct {
		object   dbus.BusObject
		metadata map[string]dbus.Variant
		status   string
		volume   float64
		position int64
	}
	var (
		view  uint64
		state State
	)
	ticker := i.Tick(0)
	for {
		if state.object != nil && state.status == "Playing" && view >= 1 {
			i.TickReset(ticker, time.Second/4)
		} else {
			i.TickReset(ticker, 0)
		}
		i.Update(true, func(render barlib.Renderer) {
			if state.object == nil {
				render(barproto.Block{
					FullText:  "?",
					Color:     0xFF0000FF,
					Separator: true,
				})
			} else {
				view %= 4
				var playColor uint32
				if state.status == "Playing" {
					playColor = 0x00FF00FF
				} else {
					playColor = 0xFFFF00FF
				}
				if view >= 1 {
					render(barproto.Block{
						FullText:            "", // music
						Color:               playColor,
						SeparatorBlockWidth: 8,
					})
				}
				if view >= 2 {
					var (
						title  string
						artist string
					)
					if v := state.metadata["xesam:title"].Value(); v != nil {
						title, _ = v.(string)
					}
					if v := state.metadata["xesam:artist"].Value(); v != nil {
						if v, _ := v.([]string); len(v) > 0 {
							artist = v[0]
						}
					}
					if artist == "" {
						if v := state.metadata["xesam:albumArtist"].Value(); v != nil {
							if v, _ := v.([]string); len(v) > 0 {
								artist = v[0]
							}
						}
					}
					if view >= 3 {
						if v := state.metadata["xesam:album"].Value(); v != nil {
							if v, _ := v.(string); len(v) > 0 {
								artist += " - " + v
							}
						}
					}
					if title == "" {
						title = "?"
					}
					if artist != "" {
						render(barproto.Block{
							Instance: "play_pause",
							FullText: artist + " - " + title + " - ",
							Color:    playColor,
						})
					} else {
						render(barproto.Block{
							Instance: "play_pause",
							FullText: title + " - ",
							Color:    playColor,
						})
					}
				}
				if view >= 1 {
					render(barproto.Block{
						Instance:            "volume",
						FullText:            strconv.Itoa(int(state.volume*100)) + "%",
						Color:               playColor,
						SeparatorBlockWidth: 8,
					})
					var length int64
					if v := state.metadata["mpris:length"].Value(); v != nil {
						length, _ = v.(int64)
					}
					if length != 0 {
						render(barproto.Block{
							Instance:            "seek",
							FullText:            fmt.Sprintf("%d:%02d / %d:%02d", state.position/int64(time.Second/time.Microsecond)/60, state.position/int64(time.Second/time.Microsecond)%60, length/int64(time.Second/time.Microsecond)/60, length/int64(time.Second/time.Microsecond)%60),
							Color:               playColor,
							SeparatorBlockWidth: 8,
						})
					} else {
						render(barproto.Block{
							Instance:            "seek",
							FullText:            fmt.Sprintf("%d:%02d", state.position/int64(time.Second/time.Microsecond)/60, state.position/int64(time.Second/time.Microsecond)%60),
							Color:               playColor,
							SeparatorBlockWidth: 8,
						})
					}
					render(barproto.Block{
						Instance:            "previous",
						FullText:            "", // previous
						Color:               0x00FF00FF,
						SeparatorBlockWidth: 4,
					})
					render(barproto.Block{
						Instance:            "next",
						FullText:            "", // next
						Color:               0x00FF00FF,
						SeparatorBlockWidth: 4,
					})
				}
				switch state.status {
				case "Playing":
					render(barproto.Block{
						Instance:  "play_pause",
						FullText:  "", // pause
						Color:     0x00FF00FF,
						Separator: true,
					})
				case "Paused":
					render(barproto.Block{
						Instance:  "play_pause",
						FullText:  "", // play
						Color:     0x00FF00FF,
						Separator: true,
					})
				default:
					render(barproto.Block{
						Instance:  "play_pause",
						FullText:  "", // play
						Color:     0xFF0000FF,
						Separator: true,
					})
				}
			}
		})
		for {
			select {
			case <-ticker:
				if state.object != nil {
					if err := state.object.StoreProperty("org.mpris.MediaPlayer2.Player.Position", &state.position); err != nil {
						return fmt.Errorf("position: %w", err)
					}
				}
			case sig := <-ch:
				switch {
				case sig.Sender == "org.freedesktop.DBus" && sig.Path == "/org/freedesktop/DBus" && sig.Name == "org.freedesktop.DBus.NameOwnerChanged" && len(sig.Body) == 3:
					var (
						name, _     = sig.Body[0].(string)
						oldOwner, _ = sig.Body[1].(string)
						newOwner, _ = sig.Body[2].(string)
					)
					if name == "org.mpris.MediaPlayer2.cmus" {
						if newOwner == "" {
							state = State{}
						} else {
							state = State{
								object: conn.Object("org.mpris.MediaPlayer2.cmus", "/org/mpris/MediaPlayer2"),
							}
							if err := state.object.StoreProperty("org.mpris.MediaPlayer2.Player.Metadata", &state.metadata); err != nil {
								return fmt.Errorf("metadata: %w", err)
							}
							if err := state.object.StoreProperty("org.mpris.MediaPlayer2.Player.PlaybackStatus", &state.status); err != nil {
								return fmt.Errorf("playback status: %w", err)
							}
							if err := state.object.StoreProperty("org.mpris.MediaPlayer2.Player.Volume", &state.volume); err != nil {
								return fmt.Errorf("volume: %w", err)
							}
							if err := state.object.StoreProperty("org.mpris.MediaPlayer2.Player.Position", &state.position); err != nil {
								return fmt.Errorf("position: %w", err)
							}
						}
					} else {
						continue
					}
					_ = oldOwner
				case sig.Path == "/org/mpris/MediaPlayer2" && sig.Name == "org.freedesktop.DBus.Properties.PropertiesChanged" && len(sig.Body) == 3:
					var (
						object, _     = sig.Body[0].(string)
						properties, _ = sig.Body[1].(map[string]dbus.Variant)
					)
					if object == "org.mpris.MediaPlayer2.Player" {
						for k, v := range properties {
							switch k {
							case "Metadata":
								state.metadata = nil
								if err := v.Store(&state.metadata); err != nil {
									return fmt.Errorf("metadata: %w", err)
								}
							case "PlaybackStatus":
								if err := v.Store(&state.status); err != nil {
									return fmt.Errorf("playback status: %w", err)
								}
							case "Volume":
								if err := v.Store(&state.volume); err != nil {
									return fmt.Errorf("volume: %w", err)
								}
							}
						}
					} else {
						continue
					}
				case sig.Path == "/org/mpris/MediaPlayer2" && sig.Name == "org.mpris.MediaPlayer2.Player.Seeked":
					if err := state.object.StoreProperty("org.mpris.MediaPlayer2.Player.Position", &state.position); err != nil {
						return fmt.Errorf("position: %w", err)
					}
				default:
					continue
				}
			case event := <-i.Event():
				switch event.Button {
				case 1:
					if state.object != nil {
						switch event.Instance {
						case "play_pause", "seek":
							if err := state.object.Call("org.mpris.MediaPlayer2.Player.PlayPause", dbus.FlagNoReplyExpected).Err; err != nil {
								return err
							}
						case "previous":
							if err := state.object.Call("org.mpris.MediaPlayer2.Player.Previous", dbus.FlagNoReplyExpected).Err; err != nil {
								return err
							}
						case "next":
							if err := state.object.Call("org.mpris.MediaPlayer2.Player.Next", dbus.FlagNoReplyExpected).Err; err != nil {
								return err
							}
						}
					}
					continue
				case 2:
					if state.object == nil {
						i3msg(`exec --no-startup-id xfce4-terminal --hide-scrollbar --hide-menubar --dynamic-title-mode none --title cmus --role cmus -e cmus`)
					} else {
						i3msg(`exec --no-startup-id killall -SIGHUP cmus`)
					}
					continue
				case 3:
					if state.object != nil {
						if view == 0 {
							if err := state.object.StoreProperty("org.mpris.MediaPlayer2.Player.Position", &state.position); err != nil {
								return fmt.Errorf("position: %w", err)
							}
						}
					}
					view++
					// no continue since we want to re-render immediately
				case 4, 5:
					if state.object != nil {
						switch event.Instance {
						case "volume":
							var vol float64
							if err := state.object.StoreProperty("org.mpris.MediaPlayer2.Player.Volume", &vol); err != nil {
								return err
							}
							if event.Button == 4 {
								vol = float64(min(max(int(vol*100)+5, 0), 100)) / 100
							} else {
								vol = float64(min(max(int(vol*100)-5, 0), 100)) / 100
							}
							if err := state.object.SetProperty("org.mpris.MediaPlayer2.Player.Volume", dbus.MakeVariant(vol)); err != nil {
								return err
							}
						case "seek":
							var delta int64
							if event.Button == 4 {
								delta = 5 * 1000 * 1000
							} else {
								delta = -5 * 1000 * 1000
							}
							if err := state.object.Call("org.mpris.MediaPlayer2.Player.Seek", dbus.FlagNoReplyExpected, delta).Err; err != nil {
								return err
							}
						default:
							if event.Button == 4 {
								i3msg(`[window_role="^cmus$"] move scratchpad; [window_role="^cmus$"] scratchpad show`)
							} else {
								i3msg(`[window_role="^cmus$"] move scratchpad`)
							}
						}
					}
					continue
				default:
					continue
				}
			}
			break
		}
	}
}
