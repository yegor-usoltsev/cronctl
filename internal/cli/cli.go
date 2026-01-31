package cli

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/alecthomas/kong"
	"github.com/yegor-usoltsev/cronctl/internal/build"
	"github.com/yegor-usoltsev/cronctl/internal/job"
	"github.com/yegor-usoltsev/cronctl/internal/scaffold"
	"github.com/yegor-usoltsev/cronctl/internal/syncer"
	"github.com/yegor-usoltsev/cronctl/internal/validate"

	"gopkg.in/yaml.v3"
)

type root struct {
	Init     initCmd     `cmd:"" help:"Create a new job scaffold."`
	Validate validateCmd `cmd:"" help:"Validate job specs."`
	Build    buildCmd    `cmd:"" help:"Run job build steps with caching."`
	Sync     syncCmd     `cmd:"" help:"Deploy jobs and manage /etc/cron.d entries."`
}

type initCmd struct {
	JobsDir string `name:"jobs-dir" default:"jobs" help:"Jobs directory."`
	JobID   string `arg:"" name:"job-id" help:"Job ID (kebab-case)."`
}

func (c *initCmd) Run(ctx context.Context) error {
	if err := scaffold.InitJob(ctx, c.JobID, scaffold.InitOptions{RootDir: ".", JobsDir: c.JobsDir}); err != nil {
		return fmt.Errorf("init job: %w", err)
	}
	log.Printf("job initialized: %s", c.JobID)
	return nil
}

type validateCmd struct {
	JobsDir  string   `name:"jobs-dir" default:"jobs" help:"Jobs directory."`
	Tags     []string `name:"tags" sep:"," help:"Include jobs that have ANY of these tags."`
	SkipTags []string `name:"skip-tags" sep:"," help:"Exclude jobs that have ANY of these tags."`
	JobID    string   `arg:"" optional:"" name:"job-id" help:"Validate only this job ID."`
}

func (c *validateCmd) Run(ctx context.Context) error {
	jobs, err := job.DiscoverRaw(ctx, c.JobsDir)
	if err != nil {
		return fmt.Errorf("discover jobs: %w", err)
	}
	if c.JobID != "" {
		jobs = onlyJob(jobs, c.JobID)
		if len(jobs) == 0 {
			return fmt.Errorf("%w: %s", errJobNotFound, c.JobID)
		}
	}
	if len(c.Tags) > 0 || len(c.SkipTags) > 0 {
		jobs = filterJobsByTags(jobs, c.Tags, c.SkipTags)
	}
	if err := validate.All(ctx, jobs); err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	return nil
}

type buildCmd struct {
	JobsDir  string   `name:"jobs-dir" default:"jobs" help:"Jobs directory."`
	Tags     []string `name:"tags" sep:"," help:"Include jobs that have ANY of these tags."`
	SkipTags []string `name:"skip-tags" sep:"," help:"Exclude jobs that have ANY of these tags."`
	Force    bool     `name:"force" help:"Rebuild regardless of cache."`
	Parallel int      `name:"parallel" default:"1" help:"Max parallel builds."`
	JobID    string   `arg:"" optional:"" name:"job-id" help:"Build only this job ID."`
}

type syncCmd struct {
	JobsDir                string   `name:"jobs-dir" default:"jobs" help:"Jobs directory."`
	Tags                   []string `name:"tags" sep:"," help:"Include jobs that have ANY of these tags."`
	SkipTags               []string `name:"skip-tags" sep:"," help:"Exclude jobs that have ANY of these tags."`
	DryRun                 bool     `name:"dry-run" help:"Print actions without making changes."`
	CronDir                string   `name:"cron-dir" default:"/etc/cron.d" help:"Cron directory to write cronctl-* files."`
	TargetDir              string   `name:"target-dir" default:"/opt/cronctl/jobs" help:"Target directory for deployed job payloads."`
	RemoveOrphans          bool     `name:"remove-orphans" help:"Remove cronctl-managed cron files not present in selection."`
	RemovePayloadOnDisable bool     `name:"remove-payload-on-disable" help:"Remove payload dir when a job is disabled."`
	ForceBuild             bool     `name:"force-build" help:"Force rebuild regardless of cache."`
	JobID                  string   `arg:"" optional:"" name:"job-id" help:"Sync only this job ID."`
}

func (c *syncCmd) Run(ctx context.Context) error {
	if os.Geteuid() != 0 {
		return errSyncNeedsRoot
	}
	jobs, err := job.Discover(ctx, c.JobsDir)
	if err != nil {
		return fmt.Errorf("discover jobs: %w", err)
	}
	if c.JobID != "" {
		jobs = onlyJob(jobs, c.JobID)
		if len(jobs) == 0 {
			return fmt.Errorf("%w: %s", errJobNotFound, c.JobID)
		}
	}
	if len(c.Tags) > 0 || len(c.SkipTags) > 0 {
		jobs = filterParsedJobsByTags(jobs, c.Tags, c.SkipTags)
	}
	if err := syncJobs(ctx, jobs, c.DryRun, c.CronDir, c.TargetDir, c.RemoveOrphans, c.RemovePayloadOnDisable, c.ForceBuild); err != nil {
		return fmt.Errorf("sync: %w", err)
	}
	return nil
}

