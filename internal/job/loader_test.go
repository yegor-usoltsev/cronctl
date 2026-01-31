package job

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDiscover(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	jobsDir := filepath.Join(root, "jobs")
	if err := os.MkdirAll(jobsDir, 0o755); err != nil {
		t.Fatalf("mkdir jobs: %v", err)
	}

	// jobs/a/job.yaml
	jobA := filepath.Join(jobsDir, "a")
	if err := os.MkdirAll(jobA, 0o755); err != nil {
		t.Fatalf("mkdir job a: %v", err)
	}
	if err := os.WriteFile(filepath.Join(jobA, "job.yaml"), []byte("$schema: \"https://cronctl.usoltsev.xyz/v0.json\"\nenabled: true\nuser: root\ntags: []\nbuild: { enabled: false, entrypoint: build.sh }\nrun: { entrypoint: run.sh }\nschedule: [{ cron: \"0 * * * *\", args: [], env: {} }]\n"), 0o644); err != nil {
		t.Fatalf("write a job.yaml: %v", err)
	}

	// jobs/b/ (no job.yaml) => ignored
	if err := os.MkdirAll(filepath.Join(jobsDir, "b"), 0o755); err != nil {
		t.Fatalf("mkdir job b: %v", err)
	}

	// jobs/.hidden/job.yaml => ignored (hidden)
	hidden := filepath.Join(jobsDir, ".hidden")
	if err := os.MkdirAll(hidden, 0o755); err != nil {
		t.Fatalf("mkdir hidden: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hidden, "job.yaml"), []byte("$schema: \"https://cronctl.usoltsev.xyz/v0.json\"\nenabled: true\nuser: root\ntags: []\nbuild: { enabled: false, entrypoint: build.sh }\nrun: { entrypoint: run.sh }\nschedule: [{ cron: \"0 * * * *\", args: [], env: {} }]\n"), 0o644); err != nil {
		t.Fatalf("write hidden job.yaml: %v", err)
	}

	got, err := Discover(context.Background(), jobsDir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 job, got %d", len(got))
	}
	if got[0].ID != "a" {
		t.Fatalf("expected job id a, got %q", got[0].ID)
	}
	if got[0].Spec.User != "root" {
		t.Fatalf("expected user root, got %q", got[0].Spec.User)
	}
	if got[0].Spec.Run.Entrypoint != "run.sh" {
		t.Fatalf("expected run entrypoint run.sh, got %q", got[0].Spec.Run.Entrypoint)
	}
	if len(got[0].Spec.Schedule) != 1 {
		t.Fatalf("expected 1 schedule item, got %d", len(got[0].Spec.Schedule))
	}
	if got[0].Spec.Schedule[0].Cron != "0 * * * *" {
		t.Fatalf("expected cron value, got %q", got[0].Spec.Schedule[0].Cron)
	}
}

func TestDiscoverRaw(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	jobsDir := filepath.Join(root, "jobs")
	if err := os.MkdirAll(jobsDir, 0o755); err != nil {
		t.Fatalf("mkdir jobs: %v", err)
	}

	jobA := filepath.Join(jobsDir, "a")
	if err := os.MkdirAll(jobA, 0o755); err != nil {
		t.Fatalf("mkdir job a: %v", err)
	}
	if err := os.WriteFile(filepath.Join(jobA, "job.yaml"), []byte("not: [valid\n"), 0o644); err != nil {
		t.Fatalf("write a job.yaml: %v", err)
	}

	got, err := DiscoverRaw(context.Background(), jobsDir)
	if err != nil {
		t.Fatalf("DiscoverRaw: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 job, got %d", len(got))
	}
	if len(got[0].RawYAML) == 0 {
		t.Fatalf("expected raw yaml")
	}
	// Raw discovery should not require YAML parsing.
}
