package syncer

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/yegor-usoltsev/cronctl/internal/build"
	"github.com/yegor-usoltsev/cronctl/internal/job"
)

func runBuildIfNeeded(ctx context.Context, dryRun bool, jobID, jobDir, entrypoint string, force bool, runAsUser bool, uid, gid int) error {
	if entrypoint == "" {
		entrypoint = job.DefaultBuildEntrypoint
	}
	if dryRun {
		log.Printf("dry-run: build: %s: run %s", jobID, entrypoint)
		return nil
	}
	statePath := filepath.Join(jobDir, ".cronctl", "filehash")

	curHash, err := build.InputsHash(ctx, jobDir)
	if err != nil {
		return fmt.Errorf("hash inputs: %w", err)
	}
	prev, ok := build.ReadHash(statePath)
	if !force && ok && prev == curHash {
		log.Printf("build: %s: skipped (cache)", jobID)
		return nil
	}

	cmdPath := filepath.Join(jobDir, entrypoint)
	cmd := exec.CommandContext(ctx, cmdPath)
	cmd.Dir = jobDir
	if runAsUser {
		if os.Geteuid() != 0 {
			return errBuildNeedsRoot
		}
		uid32, err := toUint32(uid)
		if err != nil {
			return fmt.Errorf("uid: %w", err)
		}
		gid32, err := toUint32(gid)
		if err != nil {
			return fmt.Errorf("gid: %w", err)
		}
		cred := &syscall.Credential{Uid: uid32, Gid: gid32}      //nolint:exhaustruct
		cmd.SysProcAttr = &syscall.SysProcAttr{Credential: cred} //nolint:exhaustruct
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("run build: %s: %w: %s", cmdPath, err, msg)
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return fmt.Errorf("run build: %s: %w (exit %d)", cmdPath, err, ee.ExitCode())
		}
		return fmt.Errorf("run build: %s: %w", cmdPath, err)
	}

	newHash, err := build.InputsHash(ctx, jobDir)
	if err != nil {
		return fmt.Errorf("hash inputs after build: %w", err)
	}
	if err := build.WriteHash(statePath, newHash); err != nil {
		return fmt.Errorf("write filehash: %w", err)
	}
	if runAsUser {
		_ = os.Chown(filepath.Dir(statePath), uid, gid)
		_ = os.Chown(statePath, uid, gid)
	}
	log.Printf("build: %s: ok", jobID)
	return nil
}

func toUint32(v int) (uint32, error) {
	if v < 0 {
		return 0, errNegativeID
	}
	if v > math.MaxUint32 {
		return 0, errIDTooLarge
	}
	return uint32(v), nil
}
