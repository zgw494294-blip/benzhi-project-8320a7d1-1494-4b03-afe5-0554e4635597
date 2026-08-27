package store

import (
	"encoding/json"
	"fmt"
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
	var v map[string]json.RawMessage
	if e := json.Unmarshal(b, &v); e != nil {
		return e
	}
	for _, key := range []string{"cores", "coreVersions", "cases", "caseVersions", "prechecks", "authorizations", "executions", "executionReceipts", "verificationAttempts", "verificationKeys", "findingEvents", "credentials"} {
		if _, ok := v[key]; !ok {
			return fmt.Errorf("snapshot missing %s", key)
		}
	}
	return nil
}