func (c *buildCmd) Run(ctx context.Context) error {
	jobs, err := job.Discover(ctx, c.JobsDir)
	if err != nil {
		return fmt.Errorf("discover jobs: %w", err)
	}
	if c.JobID != "" {
		jobs = onlyJob(jobs, c.JobID)
		if len(jobs) == 0 {
			return fmt.Errorf("%w: %s", errJobNotFound, c.JobID)
		}
	}
	if len(c.Tags) > 0 || len(c.SkipTags) > 0 {
		jobs = filterParsedJobsByTags(jobs, c.Tags, c.SkipTags)
	}
	if err := buildJobs(ctx, c.JobsDir, jobs, c.Force, c.Parallel); err != nil {
		return fmt.Errorf("build: %w", err)
	}
	return nil
}

func Run(args []string) int {
	if len(args) == 0 {
		args = []string{"--help"}
	}

	var cli root
	k, err := kong.New(
		&cli,
		kong.Name("cronctl"),
		kong.Description("Manage Linux cron jobs from a git repository."),
		kong.UsageOnError(),
		kong.Writers(os.Stdout, os.Stderr),
	)
	if err != nil {
		log.Printf("init cli: %v", err)
		return 1
	}

	kctx, err := k.Parse(args)
	if err != nil {
		return parseExitCode(err)
	}

	if err := kctx.Run(context.Background()); err != nil {
		var verrs validate.Errors
		if errors.As(err, &verrs) {
			for _, e := range verrs {
				log.Printf("validate: %s (%s): %s", e.Path, e.JobID, e.Msg)
			}
			return 1
		}
		log.Printf("command failed: %v", err)
		return 1
	}

	return 0
}

var errJobNotFound = errors.New("job not found")
var errSyncNeedsRoot = errors.New("sync must be run as root (try: sudo cronctl sync ...)")

func parseExitCode(err error) int {
	var ec interface{ ExitCode() int }
	if errors.As(err, &ec) {
		code := ec.ExitCode()
		if code == 0 {
			return 0
		}
		log.Printf("parse args: %v", err)
		return code
	}
	// If this isn't an ExitCoder error, treat it as a usage error.
	log.Printf("parse args: %v", err)
	return 2
}

func onlyJob(jobs []job.Job, id string) []job.Job {
	for _, j := range jobs {
		if j.ID == id {
			return []job.Job{j}
		}
	}
	return nil
}

func filterJobsByTags(jobs []job.Job, tags, skip []string) []job.Job {
	out := make([]job.Job, 0, len(jobs))
	for _, j := range jobs {
		spec, err := tryParseSpecForTags(j.RawYAML)
		if err != nil {
			// If YAML is invalid, keep the job so validate can report errors.
			out = append(out, j)
			continue
		}
		if matchTags(spec.Tags, tags, skip) {
			out = append(out, j)
		}
	}
	return out
}

func tryParseSpecForTags(raw []byte) (job.Spec, error) {
	var spec job.Spec
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		return job.Spec{}, fmt.Errorf("parse yaml: %w", err)
	}
	return spec, nil
}

func filterParsedJobsByTags(jobs []job.Job, tags, skip []string) []job.Job {
	out := make([]job.Job, 0, len(jobs))
	for _, j := range jobs {
		if matchTags(j.Spec.Tags, tags, skip) {
			out = append(out, j)
		}
	}
	return out
}

func matchTags(jobTags, include, skip []string) bool {
	if len(skip) > 0 && hasAny(jobTags, skip) {
		return false
	}
	if len(include) == 0 {
		return true
	}
	return hasAny(jobTags, include)
}

func hasAny(haystack, needles []string) bool {
	if len(haystack) == 0 || len(needles) == 0 {
		return false
	}
	m := make(map[string]struct{}, len(haystack))
	for _, t := range haystack {
		m[t] = struct{}{}
	}
	for _, n := range needles {
		if _, ok := m[n]; ok {
			return true
		}
	}
	return false
}

func buildJobs(ctx context.Context, jobsDir string, jobs []job.Job, force bool, parallel int) error {
	if err := build.All(ctx, jobsDir, jobs, build.Options{Force: force, Parallel: parallel}); err != nil {
		return fmt.Errorf("build jobs: %w", err)
	}
	return nil
}

func syncJobs(ctx context.Context, jobs []job.Job, dryRun bool, cronDir, targetDir string, removeOrphans, removePayloadOnDisable, forceBuild bool) error {
	opts := syncer.Options{
		CronDir:                cronDir,
		TargetDir:              targetDir,
		DryRun:                 dryRun,
		RemoveOrphans:          removeOrphans,
		RemovePayloadOnDisable: removePayloadOnDisable,
		ForceBuild:             forceBuild,
		Chown:                  true,
		RunBuildAsJobUser:      true,
	}
	if err := syncer.Sync(ctx, jobs, opts); err != nil {
		return fmt.Errorf("sync jobs: %w", err)
	}
	return nil
}
