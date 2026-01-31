package build

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type ignoreRule struct {
	negate   bool
	dirOnly  bool
	anchored bool
	re       *regexp.Regexp
}

type ignoreMatcher struct {
	rules []ignoreRule
}

func (m ignoreMatcher) match(rel string, isDir bool) bool {
	rel = filepath.ToSlash(rel)
	if rel == "" || rel == "." {
		return false
	}

	ignored := false
	for _, r := range m.rules {
		if r.dirOnly && !isDir {
			continue
		}
		if r.re == nil {
			continue
		}
		if !r.anchored {
			// Gitignore patterns without a slash can match basenames anywhere.
			// We approximate by matching both full rel and basename.
			if !r.re.MatchString(rel) && !r.re.MatchString(filepath.Base(rel)) {
				continue
			}
		} else {
			if !r.re.MatchString(rel) {
				continue
			}
		}
		if r.negate {
			ignored = false
		} else {
			ignored = true
		}
	}
	return ignored
}

func loadJobIgnore(jobDir string) (ignoreMatcher, error) {
	// Defaults: always ignore internal state and git metadata.
	lines := []string{
		".cronctl/",
		".git/",
	}

	path := filepath.Join(jobDir, ".gitignore")
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return compileIgnore(lines), nil
		}
		return ignoreMatcher{}, fmt.Errorf("read %s: %w", path, err)
	}
	s := bufio.NewScanner(strings.NewReader(string(b)))
	for s.Scan() {
		lines = append(lines, s.Text())
	}
	if err := s.Err(); err != nil {
		return ignoreMatcher{}, fmt.Errorf("scan %s: %w", path, err)
	}
	return compileIgnore(lines), nil
}

func compileIgnore(lines []string) ignoreMatcher {
	rules := make([]ignoreRule, 0, len(lines))
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		r := ignoreRule{negate: false, dirOnly: false, anchored: false, re: nil}
		if strings.HasPrefix(line, "!") {
			r.negate = true
			line = strings.TrimSpace(strings.TrimPrefix(line, "!"))
		}
		if strings.HasSuffix(line, "/") {
			r.dirOnly = true
			line = strings.TrimSuffix(line, "/")
		}
		if strings.HasPrefix(line, "/") {
			r.anchored = true
			line = strings.TrimPrefix(line, "/")
		}
		if line == "" {
			continue
		}
		re, ok := gitignorePatternToRegexp(line, r.anchored)
		if !ok {
			continue
		}
		r.re = re
		rules = append(rules, r)
	}
	return ignoreMatcher{rules: rules}
}

func gitignorePatternToRegexp(pat string, anchored bool) (*regexp.Regexp, bool) {
	// Minimal gitignore -> regexp conversion.
	// Supported: '*', '?', character classes, and '**' spanning directories.
	// For anchored patterns, match from start. Otherwise, match anywhere.
	var b strings.Builder
	if anchored {
		b.WriteString("^")
	} else {
		b.WriteString("(?:^|.*/)")
	}

	for i := 0; i < len(pat); i++ {
		c := pat[i]
		switch c {
		case '*':
			// '**' => any chars including '/'
			if i+1 < len(pat) && pat[i+1] == '*' {
				b.WriteString(".*")
				i++
				continue
			}
			b.WriteString("[^/]*")
		case '?':
			b.WriteString("[^/]")
		case '.', '+', '(', ')', '|', '^', '$', '{', '}', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteString("$")

	re, err := regexp.Compile(b.String())
	if err != nil {
		return nil, false
	}
	return re, true
}

func detectGitRoot(ctx context.Context, dir string) (string, bool) {
	if ctx.Err() != nil {
		return "", false
	}
	if _, err := exec.LookPath("git"); err != nil {
		return "", false
	}
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return "", false
	}
	if _, err := os.Stat(root); err != nil {
		return "", false
	}
	return root, true
}

func gitListInputs(ctx context.Context, gitRoot, jobDir string) ([]string, error) {
	jobAbs, err := filepath.Abs(jobDir)
	if err != nil {
		return nil, fmt.Errorf("abs job dir: %w", err)
	}
	rootAbs, err := filepath.Abs(gitRoot)
	if err != nil {
		return nil, fmt.Errorf("abs git root: %w", err)
	}
	rel, err := filepath.Rel(rootAbs, jobAbs)
	if err != nil {
		return nil, fmt.Errorf("rel job dir: %w", err)
	}
	rel = filepath.ToSlash(rel)

	// Include tracked + untracked, exclude ignored (uses hierarchical .gitignore).
	cmd := exec.CommandContext(ctx, "git", "-C", rootAbs, "ls-files", "--cached", "--others", "--exclude-standard", "--", rel)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}
	// Normalize and sort.
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	sort.Strings(lines)
	return lines, nil
}
