package main

import (
	"bytes"
	"os"
	"strconv"
	"unsafe"

	"go.i3wm.org/i3/v4"
)

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
