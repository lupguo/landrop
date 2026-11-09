package server

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"testing"
)

func TestIsDiskFull(t *testing.T) {
	if !isDiskFull(syscall.ENOSPC) {
		t.Fatal("ENOSPC")
	}
	if !isDiskFull(fmt.Errorf("copy: %w", &os.PathError{Op: "write", Path: "x", Err: syscall.ENOSPC})) {
		t.Fatal("wrapped ENOSPC")
	}
	for _, err := range []error{nil, errors.New("disk full"), syscall.EACCES, os.ErrNotExist} {
		if isDiskFull(err) {
			t.Fatalf("%v reported as disk full", err)
		}
	}
}
