// # mako
//
// Uses mako over DBus. Shows and hides notification history on scroll. Allows
// notification actions for the last notification to be selected on
// middle-click.
package main

import (
	"github.com/godbus/dbus/v5"
	"github.com/pgaskin/barlib"
	"github.com/pgaskin/barlib/barproto"
)

type Mako struct {
}

func (c Mako) Run(i barlib.Instance) error {
	// scroll up to show, down to hide, click for dnd
	conn, err := dbus.SessionBus()
	if err != nil {
		return err
	}
	obj := conn.Object("org.freedesktop.Notifications", dbus.ObjectPath("/fr/emersion/Mako"))
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
		dbus.WithMatchArg(0, "fr.emersion.Mako"),
	); err != nil {
		return err
	}
	ch := make(chan *dbus.Signal, 6)
	conn.Signal(ch)
	for {
		i.Update(false, func(render barlib.Renderer) {
			render(barproto.Block{
				FullText:  "\uf0f3",
				Separator: true,
			})
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
					if sig.Body[0].(string) != "fr.emersion.Mako" {
						continue
					}
				}
			case event := <-i.Event():
				switch event.Button {
				case 2:
					nirimsg("action", "spawn", "--", "makoctl", "menu", "--", "fuzzel", "--dmenu")
				case 4:
					if err := obj.Call("fr.emersion.Mako.DismissNotifications", 0, map[string]dbus.Variant{}).Err; err != nil {
						return err
					}
				case 5:
					if err := obj.Call("fr.emersion.Mako.RestoreNotification", 0).Err; err != nil {
						return err
					}
				}
				continue
			}
			break
		}
	}
}
