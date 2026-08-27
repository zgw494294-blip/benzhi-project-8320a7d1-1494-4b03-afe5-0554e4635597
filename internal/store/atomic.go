package store

import (
	"os"
	"path/filepath"
)

func AtomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if e := os.WriteFile(tmp, data, 0644); e != nil {
		return e
	}
	return os.Rename(tmp, filepath.Clean(path))
}
