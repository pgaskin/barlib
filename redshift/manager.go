// Package redshift is a minimal pure-go implementation of color temperature
// shifting using gamma ramps with support for X11 and Wayland.
package redshift

import (
	"errors"
	"log/slog"
	"os"
)

// Manager controls color ramps for a display manager. It is safe for concurrent
// usage.
type Manager interface {
	// Set sets the color ramp for all current and future outputs, waiting for
	// it to be applied to any current ones.
	Set(WhitePoint)

	// Close closes the connection to the display manager. It may or may not
	// revert the color ramps.
	Close()
}

// New creates a [Manager] for the current display manager, if supported. If a
// fatal error occurs, the chan will return it, and the connection should be
// closed as it will no longer be usable. If logger is not nil, it is used for
// debug logs from this package.
func New(logger *slog.Logger) (Manager, <-chan error, error) {
	switch {
	case os.Getenv("WAYLAND_DISPLAY") != "":
		return NewWayland("", logger)
	case os.Getenv("DISPLAY") != "":
		return NewX11("", logger)
	default:
		return nil, nil, errors.ErrUnsupported
	}
}

// GammaRamp computes a gamma ramp. A white component value of 1 is neutral.
func GammaRamp[C ~uint8 | uint16 | ~uint32 | uint64](r, g, b []C, white WhitePoint) {
	for index := range len(r) {
		r[index] = C(float64(index) / float64(len(r)-1) * float64(^C(0)) * float64(white[0]))
	}
	for index := range len(g) {
		g[index] = C(float64(index) / float64(len(g)-1) * float64(^C(0)) * float64(white[1]))
	}
	for index := range len(b) {
		b[index] = C(float64(index) / float64(len(b)-1) * float64(^C(0)) * float64(white[2]))
	}
}
