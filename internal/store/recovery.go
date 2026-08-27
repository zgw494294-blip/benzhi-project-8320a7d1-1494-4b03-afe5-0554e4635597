package store

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func ValidateSnapshot(dir string) error {
	if dir == "" {
		return nil
	}
	b, e := os.ReadFile(filepath.Join(dir, "snapshot.json"))
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	var v any
	return json.Unmarshal(b, &v)
}
