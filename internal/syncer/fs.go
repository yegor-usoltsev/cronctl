package syncer

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
)

func removeFileIfExists(dryRun bool, path string) error {
	if dryRun {
		log.Printf("dry-run: remove file %s", path)
		return nil
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

func removeDirIfExists(dryRun bool, path string) error {
	if dryRun {
		log.Printf("dry-run: remove dir %s", path)
		return nil
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove dir %s: %w", path, err)
	}
	return nil
}

func stageJobDir(dryRun bool, targetDir, jobID string) (string, error) {
	if dryRun {
		// Use deterministic placeholder in logs; it won't be used for I/O.
		return filepath.Join(targetDir, ".cronctl-staging-"+jobID), nil
	}
	staging, err := os.MkdirTemp(targetDir, ".cronctl-staging-"+jobID+"-")
	if err != nil {
		return "", fmt.Errorf("mkdir temp in %s: %w", targetDir, err)
	}
	return staging, nil
}

func replaceDir(dryRun bool, srcTmp, dst string) error {
	if dryRun {
		log.Printf("dry-run: replace dir %s -> %s", srcTmp, dst)
		return nil
	}
	// Try to make the swap robust: move current aside, move new in, then delete old.
	backup := dst + ".cronctl-old"
	_ = os.RemoveAll(backup)
	if err := os.Rename(dst, backup); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("rename %s -> %s: %w", dst, backup, err)
		}
	}
	if err := os.Rename(srcTmp, dst); err != nil {
		// Best-effort rollback.
		_ = os.Rename(backup, dst)
		return fmt.Errorf("rename %s -> %s: %w", srcTmp, dst, err)
	}
	_ = os.RemoveAll(backup)
	return nil
}

func writeFileAtomic(path string, perm fs.FileMode, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".cronctl-tmp-")
	if err != nil {
		return fmt.Errorf("create temp in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	// Set explicit permissions for the temp file
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp %s: %w", tmpName, err)
	}

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp %s: %w", tmpName, err)
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName) // Clean up failed file
		return fmt.Errorf("close temp %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp %s -> %s: %w", tmpName, path, err)
	}
	return nil
}
