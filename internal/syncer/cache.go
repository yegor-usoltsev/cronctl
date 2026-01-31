package syncer

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func carryOverFilehash(dryRun bool, deployedDir, stagingDir string) error {
	from := filepath.Join(deployedDir, ".cronctl", "filehash")
	toDir := filepath.Join(stagingDir, ".cronctl")
	to := filepath.Join(toDir, "filehash")

	if dryRun {
		log.Printf("dry-run: carry over cache %s -> %s", from, to)
		return nil
	}
	b, err := os.ReadFile(from)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read %s: %w", from, err)
	}
	if err := os.MkdirAll(toDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", toDir, err)
	}
	// #nosec G306 -- this is non-secret cache metadata.
	if err := os.WriteFile(to, b, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", to, err)
	}
	return nil
}
