// Package i3status-custom is a subset of my personal i3status configuration.
//
// In general, all modules use left-clicks to trigger actions, right-clicks to
// change views, middle-clicks to open an external application with more
// details, and scrolling to change values.
//
// Most modules use DBus or another event-driven protocol rather than polling
// for efficiency and responsiveness, and avoid calling external commands or
// using CGO.
//
// Icons from Font Awesome 5 Pro are used in the output.
//
// The modules are documented below and can be used as a starting point for
// making your own ones.
package main

import (
	"strings"
	"time"

	"github.com/pgaskin/barlib"
)

/*
	bar {
		id test
		position top
		mode hide
		status_command exec /tmp/i3status-custom
		font pango:DejaVu Sans Mono 8, Font Awesome 5 Pro Regular 8
	}
	go build -o /tmp/i3status-custom .
	pkill -f bar_id=test
	i3bar --bar_id=test --verbose
*/

func main() {
	barlib.Main(time.Second/4,
		// TODO: generic mpris?
		CMUS{},
		PulseAudio{
			ShowSink:   true,
			ShowSource: true,
		},
		Backlight{
			Interval:  time.Second * 5,
			Subsystem: "backlight",
			Name:      "amdgpu_bl1",
		},
		Redshift{
			Latitude:         44.5,
			Longitude:        -76.5,
			ElevationDay:     3,  // 3 degrees above the horizon
			ElevationNight:   -6, // civil twilight
			TemperatureDay:   6500,
			TemperatureNight: 3000,
		},
		XRandR{},
		DDC{
			Interval:   time.Minute,
			ID:         "ACR2406-F2179101",
			HideIfGone: true,
			Brightness: true,
			Contrast:   true,
			Presets: [][2]uint16{
				{0, 0},
				{10, 10},
				{45, 35},
				{60, 60},
			},
		},
		Battery{
			Name: "BAT0",
		},
		BluezDevice{
			Label:   "\uf025",
			Adapter: "hci0",
			Name:    "dev_00_1B_66_10_CA_67",
		},
		BluezDevice{
			Label:   "\uf025",
			Adapter: "hci0",
			Name:    "dev_74_F8_DB_95_10_72",
		},
		Interfaces{
			Interval: time.Second * 5,
			Filter: func(iface string) bool {
				switch {
				case strings.HasPrefix(iface, "wg"):
				case strings.HasPrefix(iface, "en"):
				case strings.HasPrefix(iface, "wl"):
				default:
					return false
				}
				return true
			},
			Icon: func(iface string) rune {
				switch {
				case strings.HasPrefix(iface, "wg"):
					return '\uf30d'
				case strings.HasPrefix(iface, "en"):
					return '\uf6ff'
				case strings.HasPrefix(iface, "wl"):
					return '\uf1eb'
				default:
					return 0
				}
			},
		},
		WiFi{
			Interval:       time.Second * 5,
			Threshold:      -70,
			ThresholdColor: 0xFFFF00FF,
		},
		Disk{
			Interval:       time.Second * 5,
			Threshold:      10 * 1024 * 1024 * 1024,
			ThresholdColor: 0xFFFF00FF,
			Mountpoint:     "/home",
		},
		CPU{
			Interval: time.Second * 3,
		},
		Temperature{
			Interval: time.Second * 3,
			Chip:     "k10temp",
			Index:    1,
		},
		Memory{
			Interval:       time.Second * 3,
			Threshold:      1 * 1024 * 1024 * 1024,
			ThresholdColor: 0xFFFF00FF,
		},
		Time{
			LayoutFull:  "Mon 01/02 15:04:05",
			LayoutShort: "15:04",
			Interval:    time.Second,
			Color:       0x87CEEBFF,
		},
		Dunst{},
	)
}
