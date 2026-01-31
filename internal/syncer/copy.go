package syncer

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func copyJobDir(dryRun bool, src, dst string) error {
	if dryRun {
		return nil
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dst, err)
	}

	if err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk %s: %w", path, err)
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("rel %s: %w", path, err)
		}
		if rel == "." {
			return nil
		}
		// Security: Prevent path traversal attacks
		if strings.Contains(rel, "..") {
			return fmt.Errorf("%w: %s", errPathTraversal, rel)
		}
		relSlash := filepath.ToSlash(rel)
		if relSlash == ".cronctl" || relSlash == ".git" || relSlash == ".DS_Store" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(relSlash, ".cronctl/") || strings.HasPrefix(relSlash, ".git/") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		outPath := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("info %s: %w", path, err)
		}

		mode := info.Mode()
		switch {
		case mode.IsDir():
			return os.MkdirAll(outPath, mode.Perm())
		case mode.Type()&fs.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("readlink %s: %w", path, err)
			}
			if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", filepath.Dir(outPath), err)
			}
			return os.Symlink(link, outPath)
		case mode.IsRegular():
			if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", filepath.Dir(outPath), err)
			}
			return copyFile(path, outPath, mode.Perm())
		default:
			// Skip devices, sockets, etc.
			return nil
		}
	}); err != nil {
		return fmt.Errorf("walk %s: %w", src, err)
	}
	return nil
}

func copyFile(src, dst string, perm fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy %s -> %s: %w", src, dst, err)
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(dst) // Clean up failed file
		return fmt.Errorf("close %s: %w", dst, err)
	}
	return nil
}
