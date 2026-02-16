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
	"os"
	"slices"
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

/*
	command = "i3status-custom"
	background = "#00000000"
	color = "#ccccccee"
	font = "Font Awesome 5 Pro Regular 8, Noto Sans Mono 8"
	height = 18
	layer = "background"
	separator = "#777777cc"
	separator_width = 1.0

	go build -o /tmp/i3status-custom .
	i3bar-river --config config
*/

func main() {
	var mid string
	if buf, err := os.ReadFile("/etc/machine-id"); err == nil {
		mid = string(buf)
	}

	const (
		p1 = "5a" // main laptop (p16s g2 amd 21k9 7840u) (x11 i3wm)
		s1 = "44" // old laptop (x395 amd 20nl 3500u) (wayland niri)
		s2 = "2c" // secondary laptop (t14s g6 intel 21qx 258v) (wayland niri)
		d1 = "22" // desktop (amd) (x11 i3wm, but mostly used headless)
	)

	var (
		mods []barlib.Module

		is = func(mids ...string) bool {
			return slices.ContainsFunc(mids, func(x string) bool {
				return strings.HasPrefix(mid, x)
			})
		}
		add = func(mod barlib.Module, mids ...string) {
			if len(mids) == 0 || is(mids...) {
				mods = append(mods, mod)
			}
		}
	)

	add(CMUS{}, p1, s1, s2) // TODO: generic mpris?

	add(PulseAudio{
		ShowSink:   true,
		ShowSource: true,
	}, p1, s1, s2)

	add(Backlight{
		Interval:    time.Second * 5,
		Subsystem:   "backlight",
		Name:        "amdgpu_bl1",
		SessionName: "self",
	}, p1, s1)

	add(Backlight{
		Interval:    time.Second * 5,
		Subsystem:   "backlight",
		Name:        "intel_backlight",
		SessionName: "auto",
	}, s2)

	add(Redshift{
		Latitude:         44.5,
		Longitude:        -76.5,
		ElevationDay:     3,  // 3 degrees above the horizon
		ElevationNight:   -6, // civil twilight
		TemperatureDay:   6500,
		TemperatureNight: 3000,
	})

	add(XRandR{}, p1)

	add(DDC{
		Interval:   time.Minute,
		ID:         "ACRE70C-A55C5042",
		HideIfGone: true,
		Blind:      true, // read seems to be broken (though windows works somehow)
		Brightness: true,
		Contrast:   true,
		Presets: [][2]uint16{
			{0, 0},
			{10, 10},
			{15, 20},
			{25, 35},
			{35, 35},
			{45, 35},
			{50, 50},
			{60, 50},
			{80, 50},
			{100, 50},
			{100, 75},
			{100, 100},
		},
	}, p1)

	add(DDC{
		Interval:   time.Minute,
		ID:         "ACRE70C-00000000", //"ACRE70C-9C5E5042", (the Cable Matters 201376-BLK is nice but it doesn't pass the edid serial properly)
		HideIfGone: true,
		Blind:      true, // read seems to be broken (though windows works somehow)
		Brightness: true,
		Contrast:   true,
		Presets: [][2]uint16{
			{0, 0},
			{10, 10},
			{15, 20},
			{25, 35},
			{35, 35},
			{45, 35},
			{50, 50},
			{60, 50},
			{80, 50},
			{100, 50},
			{100, 75},
			{100, 100},
		},
	}, p1)

	add(DDC{
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
			{100, 80},
		},
	}, p1)

	add(Battery{
		Name: "BAT0",
	}, p1, s1, s2)

	add(PowerProfiles{}, s2)

	add(BluezDevice{
		Label:   "\uf025",
		Adapter: "hci0",
		Name:    "dev_00_1B_66_10_CA_67",
	}, p1)

	add(BluezDevice{
		Label:   "\uf025",
		Adapter: "hci0",
		Name:    "dev_74_F8_DB_95_10_72",
	}, p1)

	add(BluezDevice{
		Label:   "\uf58f",
		Adapter: "hci0",
		Name:    "dev_F0_AE_66_B2_4E_95",
	}, p1)

	add(BluezDevice{
		Label:   "\uf8cd",
		Adapter: "hci0",
		Name:    "dev_DF_78_76_F8_EC_1E", // M575S
	}, s2)

	add(BluezDevice{
		Label:   "\uf11c",
		Adapter: "hci0",
		Name:    "dev_DC_93_71_31_A6_A5", // keyboard
	}, s2)

	add(Interfaces{
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
	})

	add(WiFi{
		Interval:       time.Second * 5,
		Threshold:      -70,
		ThresholdColor: 0xFFFF00FF,
	}, p1, s1, s2)

	add(Disk{
		Interval:       time.Second * 5,
		Threshold:      10 * 1024 * 1024 * 1024,
		ThresholdColor: 0xFFFF00FF,
		Mountpoint:     "/home",
	})

	if is(s2) {
		add(CPU{
			Group: []int{
				0: 1, // P
				1: 1, // P
				2: 1, // P
				3: 1, // P
				4: 2, // LPE
				5: 2, // LPE
				6: 2, // LPE
				7: 2, // LPE
			},
			Interval: time.Second * 3,
		})
	} else {
		add(CPU{
			Interval: time.Second * 3,
		})
	}

	add(Temperature{
		Interval: time.Second * 3,
		Chip:     "k10temp",
	}, d1)

	add(Temperature{
		Interval: time.Second * 3,
		Chip:     "thinkpad",
		Sensor:   "CPU",
	}, p1, s1, s2)

	add(Fan{
		Interval: time.Second * 3,
		Chip:     "thinkpad",
	}, p1, s1, s2)

	add(Memory{
		Interval:       time.Second * 3,
		Threshold:      1 * 1024 * 1024 * 1024,
		ThresholdColor: 0xFFFF00FF,
	})

	add(Time{
		LayoutFull:  "Mon 01/02 15:04:05",
		LayoutShort: "15:04",
		Interval:    time.Second,
		Color:       0x87CEEBFF,
	})

	if niri {
		add(Mako{})
	} else {
		add(Dunst{})
	}

	barlib.Main(time.Second/4, mods...)
}
