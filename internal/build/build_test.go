package build

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/yegor-usoltsev/cronctl/internal/job"
)

func TestAll_SkipsAndWritesState(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	jobsDir := filepath.Join(root, "jobs")
	jobDir := filepath.Join(jobsDir, "a-job")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Job spec (parsed jobs are passed to build).
	j := job.Job{ID: "a-job", Dir: jobDir, Spec: job.Spec{Name: "a-job", Enabled: true, User: "root", Tags: []string{}, Build: job.BuildSpec{Enabled: true, Entrypoint: "build.sh"}, Run: job.RunSpec{Entrypoint: "run.sh"}, Schedule: []job.ScheduleItem{}}}

	if err := os.WriteFile(filepath.Join(jobDir, "build.sh"), []byte("#!/usr/bin/env bash\nset -euo pipefail\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write build.sh: %v", err)
	}

	if err := All(context.Background(), jobsDir, []job.Job{j}, Options{Parallel: 1}); err != nil {
		t.Fatalf("All: %v", err)
	}

	statePath := filepath.Join(jobDir, ".cronctl", "filehash")
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("expected filehash, stat: %v", err)
	}

	// Second run should hit cache.
	if err := All(context.Background(), jobsDir, []job.Job{j}, Options{Parallel: 1}); err != nil {
		t.Fatalf("All (2): %v", err)
	}
}
