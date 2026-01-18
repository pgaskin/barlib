// # battery
//
// Shows battery information for a specific battery using UPower over DBus, plus
// sysfs for showing charge thresholds if supported.
package main

import (
	"fmt"
	"path/filepath"

	"github.com/godbus/dbus/v5"
	"github.com/pgaskin/barlib"
	"github.com/pgaskin/barlib/barproto"
)

type Battery struct {
	Name string
}

func (c Battery) Run(i barlib.Instance) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	obj := conn.Object("org.freedesktop.UPower", dbus.ObjectPath("/org/freedesktop/UPower/devices/battery_"+c.Name))
	if err := conn.AddMatchSignal(
		dbus.WithMatchObjectPath(obj.Path()),
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchMember("PropertiesChanged"),
		dbus.WithMatchArg(0, "org.freedesktop.UPower.Device"),
	); err != nil {
		return err
	}
	ch := make(chan *dbus.Signal, 6)
	conn.Signal(ch)
	for {
		var prop struct {
			IsPresent                   bool
			Energy                      float64
			EnergyEmpty                 float64
			EnergyFull                  float64
			EnergyRate                  float64
			Voltage                     float64
			ChargeCycles                int32
			TimeToEmpty                 int64
			TimeToFull                  int64
			State                       uint32
			ChargeControlStartThreshold int // default: 0
			ChargeControlStopThreshold  int // default: 100
		}
		err := func() error {
			var typ uint32
			if err := obj.StoreProperty("org.freedesktop.UPower.Device.Type", &typ); err != nil {
				return fmt.Errorf("type: %w", err)
			} else if typ != 2 {
				return fmt.Errorf("type: not battery")
			}
			if err := obj.StoreProperty("org.freedesktop.UPower.Device.IsPresent", &prop.IsPresent); err != nil {
				return fmt.Errorf("is present: %w", err)
			}
			if err := obj.StoreProperty("org.freedesktop.UPower.Device.Energy", &prop.Energy); err != nil {
				return fmt.Errorf("energy: %w", err)
			}
			if err := obj.StoreProperty("org.freedesktop.UPower.Device.EnergyEmpty", &prop.EnergyEmpty); err != nil {
				return fmt.Errorf("energy empty: %w", err)
			}
			if err := obj.StoreProperty("org.freedesktop.UPower.Device.EnergyFull", &prop.EnergyFull); err != nil {
				return fmt.Errorf("energy full: %w", err)
			}
			if err := obj.StoreProperty("org.freedesktop.UPower.Device.EnergyRate", &prop.EnergyRate); err != nil {
				return fmt.Errorf("energy rate: %w", err)
			}
			if err := obj.StoreProperty("org.freedesktop.UPower.Device.Voltage", &prop.Voltage); err != nil {
				return fmt.Errorf("voltage: %w", err)
			}
			if err := obj.StoreProperty("org.freedesktop.UPower.Device.ChargeCycles", &prop.ChargeCycles); err != nil {
				return fmt.Errorf("charge cycles: %w", err)
			}
			if err := obj.StoreProperty("org.freedesktop.UPower.Device.TimeToEmpty", &prop.TimeToEmpty); err != nil {
				return fmt.Errorf("time to empty: %w", err)
			}
			if err := obj.StoreProperty("org.freedesktop.UPower.Device.TimeToFull", &prop.TimeToFull); err != nil {
				return fmt.Errorf("time to full: %w", err)
			}
			if err := obj.StoreProperty("org.freedesktop.UPower.Device.State", &prop.State); err != nil {
				return fmt.Errorf("state: %w", err)
			}
			prop.ChargeControlStartThreshold, _ = readFileInt[int](filepath.Join("/sys/class/power_supply", c.Name, "charge_control_start_threshold"))
			prop.ChargeControlStopThreshold, _ = readFileInt[int](filepath.Join("/sys/class/power_supply", c.Name, "charge_control_stop_threshold"))
			if prop.ChargeControlStopThreshold == 0 {
				prop.ChargeControlStopThreshold = 100
			}
			return nil
		}()
		i.Update(false, func(render barlib.Renderer) {
			if err != nil {
				render.Err(err)
				return
			}
			var percent float64
			if prop.EnergyFull != 0 {
				percent = (prop.Energy - prop.EnergyEmpty) / (prop.EnergyFull - prop.EnergyEmpty) * 100
			}
			block := barproto.Block{
				Separator: true,
			}
			if prop.IsPresent {
				switch prop.State {
				case 0: // unknown
					block.FullText = "?"
					block.Color = 0xFF0000FF
				case 1: // charging
					var threshold string
					if prop.ChargeControlStopThreshold != 0 && prop.ChargeControlStopThreshold != 100 {
						threshold = fmt.Sprintf(" \uf178 %d%%", prop.ChargeControlStopThreshold)
					} else {
						threshold = "%"
					}
					block.FullText = fmt.Sprintf("%.1f%s %.1fV %.1fW %d:%02d:%02d", percent, threshold, prop.Voltage, prop.EnergyRate, prop.TimeToFull/60/60, prop.TimeToFull/60%60, prop.TimeToFull%60)
					block.Color = 0x00FF00FF
				case 2: // discharging
					block.FullText = fmt.Sprintf("%.1f%% %.1fV %.1fW %d:%02d:%02d", percent, prop.Voltage, prop.EnergyRate, prop.TimeToEmpty/60/60, prop.TimeToEmpty/60%60, prop.TimeToFull%60)
					block.Color = 0xFFFF00FF
				case 3: // empty
					block.FullText = fmt.Sprintf("%.1f%% %.1fV EMPTY", percent, prop.Voltage)
					block.Color = 0xFF0000FF
				case 4: // fully charged
					block.FullText = fmt.Sprintf("%.0f%%", percent)
					block.Color = 0x00FF00FF
				case 5: // pending charge
					block.FullText = fmt.Sprintf("%.0f", percent)
					if prop.ChargeControlStartThreshold != 0 {
						block.FullText += fmt.Sprintf(" \uf178 %d%%", prop.ChargeControlStartThreshold)
					} else {
						block.FullText += "%"
					}
				case 6: // pending discharge
					fallthrough
				default:
					block.FullText = fmt.Sprintf("%.0f%%", percent)
				}
			} else {
				block.FullText = "-"
				block.Color = 0xFF0000FF
			}
			render(block)
		})
		<-ch
	}
}
