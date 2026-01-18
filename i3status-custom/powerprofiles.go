// # [ower profiles
//
// Toggles between platform power profiles.
package main

import (
	"slices"

	"github.com/godbus/dbus/v5"
	"github.com/pgaskin/barlib"
	"github.com/pgaskin/barlib/barproto"
)

type PowerProfiles struct {
}

func (c PowerProfiles) Run(i barlib.Instance) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	obj := conn.Object("net.hadess.PowerProfiles", dbus.ObjectPath("/net/hadess/PowerProfiles"))
	if err := conn.AddMatchSignal(
		dbus.WithMatchSender("org.freedesktop.DBus"),
		dbus.WithMatchObjectPath("/org/freedesktop/DBus"),
		dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
		dbus.WithMatchArg(0, "net.hadess.PowerProfiles"),
	); err != nil {
		return err
	}
	if err := conn.AddMatchSignal(
		dbus.WithMatchObjectPath(obj.Path()),
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchMember("PropertiesChanged"),
		dbus.WithMatchArg(0, "net.hadess.PowerProfiles"),
	); err != nil {
		return err
	}
	ch := make(chan *dbus.Signal, 6)
	conn.Signal(ch)
	for {
		var (
			activeProfile string
			profiles      []map[string]dbus.Variant
		)
		if err := obj.StoreProperty("net.hadess.PowerProfiles.ActiveProfile", &activeProfile); err == nil {
			if err := obj.StoreProperty("net.hadess.PowerProfiles.Profiles", &profiles); err != nil {
				return err
			}
		}
		i.Update(false, func(render barlib.Renderer) {
			if len(profiles) != 0 {
				block := barproto.Block{
					FullText:  activeProfile,
					Separator: true,
				}
				switch activeProfile {
				case "performance":
					block.FullText = "\uf625"
					block.Color = 0xFF5C00FF
				case "power-saver":
					block.FullText = "\uf300"
					block.Color = 0xFFFF00FF
				case "balanced":
					block.FullText = "\uf24e"
				}
				render(block)
			}
		})
		for {
			select {
			case sig := <-ch:
				switch {
				case sig.Sender == "org.freedesktop.DBus" && sig.Path == "/org/freedesktop/DBus" && sig.Name == "org.freedesktop.DBus.NameOwnerChanged" && len(sig.Body) == 3:
					if sig.Body[0].(string) != "net.hadess.PowerProfiles" {
						continue
					}
				case sig.Path == obj.Path() && sig.Name == "org.freedesktop.DBus.Properties.PropertiesChanged" && len(sig.Body) == 3:
					if sig.Body[0].(string) != "net.hadess.PowerProfiles" {
						continue
					}
				}
			case event := <-i.Event():
				switch event.Button {
				case 1:
					if len(profiles) != 0 {
						i := (slices.IndexFunc(profiles, func(p map[string]dbus.Variant) bool {
							return p["Profile"].Value() == activeProfile
						}) + 1) % len(profiles)
						if err := obj.SetProperty("net.hadess.PowerProfiles.ActiveProfile", profiles[i]["Profile"]); err != nil {
							return err
						}
					}
				}
				continue
			}
			break
		}
	}
}
