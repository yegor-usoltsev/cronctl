package syncer

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func pruneOrphans(dryRun bool, cronDir string, keep map[string]struct{}) error {
	ents, err := os.ReadDir(cronDir)
	if err != nil {
		return fmt.Errorf("read cron dir %s: %w", cronDir, err)
	}
	for _, e := range ents {
		name := e.Name()
		if !strings.HasPrefix(name, "cronctl-") {
			continue
		}
		id := strings.TrimPrefix(name, "cronctl-")
		if _, ok := keep[id]; ok {
			continue
		}
		path := filepath.Join(cronDir, name)
		if dryRun {
			log.Printf("dry-run: prune orphan cron %s", path)
			continue
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove orphan %s: %w", path, err)
		}
	}
	return nil
}
