package server

import (
	"fmt"
	"os"
	"testing"

	"golang.org/x/sys/windows"
)

func TestIsDiskFullWindows(t *testing.T) {
	for _, e := range []error{windows.ERROR_DISK_FULL, windows.ERROR_HANDLE_DISK_FULL} {
		if !isDiskFull(fmt.Errorf("w: %w", &os.PathError{Op: "write", Path: "x", Err: e})) {
			t.Fatalf("%v not disk full", e)
		}
	}
}
