// Copyright © 2015 Giulio camuffo
// Copyright © 2018 Simon Ser
//
// Permission to use, copy, modify, distribute, and sell this
// software and its documentation for any purpose is hereby granted
// without fee, provided that the above copyright notice appear in
// all copies and that both that copyright notice and this permission
// notice appear in supporting documentation, and that the name of
// the copyright holders not be used in advertising or publicity
// pertaining to distribution of the software without specific,
// written prior permission.  The copyright holders make no
// representations about the suitability of this software for any
// purpose.  It is provided "as is" without express or implied
// warranty.
//
// THE COPYRIGHT HOLDERS DISCLAIM ALL WARRANTIES WITH REGARD TO THIS
// SOFTWARE, INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY AND
// FITNESS, IN NO EVENT SHALL THE COPYRIGHT HOLDERS BE LIABLE FOR ANY
// SPECIAL, INDIRECT OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
// WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN
// AN ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION,
// ARISING OUT OF OR IN CONNECTION WITH THE USE OR PERFORMANCE OF
// THIS SOFTWARE.

package zwlr

import (
	"fmt"

	"codeberg.org/tesselslate/wl"
)

// # Manager to create per-output gamma controls
//
// This interface is a manager that allows creating per-output gamma
// controls.
type GammaControlManagerV1 wl.Object

// Note: Do not modify this variable.
var GammaControlManagerV1Interface = wl.Interface{
	ErrorStr: nil,
	Dispatch: nil,
	NumFd:    nil,
	Name:     "zwlr_gamma_control_manager_v1",
}

// # Get a gamma control for an output
//
// Create a gamma control that can be used to adjust gamma tables for the
// provided output.
func (S *GammaControlManagerV1) GetGammaControl(output wl.Output) GammaControlV1 {
	O := (*wl.Object)(S)
	M := wl.NewMessage(0)
	R := M.WriteNewIdStatic(*O, &GammaControlV1Interface)
	M.WriteObject(wl.Object(output), false)
	M.WriteHeader(O.GetId(), 0)
	O.Enqueue(M)

	if O.Debug() {
		M.DebugRequest(O.GetDisplay(), "get_gamma_control", wl.NewId(R), wl.Object(output))
	}
	return GammaControlV1(R)
}

// # Destroy the manager
//
// All objects created by the manager will still remain valid, until their
// appropriate destroy request has been called.
func (S *GammaControlManagerV1) Destroy() {
	O := (*wl.Object)(S)
	M := wl.NewMessage(0)
	M.WriteHeader(O.GetId(), 1)
	O.Enqueue(M)

	if O.Debug() {
		M.DebugRequest(O.GetDisplay(), "destroy")
	}
	O.Destroy()
}

// # Adjust gamma tables for an output
//
// This interface allows a client to adjust gamma tables for a particular
// output.
//
// The client will receive the gamma size, and will then be able to set gamma
// tables. At any time the compositor can send a failed event indicating that
// this object is no longer valid.
//
// There can only be at most one gamma control object per output, which
// has exclusive access to this particular output. When the gamma control
// object is destroyed, the gamma table is restored to its original value.
type GammaControlV1 wl.Object

type GammaControlV1Listener struct {
	// # Size of gamma ramps
	//
	// Advertise the size of each gamma ramp.
	//
	// This event is sent immediately when the gamma control object is created.
	GammaSize func(data any, self GammaControlV1, size uint32) error

	// # Object no longer valid
	//
	// This event indicates that the gamma control is no longer valid. This
	// can happen for a number of reasons, including:
	// - The output doesn't support gamma tables
	// - Setting the gamma tables failed
	// - Another client already has exclusive gamma control for this output
	// - The compositor has transferred gamma control to another client
	//
	// Upon receiving this event, the client should destroy this object.
	Failed func(data any, self GammaControlV1) error

	// Unexported. Forbids unkeyed struct initialization.
	_ struct{}
}

// Note: Do not modify this variable.
var GammaControlV1Interface = wl.Interface{
	ErrorStr: errorStrGammaControlV1,
	Dispatch: []func(wl.Object, wl.Message) error{dispatchGammaControlV1GammaSize, dispatchGammaControlV1Failed},
	NumFd:    []int{0, 0},
	Name:     "zwlr_gamma_control_v1",
}

func errorStrGammaControlV1(code uint32) string {
	return GammaControlV1Error(code).String()
}

// SetListener sets the event listener for the GammaControlV1. Overwriting an existing
// listener is illegal and will result in a panic.
func (o *GammaControlV1) SetListener(listener GammaControlV1Listener, data any) {
	(*wl.Object)(o).SetListener(listener, data)
}

type GammaControlV1Error int32

const (
	GammaControlV1ErrorInvalidGamma GammaControlV1Error = 1 // Invalid gamma tables
)

const strGammaControlV1Error = "invalid_gamma"

var mapGammaControlV1Error = map[GammaControlV1Error]string{1: strGammaControlV1Error[0:13]}

func (v GammaControlV1Error) String() string {
	if str, ok := mapGammaControlV1Error[v]; ok {
		return str
	}
	return fmt.Sprintf("GammaControlV1Error(%d)", v)
}

func dispatchGammaControlV1GammaSize(O wl.Object, M wl.Message) error {
	size, err := M.ReadUint()
	if err != nil {
		return err
	}

	L, K := O.GetListener().(GammaControlV1Listener)
	if !K && O.Debug() {
		M.DebugEvent(O.GetDisplay(), true, "gamma_size", size)
		return nil
	}

	F := L.GammaSize
	if O.Debug() {
		M.DebugEvent(O.GetDisplay(), F == nil, "gamma_size", size)
	}

	var R error
	if F != nil {
		R = F(O.GetData(), GammaControlV1(O), size)
	}
	return R
}

func dispatchGammaControlV1Failed(O wl.Object, M wl.Message) error {

	L, K := O.GetListener().(GammaControlV1Listener)
	if !K && O.Debug() {
		M.DebugEvent(O.GetDisplay(), true, "failed")
		return nil
	}

	F := L.Failed
	if O.Debug() {
		M.DebugEvent(O.GetDisplay(), F == nil, "failed")
	}

	var R error
	if F != nil {
		R = F(O.GetData(), GammaControlV1(O))
	}
	return R
}

// # Set the gamma table
//
// Set the gamma table. The file descriptor can be memory-mapped to provide
// the raw gamma table, which contains successive gamma ramps for the red,
// green and blue channels. Each gamma ramp is an array of 16-byte unsigned
// integers which has the same length as the gamma size.
//
// The file descriptor data must have the same length as three times the
// gamma size.
func (S *GammaControlV1) SetGamma(fd int) {
	O := (*wl.Object)(S)
	M := wl.NewMessage(1)
	M.WriteFd(fd)
	M.WriteHeader(O.GetId(), 0)
	O.Enqueue(M)

	if O.Debug() {
		M.DebugRequest(O.GetDisplay(), "set_gamma", fd)
	}
}

// # Destroy this control
//
// Destroys the gamma control object. If the object is still valid, this
// restores the original gamma tables.
func (S *GammaControlV1) Destroy() {
	O := (*wl.Object)(S)
	M := wl.NewMessage(0)
	M.WriteHeader(O.GetId(), 1)
	O.Enqueue(M)

	if O.Debug() {
		M.DebugRequest(O.GetDisplay(), "destroy")
	}
	O.Destroy()
}
