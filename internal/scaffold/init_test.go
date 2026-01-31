package scaffold

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitJob_CreatesFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := InitJob(context.Background(), "my-job", InitOptions{RootDir: root, JobsDir: "jobs"}); err != nil {
		t.Fatalf("InitJob: %v", err)
	}

	jobDir := filepath.Join(root, "jobs", "my-job")
	assertFileModeExec(t, filepath.Join(jobDir, "run.sh"))
	assertFileModeExec(t, filepath.Join(jobDir, "build.sh"))

	gi, err := os.ReadFile(filepath.Join(jobDir, ".gitignore"))
	if err != nil {
		t.Fatalf("read job .gitignore: %v", err)
	}
	if !strings.Contains(string(gi), ".cronctl/") {
		t.Fatalf("job .gitignore missing .cronctl/")
	}

	jobYAMLPath := filepath.Join(jobDir, "job.yaml")
	b, err := os.ReadFile(jobYAMLPath)
	if err != nil {
		t.Fatalf("read job.yaml: %v", err)
	}
	if !strings.Contains(string(b), "name: my-job") {
		t.Fatalf("job.yaml missing name")
	}
	if !strings.Contains(string(b), "build:") {
		t.Fatalf("job.yaml missing build section")
	}

	// Root .gitignore is not modified by init.
}

func assertFileModeExec(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("expected executable file: %s", path)
	}
}
