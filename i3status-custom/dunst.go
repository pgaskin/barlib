// # dunst
//
// Uses dunst over DBus. Toggles do-not-disturb on click. Shows and hides
// notification history on scroll. Allows notification actions for the last
// notification to be selected on middle-click.
package main

import (
	"github.com/godbus/dbus/v5"
	"github.com/pgaskin/barlib"
	"github.com/pgaskin/barlib/barproto"
)

type Dunst struct {
}

func (c Dunst) Run(i barlib.Instance) error {
	// scroll up to show, down to hide, click for dnd
	conn, err := dbus.SessionBus()
	if err != nil {
		return err
	}
	obj := conn.Object("org.freedesktop.Notifications", dbus.ObjectPath("/org/freedesktop/Notifications"))
	if err := conn.AddMatchSignal(
		dbus.WithMatchSender("org.freedesktop.DBus"),
		dbus.WithMatchObjectPath("/org/freedesktop/DBus"),
		dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
		dbus.WithMatchArg(0, "org.freedesktop.Notifications"),
	); err != nil {
		return err
	}
	if err := conn.AddMatchSignal(
		dbus.WithMatchObjectPath(obj.Path()),
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchMember("PropertiesChanged"),
		dbus.WithMatchArg(0, "org.dunstproject.cmd0"),
	); err != nil {
		return err
	}
	// note: https://github.com/dunst-project/dunst/issues/765
	ch := make(chan *dbus.Signal, 6)
	conn.Signal(ch)
	for {
		var (
			paused bool
		)
		if err := obj.StoreProperty("org.dunstproject.cmd0.paused", &paused); err != nil {
			return err
		}
		i.Update(false, func(render barlib.Renderer) {
			if paused {
				render(barproto.Block{
					FullText:  "\uf1f6",
					Separator: true,
					Color:     0xFFFF00FF,
				})
			} else {
				render(barproto.Block{
					FullText:       "\uf0f3",
					MinWidthString: "\uf1f6",
					Align:          "center",
					Separator:      true,
					Color:          0x87CEEBFF,
				})
			}
		})
		for {
			select {
			case sig := <-ch:
				switch {
				case sig.Sender == "org.freedesktop.DBus" && sig.Path == "/org/freedesktop/DBus" && sig.Name == "org.freedesktop.DBus.NameOwnerChanged" && len(sig.Body) == 3:
					if sig.Body[0].(string) != "org.freedesktop.Notifications" {
						continue
					}
				case sig.Path == obj.Path() && sig.Name == "org.freedesktop.DBus.Properties.PropertiesChanged" && len(sig.Body) == 3:
					if sig.Body[0].(string) != "org.dunstproject.cmd0" {
						continue
					}
				}
			case event := <-i.Event():
				switch event.Button {
				case 1:
					if err := obj.SetProperty("org.dunstproject.cmd0.paused", dbus.MakeVariant(!paused)); err != nil {
						return err
					}
				case 2:
					i3msg(`exec --no-startup-id dunstctl action`)
				case 4:
					if err := obj.Call("org.dunstproject.cmd0.NotificationCloseLast", 0).Err; err != nil {
						return err
					}
				case 5:
					if err := obj.Call("org.dunstproject.cmd0.NotificationShow", 0).Err; err != nil {
						return err
					}
				}
				continue
			}
			break
		}
	}
}
