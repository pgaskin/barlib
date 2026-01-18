package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"unsafe"

	"go.i3wm.org/i3/v4"
)

var niri = os.Getenv("NIRI_SOCKET") != ""

func readFileInt[T ~int | ~int8 | ~int16 | ~int32 | ~int64](name string) (T, error) {
	var z T
	b, err := os.ReadFile(name)
	if err != nil {
		return z, err
	}
	v, err := strconv.ParseInt(string(bytes.TrimSpace(b)), 10, int(unsafe.Sizeof(z)*8))
	if err != nil {
		return z, err
	}
	return T(v), nil
}

func readFileUint[T ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64](name string) (T, error) {
	var z T
	b, err := os.ReadFile(name)
	if err != nil {
		return z, err
	}
	v, err := strconv.ParseUint(string(bytes.TrimSpace(b)), 10, int(unsafe.Sizeof(z)*8))
	if err != nil {
		return z, err
	}
	return T(v), nil
}

func i3msg(msg string) error {
	_, err := i3.RunCommand(msg)
	return err
}

func nirimsg(msg ...string) error {
	// TODO: native go version?
	cmd := exec.Command("niri", "msg")
	cmd.Args = append(cmd.Args, msg...)
	if _, err := cmd.Output(); err != nil {
		if xx, ok := errors.AsType[*exec.ExitError](err); ok {
			if len(xx.Stderr) != 0 {
				err = fmt.Errorf("%w (stderr: %q)", err, xx.Stderr)
			}
		}
		return err
	}
	return nil
}
