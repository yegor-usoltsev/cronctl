package validate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/yegor-usoltsev/cronctl/internal/job"
	"github.com/yegor-usoltsev/cronctl/internal/schema"

	"gopkg.in/yaml.v3"
)

type Error struct {
	JobID string
	Path  string
	Msg   string
}

func (e Error) Error() string {
	return fmt.Sprintf("%s (%s): %s", e.Path, e.JobID, e.Msg)
}

type Errors []Error

func (e Errors) Error() string {
	switch len(e) {
	case 0:
		return "validation failed"
	case 1:
		return e[0].Error()
	default:
		return fmt.Sprintf("%s (and %d more)", e[0].Error(), len(e)-1)
	}
}

var jobIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func All(ctx context.Context, jobs []job.Job) error {
	var errs Errors

	schemaV0, err := schema.V0()
	if err != nil {
		return fmt.Errorf("load schema: %w", err)
	}

	for _, j := range jobs {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("validate: %w", err)
		}
		errs = append(errs, Job(ctx, schemaV0, j)...)
	}

	if len(errs) == 0 {
		return nil
	}
	sort.Slice(errs, func(i, j int) bool {
		if errs[i].Path != errs[j].Path {
			return errs[i].Path < errs[j].Path
		}
		if errs[i].JobID != errs[j].JobID {
			return errs[i].JobID < errs[j].JobID
		}
		return errs[i].Msg < errs[j].Msg
	})
	return errs
}

func Job(ctx context.Context, schemaV0 *jsonschema.Schema, j job.Job) []Error {
	if err := ctx.Err(); err != nil {
		return []Error{{JobID: j.ID, Path: j.YAML, Msg: err.Error()}}
	}

	errPath := j.YAML
	var errs []Error

	if !jobIDRe.MatchString(j.ID) {
		errs = append(errs, Error{JobID: j.ID, Path: errPath, Msg: "invalid job id, expected kebab-case ([a-z0-9][a-z0-9-]*)"})
	}

	if len(bytesTrimSpace(j.RawYAML)) == 0 {
		errs = append(errs, Error{JobID: j.ID, Path: errPath, Msg: "empty job.yaml"})
		return errs
	}

	if err := validateSchema(schemaV0, j.RawYAML); err != nil {
		errs = append(errs, Error{JobID: j.ID, Path: errPath, Msg: "JSON schema validation failed: " + err.Error()})
	}

	spec, err := decodeSpec(j.RawYAML)
	if err != nil {
		errs = append(errs, Error{JobID: j.ID, Path: errPath, Msg: "parse yaml: " + err.Error()})
		return errs
	}
	j.Spec = spec

	if j.Spec.Name != "" && j.Spec.Name != j.ID {
		errs = append(errs, Error{JobID: j.ID, Path: errPath, Msg: fmt.Sprintf("name must match job id: %q != %q", j.Spec.Name, j.ID)})
	}

	for i, s := range j.Spec.Schedule {
		if len(strings.Fields(s.Cron)) != 5 {
			errs = append(errs, Error{JobID: j.ID, Path: errPath, Msg: fmt.Sprintf("schedule[%d].cron must have 5 fields", i)})
		}
	}

	if strings.TrimSpace(j.Spec.Schema) != "" && j.Spec.Schema != schema.V0URL {
		errs = append(errs, Error{JobID: j.ID, Path: errPath, Msg: fmt.Sprintf("$schema must be %q", schema.V0URL)})
	}
	if strings.TrimSpace(j.Spec.Schema) == "" {
		errs = append(errs, Error{JobID: j.ID, Path: errPath, Msg: fmt.Sprintf("$schema is required and must be %q", schema.V0URL)})
	}

	ep := strings.TrimSpace(j.Spec.Run.Entrypoint)
	if ep == "" {
		ep = job.DefaultRunEntrypoint
	}
	p := filepath.Join(j.Dir, ep)
	if _, err := os.Stat(p); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, Error{JobID: j.ID, Path: errPath, Msg: "run.entrypoint file does not exist: " + p})
		} else {
			errs = append(errs, Error{JobID: j.ID, Path: errPath, Msg: fmt.Sprintf("stat run.entrypoint: %s: %v", p, err)})
		}
	}

	if !j.Spec.Build.Enabled {
		return errs
	}
	ep = strings.TrimSpace(j.Spec.Build.Entrypoint)
	if ep == "" {
		ep = job.DefaultBuildEntrypoint
	}
	p = filepath.Join(j.Dir, ep)
	if _, err := os.Stat(p); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, Error{JobID: j.ID, Path: errPath, Msg: "build.entrypoint file does not exist: " + p})
		} else {
			errs = append(errs, Error{JobID: j.ID, Path: errPath, Msg: fmt.Sprintf("stat build.entrypoint: %s: %v", p, err)})
		}
	}

	return errs
}

func bytesTrimSpace(b []byte) []byte {
	return bytes.TrimSpace(b)
}

func validateSchema(schemaV0 *jsonschema.Schema, yamlBytes []byte) error {
	var yamlDoc any
	if err := yaml.Unmarshal(yamlBytes, &yamlDoc); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}
	jsonBytes, err := json.Marshal(yamlDoc)
	if err != nil {
		return fmt.Errorf("convert yaml to json: %w", err)
	}
	var jsonDoc any
	if err := json.Unmarshal(jsonBytes, &jsonDoc); err != nil {
		return fmt.Errorf("parse json: %w", err)
	}
	if err := schemaV0.Validate(jsonDoc); err != nil {
		return schemaError{msg: formatSchemaErr(err)}
	}
	return nil
}

type schemaError struct{ msg string }

func (e schemaError) Error() string { return e.msg }

func decodeSpec(b []byte) (job.Spec, error) {
	var spec job.Spec
	if err := yaml.Unmarshal(b, &spec); err != nil {
		return job.Spec{}, fmt.Errorf("yaml unmarshal: %w", err)
	}
	return spec, nil
}

func formatSchemaErr(err error) string {
	var ve *jsonschema.ValidationError
	if errors.As(err, &ve) {
		b, mErr := json.Marshal(ve.BasicOutput())
		if mErr != nil {
			return "schema: " + err.Error()
		}
		return "schema: " + string(b)
	}
	return "schema: " + err.Error()
}
