package job

const (
	DefaultBuildEntrypoint = "build.sh"
	DefaultRunEntrypoint   = "run.sh"
)

// Job represents a job directory discovered under jobs/<job-id>/.
type Job struct {
	ID   string
	Dir  string
	YAML string

	RawYAML []byte
	Spec    Spec
}

func New(id, dir, yamlPath string, raw []byte) Job {
	// Spec fields get filled by YAML decode; initialize with zero-values.
	var zero Spec
	return Job{ID: id, Dir: dir, YAML: yamlPath, RawYAML: raw, Spec: zero}
}

// Spec is the on-disk job.yaml structure.
//
// Note: defaults from the published JSON schema are not automatically applied
// by cronctl; fields omitted in YAML will be their Go zero values.
type Spec struct {
	Schema  string            `yaml:"$schema,omitempty"`
	Name    string            `yaml:"name"`
	Enabled bool              `yaml:"enabled"`
	User    string            `yaml:"user"`
	Tags    []string          `yaml:"tags"`
	Env     map[string]string `yaml:"env,omitempty"`

	Build BuildSpec `yaml:"build"`
	Run   RunSpec   `yaml:"run"`

	Schedule []ScheduleItem `yaml:"schedule"`
}

type BuildSpec struct {
	Enabled    bool   `yaml:"enabled"`
	Entrypoint string `yaml:"entrypoint,omitempty"`
}

type RunSpec struct {
	Entrypoint string `yaml:"entrypoint,omitempty"`
}

type ScheduleItem struct {
	Cron   string            `yaml:"cron"`
	Args   []string          `yaml:"args,omitempty"`
	Env    map[string]string `yaml:"env,omitempty"`
	Silent bool              `yaml:"silent,omitempty"`
}
