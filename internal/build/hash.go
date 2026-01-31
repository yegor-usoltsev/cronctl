package build

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// InputsHash computes the hash for a job directory.
//
// In a git repository, it relies on `git ls-files` to respect all applicable
// .gitignore files (from the repo root down to the job directory), including
// untracked-but-not-ignored files.
//
// Outside of git, it walks the job directory and applies only the job-local
// .gitignore.
func InputsHash(ctx context.Context, jobDir string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("hash inputs: %w", err)
	}
	if root, ok := detectGitRoot(ctx, jobDir); ok {
		return gitInputsHash(ctx, root, jobDir)
	}
	m, err := loadJobIgnore(jobDir)
	if err != nil {
		return "", fmt.Errorf("load job ignore: %w", err)
	}
	return walkHash(ctx, jobDir, m)
}

func gitInputsHash(ctx context.Context, gitRoot, jobDir string) (string, error) {
	paths, err := gitListInputs(ctx, gitRoot, jobDir)
	if err != nil {
		return "", err
	}
	jobAbs, err := filepath.Abs(jobDir)
	if err != nil {
		return "", fmt.Errorf("abs job dir: %w", err)
	}
	rootAbs, err := filepath.Abs(gitRoot)
	if err != nil {
		return "", fmt.Errorf("abs git root: %w", err)
	}
	relJobFromRoot, err := filepath.Rel(rootAbs, jobAbs)
	if err != nil {
		return "", fmt.Errorf("rel job dir: %w", err)
	}
	relJobFromRoot = filepath.ToSlash(relJobFromRoot)
	if relJobFromRoot != "" && relJobFromRoot != "." {
		relJobFromRoot += "/"
	}

	h := sha256.New()
	for _, p := range paths {
		if p == "" {
			continue
		}
		p = filepath.ToSlash(p)
		if relJobFromRoot != "" {
			p = strings.TrimPrefix(p, relJobFromRoot)
		}
		abs := filepath.Join(rootAbs, filepath.FromSlash(relJobFromRoot+p))
		info, err := os.Stat(abs)
		if err != nil {
			return "", fmt.Errorf("stat input: %s: %w", abs, err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		b, err := os.ReadFile(abs)
		if err != nil {
			return "", fmt.Errorf("read input: %s: %w", abs, err)
		}
		_, _ = io.WriteString(h, p)
		_, _ = h.Write([]byte{0})
		_, _ = io.WriteString(h, info.Mode().String())
		_, _ = h.Write([]byte{0})
		_, _ = io.Copy(h, bytes.NewReader(b))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func walkHash(ctx context.Context, jobDir string, ignore ignoreMatcher) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("walk hash: %w", err)
	}

	type item struct {
		path string
		mode fs.FileMode
		data []byte
	}
	var items []item

	err := filepath.WalkDir(jobDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk: %w", err)
		}
		if ctx.Err() != nil {
			return fmt.Errorf("walk: %w", ctx.Err())
		}
		rel, rerr := filepath.Rel(jobDir, path)
		if rerr != nil {
			return fmt.Errorf("rel: %w", rerr)
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if ignore.match(rel, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return fmt.Errorf("info: %w", ierr)
		}
		if !info.Mode().IsRegular() {
			// Ignore symlinks/devices/etc.
			return nil
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return fmt.Errorf("read file: %w", rerr)
		}
		items = append(items, item{path: rel, mode: info.Mode(), data: b})
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk job dir: %w", err)
	}

	sort.Slice(items, func(i, j int) bool { return items[i].path < items[j].path })
	h := sha256.New()
	for _, it := range items {
		_, _ = io.WriteString(h, it.path)
		_, _ = h.Write([]byte{0})
		_, _ = io.WriteString(h, it.mode.String())
		_, _ = h.Write([]byte{0})
		_, _ = io.Copy(h, bytes.NewReader(it.data))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
