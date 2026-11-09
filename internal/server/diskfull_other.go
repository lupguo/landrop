//go:build !windows

package server

import (
	"errors"
	"syscall"
)

// isDiskFull reports whether err means the volume is out of space.
func isDiskFull(err error) bool {
	return errors.Is(err, syscall.ENOSPC)
}
