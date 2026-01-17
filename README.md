<h1 align="center">barlib</h1>

<a href="https://pkg.go.dev/github.com/pgaskin/barlib"><img align="right" src="https://pkg.go.dev/badge/github.com/pgaskin/barlib" alt="PkgGoDev"></a>

**A simple but flexible library for implementing efficient, fast, responsive, and
error-tolerant i3status replacements in Go.**

- Very flexible immediate-mode API.
- Per-module error handling and error recovery with proper cleanup.
- Multiple blocks per module with custom event handling.
- Memory/CPU efficency.
- Bar stop/continue handling.
- Aligned ticks across all modules with customizable global base tick rate (so the bar sleeps for as long as possible between updates).
- Update coalescing (so the bar updates all at once when multiple modules update at around the same time).
- Implements [i3bar protocol](https://i3wm.org/docs/i3bar-protocol.html) version 1 for [i3bar](https://github.com/i3/i3/tree/next/i3bar) v4.3+.
- Compatible with [i3bar-river](https://github.com/MaxVerevkin/i3bar-river) and [swaybar](https://github.com/swaywm/sway/tree/master/swaybar) on wayland.
- Unique sample module features not seen in other i3status implementations, like:
  - DDC-CI monitor brightness/contrast control.
  - Integrated display color temperature control.
  - Multi-state modules.
  - Battery charge limit display.
  - Extremely powerful mouse-driven media player control.
  - Automatic display layout presets.
- Highly optimized event-driven pure-go modules which react properly to external changes.

The almost fully event-driven nature, synchronized ticks/updates, and unified socket connections across modules makes this implementation significantly more power-efficient than other alternatives like py3status, waybar, and i3status-rust.

#### Usage

See [example_test.go](./example_test.go) for basic barlib usage, and [i3status-custom](./i3status-custom) for example modules I use myself.
