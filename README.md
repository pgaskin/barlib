<h1 align="center">barlib</h1>

**A simple but flexible library for implementing efficient, fast, responsive, and
error-tolerant i3status replacements in Go.**

- Very flexible immediate-mode API.
- Per-module error handling and error recovery with proper cleanup.
- Multiple blocks per module with custom event handling.
- Memory/CPU efficency.
- Bar stop/continue handling.
- Aligned ticks across all modules with customizable global base tick rate (so the bar sleeps for as long as possible between updates).
- Update coalescing (so the bar updates all at once when multiple modules update at around the same time).
- Implements [i3bar protocol](https://i3wm.org/docs/i3bar-protocol.html) version 1 for i3 v4.3+.

#### Usage

See [example_test.go](./example_test.go) for basic barlib usage.