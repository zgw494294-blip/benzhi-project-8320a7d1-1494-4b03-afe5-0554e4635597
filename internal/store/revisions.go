package store

import (
	"corepreservation/internal/domain"
	"encoding/json"
	"os"
	"path/filepath"
)

func WriteRevision(dir, id string, v any) error {
	if dir == "" {
		return nil
	}
	d := filepath.Join(dir, "revisions")
	if e := os.MkdirAll(d, 0755); e != nil {
		return e
	}
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(d, id+"-"+domain.Digest(v)+".json"), b, 0644)
}
