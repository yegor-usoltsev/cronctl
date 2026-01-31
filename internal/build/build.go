package build

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"sync"

	"github.com/yegor-usoltsev/cronctl/internal/job"
)

type Options struct {
	Force    bool
	Parallel int
}

func All(ctx context.Context, _ string, jobs []job.Job, opts Options) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("build jobs: %w", err)
	}
	if len(jobs) == 0 {
		return nil
	}
	if opts.Parallel < 1 {
		opts.Parallel = 1
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	workCh := make(chan job.Job)
	errCh := make(chan error, 1)

	worker := func() {
		defer wg.Done()
		for j := range workCh {
			if err := ctx.Err(); err != nil {
				return
			}
			if err := one(ctx, j, opts.Force); err != nil {
				select {
				case errCh <- err:
				default:
				}
				cancel()
				return
			}
		}
	}
	workers := min(opts.Parallel, len(jobs))
	wg.Add(workers)
	for range workers {
		go worker()
	}

	for _, j := range jobs {
		if err := ctx.Err(); err != nil {
			break
		}
		workCh <- j
	}
	close(workCh)

	wg.Wait()

	select {
	case err := <-errCh:
		return err
	default:
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("build jobs: %w", err)
		}
		return nil
	}
}

func one(ctx context.Context, j job.Job, force bool) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("job %s: %w", j.ID, err)
	}
	if !j.Spec.Enabled {
		return nil
	}
	if !j.Spec.Build.Enabled {
		return nil
	}
	entrypoint := filepath.Clean(j.Spec.Build.Entrypoint)
	if entrypoint == "." || entrypoint == "" {
		entrypoint = job.DefaultBuildEntrypoint
	}

	statePath := StateFilePath(j.Dir)
	curHash, err := InputsHash(ctx, j.Dir)
	if err != nil {
		return fmt.Errorf("job %s: hash inputs: %w", j.ID, err)
	}

	prevHash, ok := ReadHash(statePath)
	if !force && ok && prevHash == curHash {
		log.Printf("build: %s: skipped (cache)", j.ID)
		return nil
	}

	log.Printf("build: %s: running %s", j.ID, entrypoint)
	if err := runBuild(ctx, j.Dir, entrypoint); err != nil {
		var ee *execError
		if errors.As(err, &ee) {
			return fmt.Errorf("job %s: build failed (%s): %w", j.ID, ee.Path, err)
		}
		return fmt.Errorf("job %s: build failed: %w", j.ID, err)
	}

	// Recompute after build so cache reflects in-place changes (esp. non-git mode).
	newHash, err := InputsHash(ctx, j.Dir)
	if err != nil {
		return fmt.Errorf("job %s: hash inputs after build: %w", j.ID, err)
	}
	if err := WriteHash(statePath, newHash); err != nil {
		return fmt.Errorf("job %s: write state: %w", j.ID, err)
	}
	log.Printf("build: %s: ok", j.ID)
	return nil
}
