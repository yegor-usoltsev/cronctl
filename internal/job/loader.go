package job

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// DiscoverRaw walks jobsDir and returns jobs found as jobs/<id>/job.yaml.
//
// It returns the raw YAML bytes without decoding. This is useful for validate,
// where we want to report YAML and schema errors per-job without failing fast
// on the first parse error.
func DiscoverRaw(ctx context.Context, jobsDir string) ([]Job, error) {
	return discover(ctx, jobsDir, false)
}

// Discover walks jobsDir and returns jobs found as jobs/<id>/job.yaml and
// decodes YAML into the job spec.
func Discover(ctx context.Context, jobsDir string) ([]Job, error) {
	return discover(ctx, jobsDir, true)
}

func discover(ctx context.Context, jobsDir string, parse bool) ([]Job, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("discover jobs: %w", err)
	}

	entries, err := os.ReadDir(jobsDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("jobs dir does not exist: %s: %w", jobsDir, err)
		}
		return nil, fmt.Errorf("read jobs dir: %s: %w", jobsDir, err)
	}

	jobs := make([]Job, 0, len(entries))
	for _, ent := range entries {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("discover jobs: %w", err)
		}
		if !ent.IsDir() {
			continue
		}
		id := ent.Name()
		if strings.HasPrefix(id, ".") {
			continue
		}
		jobDir := filepath.Join(jobsDir, id)
		yamlPath := filepath.Join(jobDir, "job.yaml")
		raw, err := os.ReadFile(yamlPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("read job yaml: %s: %w", yamlPath, err)
		}

		j := New(id, jobDir, yamlPath, raw)
		if parse {
			var spec Spec
			if err := yaml.Unmarshal(raw, &spec); err != nil {
				return nil, fmt.Errorf("parse yaml: %s: %w", yamlPath, err)
			}
			j.Spec = spec
		}
		jobs = append(jobs, j)
	}

	sort.Slice(jobs, func(i, j int) bool { return jobs[i].ID < jobs[j].ID })
	return jobs, nil
}
