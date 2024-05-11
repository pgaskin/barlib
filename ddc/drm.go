package ddc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// FindMonitor finds a DRM card name by the monitor's EDID's PNP ID and serial.
func FindMonitor(id string) (string, error) {
	// https://en.wikipedia.org/wiki/Extended_Display_Identification_Data

	cfs, err := os.ReadDir("/sys/class/drm")
	if err != nil {
		return "", fmt.Errorf("list drm nodes: %w", err)
	}
	for _, cf := range cfs {
		if !strings.HasPrefix(cf.Name(), "card") {
			continue
		}
		buf, err := os.ReadFile(filepath.Join("/sys/class/drm", cf.Name(), "edid"))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return "", fmt.Errorf("read %s edid: %w", cf.Name(), err)
		}
		if !bytes.HasPrefix(buf, []byte{0x00, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00}) {
			continue // bad edid header
		}
		if len(buf) < 16 {
			continue // too short
		}
		vnd := binary.BigEndian.Uint16(buf[8:10])
		prd := buf[10:12]
		ser := buf[12:16]
		str := string([]byte{
			// pnp vendor
			'A' - 1 + byte(0b11111&(vnd>>(5*2))),
			'A' - 1 + byte(0b11111&(vnd>>(5*1))),
			'A' - 1 + byte(0b11111&(vnd>>(5*0))),
			// product code
			"0123456789ABCDEF"[prd[0]>>4],
			"0123456789ABCDEF"[prd[0]&0xf],
			"0123456789ABCDEF"[prd[1]>>4],
			"0123456789ABCDEF"[prd[1]&0xf],
			'-', // serial
			"0123456789ABCDEF"[ser[0]>>4],
			"0123456789ABCDEF"[ser[0]&0xf],
			"0123456789ABCDEF"[ser[1]>>4],
			"0123456789ABCDEF"[ser[1]&0xf],
			"0123456789ABCDEF"[ser[2]>>4],
			"0123456789ABCDEF"[ser[2]&0xf],
			"0123456789ABCDEF"[ser[3]>>4],
			"0123456789ABCDEF"[ser[3]&0xf],
		})
		if str == id {
			return cf.Name(), nil
		}
	}
	return "", err
}

// FindI2C finds I2C devices exposed on a DRM card.
func FindI2C(card string) ([]int, error) {
	// https://www.kernel.org/doc/Documentation/i2c/dev-interface

	cfs, err := os.ReadDir(filepath.Join("/sys/class/drm", card))
	if err != nil {
		return nil, err
	}
	var i2cs []int
	for _, x := range cfs {
		if s, ok := strings.CutPrefix(x.Name(), "i2c-"); ok {
			if n, err := strconv.ParseInt(s, 10, 0); err == nil {
				i2cs = append(i2cs, int(n))
			}
		}
	}
	return i2cs, nil
}
