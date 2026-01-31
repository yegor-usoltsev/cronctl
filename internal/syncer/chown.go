package syncer

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
)

// chownTree recursively changes ownership of all files and directories under root to uid:gid.
// If dryRun is true, only logs the intended changes without making modifications.
func chownTree(dryRun bool, root string, uid, gid int) error {
	if dryRun {
		log.Printf("dry-run: chown -R %d:%d %s", uid, gid, root)
		return nil
	}
	if err := filepath.WalkDir(root, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk %s: %w", path, err)
		}
		// Use Lchown so we don't follow symlinks.
		if err := os.Lchown(path, uid, gid); err != nil {
			return fmt.Errorf("lchown %s: %w", path, err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("walk %s: %w", root, err)
	}
	return nil
}
