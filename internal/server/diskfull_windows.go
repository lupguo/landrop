package server

import (
	"errors"
	"syscall"

	"golang.org/x/sys/windows"
)

// isDiskFull reports whether err means the volume is out of space. Windows
// reports ERROR_DISK_FULL / ERROR_HANDLE_DISK_FULL rather than ENOSPC.
func isDiskFull(err error) bool {
	return errors.Is(err, windows.ERROR_DISK_FULL) ||
		errors.Is(err, windows.ERROR_HANDLE_DISK_FULL) ||
		errors.Is(err, syscall.ENOSPC)
}
