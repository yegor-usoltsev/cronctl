package validate

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/yegor-usoltsev/cronctl/internal/job"
)

func mustSchema(t *testing.T, schemaJSON string) *jsonschema.Schema {
	t.Helper()
	c := jsonschema.NewCompiler()
	if err := c.AddResource("mem://schema", mustUnmarshalJSON(t, schemaJSON)); err != nil {
		t.Fatalf("AddResource: %v", err)
	}
	return c.MustCompile("mem://schema")
}

func mustUnmarshalJSON(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	return v
}

func TestValidateJob_OK(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	jobsDir := filepath.Join(root, "jobs")
	jobDir := filepath.Join(jobsDir, "ok-job")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "run.sh"), []byte("#!/usr/bin/env bash\nset -euo pipefail\necho ok\n"), 0o755); err != nil {
		t.Fatalf("write run.sh: %v", err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "build.sh"), []byte("#!/usr/bin/env bash\nset -euo pipefail\necho build\n"), 0o755); err != nil {
		t.Fatalf("write build.sh: %v", err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "job.yaml"), []byte("$schema: \"https://cronctl.usoltsev.xyz/v0.json\"\nname: ok-job\nenabled: true\nuser: root\ntags: []\nbuild: { enabled: false, entrypoint: build.sh }\nrun: { entrypoint: run.sh }\nschedule: [{ cron: \"0 * * * *\", args: [], env: {} }]\n"), 0o644); err != nil {
		t.Fatalf("write job.yaml: %v", err)
	}

	jobs, err := job.DiscoverRaw(context.Background(), jobsDir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	errs := Job(context.Background(), mustSchema(t, `{}`), jobs[0])
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %d: %v", len(errs), errs)
	}
}

func TestValidateJob_FailsForBadID(t *testing.T) {
	t.Parallel()

	j := job.Job{
		ID:      "Bad_ID",
		Dir:     t.TempDir(),
		YAML:    "jobs/Bad_ID/job.yaml",
		RawYAML: []byte("$schema: \"https://cronctl.usoltsev.xyz/v0.json\"\nenabled: true\nuser: root\ntags: []\nbuild: { enabled: false, entrypoint: build.sh }\nrun: { entrypoint: run.sh }\nschedule: [{ cron: \"0 * * * *\", args: [], env: {} }]\n"),
	}
	_ = os.WriteFile(filepath.Join(j.Dir, "run.sh"), []byte("#!/usr/bin/env bash\nset -euo pipefail\n"), 0o755)
	_ = os.WriteFile(filepath.Join(j.Dir, "build.sh"), []byte("#!/usr/bin/env bash\nset -euo pipefail\n"), 0o755)

	errs := Job(context.Background(), mustSchema(t, `{}`), j)
	if len(errs) == 0 {
		t.Fatalf("expected errors")
	}
}

func TestValidateJob_FailsForMissingEntrypointFile(t *testing.T) {
	t.Parallel()

	j := job.Job{
		ID:      "ok-job",
		Dir:     t.TempDir(),
		YAML:    "jobs/ok-job/job.yaml",
		RawYAML: []byte("$schema: \"https://cronctl.usoltsev.xyz/v0.json\"\nenabled: true\nuser: root\ntags: []\nbuild: { enabled: false, entrypoint: build.sh }\nrun: { entrypoint: run.sh }\nschedule: [{ cron: \"0 * * * *\", args: [], env: {} }]\n"),
	}

	errs := Job(context.Background(), mustSchema(t, `{}`), j)
	if len(errs) == 0 {
		t.Fatalf("expected errors")
	}
}
