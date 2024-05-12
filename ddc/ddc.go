// Package ddc provides utilities for setting VCPs on DDC-CI capable monitors.
package ddc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strconv"
	"syscall"
	"time"
)

const (
	_IOCTL_I2C_SLAVE = 0x0703
	_I2C_ADDR_DDC_CI = 0x37
	_I2C_ADDR_HOST   = 0x51
)

// Some widely-supported VCPs.
const (
	VCP_Brightness = 0x10
	VCP_Contrast   = 0x12
)

// Some errors.
var (
	ErrDeviceGone     = syscall.Errno(syscall.EREMOTEIO)
	ErrChecksum       = errors.New("invalid ddc checksum")
	ErrBadReply       = errors.New("bad ddc reply")
	ErrNoReply        = errors.New("no ddc reply")
	ErrUnsupportedVCP = errors.New("unsupported ddc vcp code")
)

// CI is an open connection to an I2C bus with a DDC-CI slave.
type CI struct {
	f    *os.File
	next time.Time
}

// Open opens a DDC-CI I2C bus.
func Open(i2c int) (*CI, error) {
	f, err := os.OpenFile("/dev/i2c-"+strconv.Itoa(i2c), os.O_RDWR, os.ModeDevice)
	if err != nil {
		return nil, err
	}
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), _IOCTL_I2C_SLAVE, _I2C_ADDR_DDC_CI); errno != 0 {
		f.Close()
		return nil, fmt.Errorf("failed to open address 0x%X on i2c bus %d: %w", _I2C_ADDR_DDC_CI, i2c, syscall.Errno(errno))
	}
	return &CI{f: f}, nil
}

// GetVCP gets the value and maximum of a uint16 VCP.
func (d *CI) GetVCP(vcp byte) (uint16, uint16, error) {
	if err := d.tx([]byte{0x01, vcp}, time.Millisecond*40); err != nil {
		return 0, 0, err
	}
	for retry := 0; retry < 5; retry++ {
		buf, err := d.rx()
		if errors.Is(err, ErrNoReply) || (err == nil && len(buf) == 0) {
			d.next = time.Now().Add(time.Millisecond * 40)
			continue
		}
		if err != nil {
			return 0, 0, err
		}
		if len(buf) != 8 {
			return 0, 0, fmt.Errorf("%w: unexpected ddc vcp response length %d", ErrBadReply, len(buf))
		}
		if reply := buf[0]; reply != 0x02 {
			return 0, 0, fmt.Errorf("%w: unexpected ddc reply opcode %d", ErrBadReply, reply)
		}
		if result := buf[1]; result != 0x00 {
			if result == 0x01 {
				return 0, 0, fmt.Errorf("%w 0x%02X", ErrUnsupportedVCP, vcp)
			}
			return 0, 0, fmt.Errorf("%w: unexpected ddc reply result code %d", ErrBadReply, result)
		}
		if retVCP := buf[2]; retVCP != vcp {
			return 0, 0, fmt.Errorf("%w: unexpected ddc reply vcp code 0x%02X (we requested 0x%02X)", ErrBadReply, retVCP, vcp)
		}
		var (
			max = binary.BigEndian.Uint16(buf[4:6])
			val = binary.BigEndian.Uint16(buf[6:8])
		)
		return val, max, nil
	}
	return 0, 0, ErrNoReply
}

// SetVCP sets a VCP.
func (d *CI) SetVCP(vcp byte, val uint16) error {
	// https://glenwing.github.io/docs/VESA-DDCCI-1.1.pdf page 20
	return d.tx([]byte{0x03, vcp, byte(val >> 8), byte(val)}, time.Millisecond*50)
}

// Close closes the device.
func (d *CI) Close() error {
	return d.f.Close()
}

func (d *CI) tx(cmd []byte, wait time.Duration) error {
	// https://glenwing.github.io/docs/VESA-DDCCI-1.1.pdf

	// rate-limit commands
	d.wait()

	// header (excluding slave address)
	buf := append([]byte{
		_I2C_ADDR_HOST,
		0x80 | byte(len(cmd)),
	}, cmd...)

	// checksum
	buf = append(buf, _I2C_ADDR_DDC_CI<<1)
	for i, ck := 0, len(buf)-1; i < ck; i++ {
		buf[ck] ^= buf[i]
	}

	// send
	_, err := d.f.Write(buf)
	if err == nil {
		d.next = time.Now().Add(wait)
	}
	return err
}

func (d *CI) rx() ([]byte, error) {
	// https://glenwing.github.io/docs/VESA-DDCCI-1.1.pdf

	// rate-limit commands
	d.wait()

	// read header
	hdr := make([]byte, 2)
	n, err := d.f.Read(hdr)
	if err == nil && n != len(hdr) {
		err = fmt.Errorf("short ddc header read, expected %d bytes, got %d", len(hdr), n)
	}
	if err != nil {
		return nil, err
	}

	// check source
	var (
		hdrAddr = hdr[0] >> 1
		pktLen  = int(hdr[1] &^ 0x80)
	)
	if hdrAddr == 0 {
		return nil, ErrNoReply
	}
	if hdrAddr != _I2C_ADDR_DDC_CI {
		return nil, fmt.Errorf("bad ddc source address 0x%X", hdrAddr)
	}
	if hdr[1]&0x80 == 0 {
		return nil, fmt.Errorf("bad ddc header length: flag 0x80 not set")
	}

	// read payload
	buf := make([]byte, pktLen+1)
	n, err = d.f.Read(buf)
	if err != nil && n != len(buf) {
		err = fmt.Errorf("short ddc payload read, expected %d bytes, got %d", len(buf), n)
	}
	if err != nil {
		return nil, err
	}

	// checksum
	buf[pktLen] ^= (_I2C_ADDR_HOST - 1)
	for i := 0; i < len(hdr); i++ {
		buf[pktLen] ^= hdr[i]
	}
	for i := 0; i < pktLen; i++ {
		buf[pktLen] ^= buf[i]
	}
	if buf[pktLen] != 0 {
		return nil, ErrChecksum
	}
	buf = buf[:pktLen]

	return buf, nil
}

func (d *CI) wait() {
	for t := time.Now(); t.Before(d.next); t = time.Now() {
		time.Sleep(t.Sub(d.next))
	}
}
