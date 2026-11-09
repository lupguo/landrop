package diskfree

import "testing"

func TestFree(t *testing.T) {
	n, err := Free(t.TempDir())
	if err != nil || n == 0 {
		t.Fatal(n, err)
	}
}
