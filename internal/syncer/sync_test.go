package syncer_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/yegor-usoltsev/cronctl/internal/job"
	"github.com/yegor-usoltsev/cronctl/internal/syncer"
)

func TestSyncDryRun(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create temp job dir
	tmpRoot := t.TempDir()
	jobDir := filepath.Join(tmpRoot, "jobs", "test-job")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write job files
	jobYAML := `$schema: https://cronctl.usoltsev.xyz/v0.json
name: test-job
enabled: true
user: root
tags: []
env:
  PATH: /usr/bin
build:
  enabled: false
run:
  entrypoint: run.sh
schedule:
  - cron: "0 * * * *"
    args: []
    env: {}
`
	if err := os.WriteFile(filepath.Join(jobDir, "job.yaml"), []byte(jobYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "run.sh"), []byte("#!/bin/bash\necho hello\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Load job
	jobs, err := job.Discover(ctx, filepath.Join(tmpRoot, "jobs"))
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	// Dry-run sync
	cronDir := filepath.Join(tmpRoot, "cron.d")
	targetDir := filepath.Join(tmpRoot, "deployed")

	opts := syncer.Options{
		CronDir:   cronDir,
		TargetDir: targetDir,
		DryRun:    true,
	}

	if err := syncer.Sync(ctx, jobs, opts); err != nil {
		t.Fatalf("Sync dry-run failed: %v", err)
	}

	// Verify no files were created (but target dir is OK to exist)
	if _, err := os.Stat(cronDir); !os.IsNotExist(err) {
		t.Errorf("dry-run should not create cron.d directory")
	}
	// Target dir is created by sync even in dry-run (for staging), that's OK
	// But no job dirs should exist
	if _, err := os.Stat(filepath.Join(targetDir, "test-job")); !os.IsNotExist(err) {
		t.Errorf("dry-run should not deploy job payload")
	}
}

func TestSyncDisabledJob(t *testing.T) {
	t.Parallel()
	if os.Geteuid() != 0 {
		t.Skip("skipping test that requires root")
	}

	ctx := context.Background()
	tmpRoot := t.TempDir()
	jobDir := filepath.Join(tmpRoot, "jobs", "disabled-job")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}

	jobYAML := `$schema: https://cronctl.usoltsev.xyz/v0.json
name: disabled-job
enabled: false
user: root
tags: []
env: {}
build:
  enabled: false
run:
  entrypoint: run.sh
schedule: []
`
	if err := os.WriteFile(filepath.Join(jobDir, "job.yaml"), []byte(jobYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "run.sh"), []byte("#!/bin/bash\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	jobs, err := job.Discover(ctx, filepath.Join(tmpRoot, "jobs"))
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	cronDir := filepath.Join(tmpRoot, "cron.d")
	targetDir := filepath.Join(tmpRoot, "deployed")
	cronFile := filepath.Join(cronDir, "cronctl-disabled-job")

	// Create a cron file that should be removed
	if err := os.MkdirAll(cronDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cronFile, []byte("# old cron\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := syncer.Options{
		CronDir:   cronDir,
		TargetDir: targetDir,
		DryRun:    false,
	}

	if err := syncer.Sync(ctx, jobs, opts); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// Verify cron file was removed
	if _, err := os.Stat(cronFile); !os.IsNotExist(err) {
		t.Errorf("cron file should be removed for disabled job")
	}
}

func TestSyncOrphanPruning(t *testing.T) {
	t.Parallel()
	if os.Geteuid() != 0 {
		t.Skip("skipping test that requires root")
	}

	ctx := context.Background()
	tmpRoot := t.TempDir()
	jobDir := filepath.Join(tmpRoot, "jobs", "active-job")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}

	jobYAML := `$schema: https://cronctl.usoltsev.xyz/v0.json
name: active-job
enabled: true
user: root
tags: []
env: {}
build:
  enabled: false
run:
  entrypoint: run.sh
schedule:
  - cron: "0 * * * *"
    args: []
    env: {}
`
	if err := os.WriteFile(filepath.Join(jobDir, "job.yaml"), []byte(jobYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "run.sh"), []byte("#!/bin/bash\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	jobs, err := job.Discover(ctx, filepath.Join(tmpRoot, "jobs"))
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	cronDir := filepath.Join(tmpRoot, "cron.d")
	targetDir := filepath.Join(tmpRoot, "deployed")

	// Create orphan cron files
	if err := os.MkdirAll(cronDir, 0o755); err != nil {
		t.Fatal(err)
	}
	orphanFile := filepath.Join(cronDir, "cronctl-orphan-job")
	if err := os.WriteFile(orphanFile, []byte("# orphan\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Non-cronctl file should not be touched
	otherFile := filepath.Join(cronDir, "other-cron")
	if err := os.WriteFile(otherFile, []byte("# other\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := syncer.Options{
		CronDir:       cronDir,
		TargetDir:     targetDir,
		DryRun:        false,
		RemoveOrphans: true,
	}

	if err := syncer.Sync(ctx, jobs, opts); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// Verify orphan was removed
	if _, err := os.Stat(orphanFile); !os.IsNotExist(err) {
		t.Errorf("orphan cronctl file should be removed")
	}

	// Verify non-cronctl file remains
	if _, err := os.Stat(otherFile); err != nil {
		t.Errorf("non-cronctl file should not be touched: %v", err)
	}

	// Verify active job's cron file exists
	activeFile := filepath.Join(cronDir, "cronctl-active-job")
	if _, err := os.Stat(activeFile); err != nil {
		t.Errorf("active job cron file should exist: %v", err)
	}
}

func TestSyncEmptySchedule(t *testing.T) {
	t.Parallel()
	if os.Geteuid() != 0 {
		t.Skip("skipping test that requires root")
	}

	ctx := context.Background()
	tmpRoot := t.TempDir()
	jobDir := filepath.Join(tmpRoot, "jobs", "no-schedule")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}

	jobYAML := `$schema: https://cronctl.usoltsev.xyz/v0.json
name: no-schedule
enabled: true
user: root
tags: []
env: {}
build:
  enabled: false
run:
  entrypoint: run.sh
schedule: []
`
	if err := os.WriteFile(filepath.Join(jobDir, "job.yaml"), []byte(jobYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "run.sh"), []byte("#!/bin/bash\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	jobs, err := job.Discover(ctx, filepath.Join(tmpRoot, "jobs"))
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	cronDir := filepath.Join(tmpRoot, "cron.d")
	targetDir := filepath.Join(tmpRoot, "deployed")
	cronFile := filepath.Join(cronDir, "cronctl-no-schedule")

	// Create a cron file that should be removed
	if err := os.MkdirAll(cronDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cronFile, []byte("# old cron\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := syncer.Options{
		CronDir:   cronDir,
		TargetDir: targetDir,
		DryRun:    false,
	}

	if err := syncer.Sync(ctx, jobs, opts); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// Verify cron file was removed (empty schedule = no cron file)
	if _, err := os.Stat(cronFile); !os.IsNotExist(err) {
		t.Errorf("cron file should be removed when schedule is empty")
	}

	// Verify payload was still deployed
	deployedScript := filepath.Join(targetDir, "no-schedule", "run.sh")
	if _, err := os.Stat(deployedScript); err != nil {
		t.Errorf("payload should be deployed even with empty schedule: %v", err)
	}
}
