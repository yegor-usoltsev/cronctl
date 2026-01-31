package scaffold

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/yegor-usoltsev/cronctl/internal/schema"
)

//go:embed templates/*.tmpl
var templatesFS embed.FS

var jobIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

var (
	errInvalidJobID = errors.New("invalid job id, expected kebab-case ([a-z0-9][a-z0-9-]*)")
	errJobExists    = errors.New("job dir already exists")
)

type InitOptions struct {
	RootDir string
	JobsDir string
}

type templateData struct {
	JobID     string
	SchemaURL string
}

func InitJob(ctx context.Context, jobID string, opts InitOptions) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("init: %w", err)
	}
	if !jobIDRe.MatchString(jobID) {
		return errInvalidJobID
	}
	if strings.TrimSpace(opts.JobsDir) == "" {
		opts.JobsDir = "jobs"
	}
	root := strings.TrimSpace(opts.RootDir)
	if root == "" {
		root = "."
	}

	jobsDir := opts.JobsDir
	if !filepath.IsAbs(jobsDir) {
		jobsDir = filepath.Join(root, jobsDir)
	}
	jobDir := filepath.Join(jobsDir, jobID)
	if _, err := os.Stat(jobDir); err == nil {
		return fmt.Errorf("%w: %s", errJobExists, jobDir)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("stat job dir: %s: %w", jobDir, err)
	}

	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		return fmt.Errorf("mkdir job dir: %s: %w", jobDir, err)
	}

	data := templateData{JobID: jobID, SchemaURL: schema.V0URL}

	if err := renderToFile(filepath.Join(jobDir, "run.sh"), 0o755, "run.sh.tmpl", data); err != nil {
		return err
	}
	if err := renderToFile(filepath.Join(jobDir, "build.sh"), 0o755, "build.sh.tmpl", data); err != nil {
		return err
	}
	if err := renderToFile(filepath.Join(jobDir, ".gitignore"), 0o644, ".gitignore.tmpl", data); err != nil {
		return err
	}
	if err := renderToFile(filepath.Join(jobDir, "job.yaml"), 0o644, "job.yaml.tmpl", data); err != nil {
		return err
	}
	return nil
}

func renderToFile(path string, perm fs.FileMode, name string, data templateData) error {
	b, err := renderTemplate(name, data)
	if err != nil {
		return err
	}
	return writeExclusive(path, perm, b)
}

func renderTemplate(name string, data templateData) ([]byte, error) {
	p := filepath.Join("templates", name)
	t, err := template.New(name).Option("missingkey=error").ParseFS(templatesFS, p)
	if err != nil {
		return nil, fmt.Errorf("parse template: %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("render template: %s: %w", name, err)
	}
	return buf.Bytes(), nil
}

func writeExclusive(path string, perm fs.FileMode, content []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return fmt.Errorf("create file: %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Write(content); err != nil {
		return fmt.Errorf("write file: %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close file: %s: %w", path, err)
	}
	return nil
}
