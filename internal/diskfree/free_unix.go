//go:build !windows

package diskfree

import "golang.org/x/sys/unix"

// Free returns the bytes available to unprivileged users on the filesystem containing path.
func Free(path string) (uint64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}
