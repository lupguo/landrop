// Package instance records the running server's port and local-API secret so
// `landrop send` can find it.
package instance

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Info describes a running LanDrop instance.
type Info struct {
	PID    int    `json:"pid"`
	Port   int    `json:"port"`
	Secret string `json:"secret"`
}

// DefaultPath is ~/.landrop/instance.json.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".landrop", "instance.json")
}

// Write stores i at path (directory 0700, file 0600), replacing any old file.
func Write(path string, i Info) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	os.Chmod(dir, 0o700)
	b, err := json.Marshal(i)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".instance-*.json")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(b)
	cerr := tmp.Close()
	if werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(tmp.Name(), 0o600)
	}
	if werr == nil {
		werr = os.Rename(tmp.Name(), path)
	}
	if werr != nil {
		os.Remove(tmp.Name())
	}
	return werr
}

// Read loads the instance file at path.
func Read(path string) (Info, error) {
	var i Info
	b, err := os.ReadFile(path)
	if err != nil {
		return i, err
	}
	err = json.Unmarshal(b, &i)
	return i, err
}

// Remove deletes the instance file, ignoring errors.
func Remove(path string) { os.Remove(path) }
