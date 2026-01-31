package syncer

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/yegor-usoltsev/cronctl/internal/job"
)

type Options struct {
	CronDir                string
	TargetDir              string
	DryRun                 bool
	RemoveOrphans          bool
	RemovePayloadOnDisable bool
	ForceBuild             bool

	Chown             bool
	RunBuildAsJobUser bool
}

func Sync(ctx context.Context, jobs []job.Job, opts Options) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("sync: %w", err)
	}
	if opts.CronDir == "" {
		opts.CronDir = "/etc/cron.d"
	}
	if opts.TargetDir == "" {
		opts.TargetDir = "/opt/cronctl/jobs"
	}
	if len(jobs) == 0 {
		return nil
	}

	if err := os.MkdirAll(opts.TargetDir, 0o755); err != nil {
		return fmt.Errorf("mkdir target dir: %s: %w", opts.TargetDir, err)
	}

	// Build strategy:
	// - copy sources into a temp dir under TargetDir
	// - run build in that temp dir (so we don't dirty the repo checkout)
	// - atomically swap the temp dir into TargetDir/jobID

	seen := make(map[string]struct{}, len(jobs))
	for _, j := range jobs {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("sync: %w", err)
		}
		seen[j.ID] = struct{}{}

		cronPath := filepath.Join(opts.CronDir, "cronctl-"+j.ID)
		targetPath := filepath.Join(opts.TargetDir, j.ID)

		if !j.Spec.Enabled {
			if err := removeFileIfExists(opts.DryRun, cronPath); err != nil {
				return err
			}
			if opts.RemovePayloadOnDisable {
				if err := removeDirIfExists(opts.DryRun, targetPath); err != nil {
					return err
				}
			}
			continue
		}

		uid, gid, err := resolveJobUser(j.Spec.User)
		if err != nil {
			return fmt.Errorf("job %s: resolve user %q: %w", j.ID, j.Spec.User, err)
		}

		tmpDir, err := stageJobDir(opts.DryRun, opts.TargetDir, j.ID)
		if err != nil {
			return fmt.Errorf("job %s: stage: %w", j.ID, err)
		}
		deployed := false
		if !opts.DryRun {
			defer func() {
				if deployed {
					return
				}
				_ = os.RemoveAll(tmpDir)
			}()
		}
		if err := copyJobDir(opts.DryRun, j.Dir, tmpDir); err != nil {
			return fmt.Errorf("job %s: copy payload: %w", j.ID, err)
		}
		if err := carryOverFilehash(opts.DryRun, targetPath, tmpDir); err != nil {
			return fmt.Errorf("job %s: carry over cache: %w", j.ID, err)
		}
		if opts.Chown {
			if err := chownTree(opts.DryRun, tmpDir, uid, gid); err != nil {
				return fmt.Errorf("job %s: chown payload: %w", j.ID, err)
			}
		}

		if j.Spec.Build.Enabled {
			if err := runBuildIfNeeded(ctx, opts.DryRun, j.ID, tmpDir, j.Spec.Build.Entrypoint, opts.ForceBuild, opts.RunBuildAsJobUser, uid, gid); err != nil {
				return fmt.Errorf("job %s: build: %w", j.ID, err)
			}
		}

		if err := replaceDir(opts.DryRun, tmpDir, targetPath); err != nil {
			return fmt.Errorf("job %s: deploy: %w", j.ID, err)
		}
		deployed = true

		// Ensure payload ownership (includes build outputs).
		if opts.Chown {
			if err := chownTree(opts.DryRun, targetPath, uid, gid); err != nil {
				return fmt.Errorf("job %s: chown deployed payload: %w", j.ID, err)
			}
		}

		// If schedule is empty, desired state is no cron file.
		if len(j.Spec.Schedule) == 0 {
			if err := removeFileIfExists(opts.DryRun, cronPath); err != nil {
				return err
			}
			log.Printf("sync: %s: ok (no schedule)", j.ID)
			continue
		}

		if err := writeCronFile(opts.DryRun, cronPath, j, targetPath); err != nil {
			return fmt.Errorf("job %s: write cron: %w", j.ID, err)
		}

		log.Printf("sync: %s: ok", j.ID)
	}

	if opts.RemoveOrphans {
		if err := pruneOrphans(opts.DryRun, opts.CronDir, seen); err != nil {
			return err
		}
	}

	return nil
}
