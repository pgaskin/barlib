// # bluez
//
// Shows the connection status of the specified bluetooth device, disconnecting
// and connecting on click. Uses BlueZ over DBus. Starts blueman on
// middle-click.
package main

import (
	"github.com/godbus/dbus/v5"
	"github.com/pgaskin/barlib"
	"github.com/pgaskin/barlib/barproto"
)

type BluezDevice struct {
	Label   string
	Adapter string
	Name    string
}

func (c BluezDevice) Run(i barlib.Instance) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	obj := conn.Object("org.bluez", dbus.ObjectPath("/org/bluez/"+c.Adapter+"/"+c.Name))
	if err := conn.AddMatchSignal(
		dbus.WithMatchSender("org.freedesktop.DBus"),
		dbus.WithMatchObjectPath("/org/freedesktop/DBus"),
		dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
		dbus.WithMatchArg(0, "org.bluez"),
	); err != nil {
		return err
	}
	if err := conn.AddMatchSignal(
		dbus.WithMatchObjectPath(obj.Path()),
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchMember("PropertiesChanged"),
		dbus.WithMatchArg(0, "org.bluez.Device1"),
	); err != nil {
		return err
	}
	ch := make(chan *dbus.Signal, 6)
	conn.Signal(ch)
	for {
		var (
			address   string
			connected bool
		)
		if err := obj.StoreProperty("org.bluez.Device1.Address", &address); err == nil {
			if err := obj.StoreProperty("org.bluez.Device1.Connected", &connected); err != nil {
				return err
			}
		}
		i.Update(false, func(render barlib.Renderer) {
			block := barproto.Block{
				FullText:  c.Label,
				Separator: true,
			}
			if address == "" {
				block.Color = 0xFF0000FF
			} else if connected {
				block.Color = 0x00FF00FF
			}
			render(block)
		})
		for {
			select {
			case sig := <-ch:
				switch {
				case sig.Sender == "org.freedesktop.DBus" && sig.Path == "/org/freedesktop/DBus" && sig.Name == "org.freedesktop.DBus.NameOwnerChanged" && len(sig.Body) == 3:
					if sig.Body[0].(string) != "org.bluez" {
						continue
					}
				case sig.Path == obj.Path() && sig.Name == "org.freedesktop.DBus.Properties.PropertiesChanged" && len(sig.Body) == 3:
					if sig.Body[0].(string) != "org.bluez.Device1" {
						continue
					}
				}
			case event := <-i.Event():
				switch event.Button {
				case 1:
					if address != "" {
						if connected {
							if err := obj.Call("org.bluez.Device1.Disconnect", dbus.FlagNoReplyExpected).Err; err != nil {
								_ = err // TODO: do something?
							}
						} else {
							if err := obj.Call("org.bluez.Device1.Connect", dbus.FlagNoReplyExpected).Err; err != nil {
								_ = err // TODO: do something?
							}
						}
					}
				case 2:
					if niri {
						nirimsg("action", "spawn", "--", "blueman-manager")
					} else {
						i3msg(`exec --no-startup-id blueman-manager`)
					}
				}
				continue
			}
			break
		}
	}
}
