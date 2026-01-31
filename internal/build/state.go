package build

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ReadHash(path string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	s := strings.TrimSpace(string(b))
	if s == "" {
		return "", false
	}
	return s, true
}

func WriteHash(path string, hash string) error {
	if strings.TrimSpace(hash) == "" {
		return errEmptyHash
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir state dir: %w", err)
	}

	data := []byte(hash + "\n")
	tmp := path + ".tmp"
	// #nosec G306 -- this is non-secret cache metadata.
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp state: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename state: %w", err)
	}
	return nil
}

func StateFilePath(jobDir string) string {
	return filepath.Join(jobDir, ".cronctl", "filehash")
}
