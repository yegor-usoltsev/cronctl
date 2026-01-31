# cronctl

[![Build Status](https://github.com/yegor-usoltsev/cronctl/actions/workflows/ci.yml/badge.svg)](https://github.com/yegor-usoltsev/cronctl/actions)
[![Codecov](https://codecov.io/github/yegor-usoltsev/cronctl/graph/badge.svg?token=Z1GET86OND)](https://codecov.io/github/yegor-usoltsev/cronctl)
[![GitHub Release](https://img.shields.io/github/v/release/yegor-usoltsev/cronctl?sort=semver)](https://github.com/yegor-usoltsev/cronctl/releases)

GitOps-style management of Linux cron jobs from a git repository.

**Key features:**

- Define cron jobs as YAML in a git repo
- Build step with smart caching (skip rebuild if inputs unchanged)
- Deploy to `/opt/cronctl/jobs/<id>` and manage `/etc/cron.d/` entries
- Tag-based filtering (like Ansible) for managing subsets of jobs
- Dry-run support for safe testing
- Versioned JSON Schema for IDE autocomplete

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Job Specification](#job-specification)
- [Commands](#commands)
- [Filtering with Tags](#filtering-with-tags)
- [Build Cache](#build-cache)
- [Safety Notes](#safety-notes)
- [Schema](#schema)

## Installation

Download the latest release from [GitHub Releases](https://github.com/yegor-usoltsev/cronctl/releases):

```bash
# Example for Linux amd64
curl -L https://github.com/yegor-usoltsev/cronctl/releases/latest/download/cronctl_linux_amd64 -o cronctl
chmod +x cronctl
sudo mv cronctl /usr/local/bin/
```

Or build from source:

```bash
git clone https://github.com/yegor-usoltsev/cronctl.git
cd cronctl
go build -o cronctl .
```

## Quick Start

### 1. Create a jobs repository

```bash
mkdir my-cron-jobs && cd my-cron-jobs
git init
```

### 2. Initialize a job

```bash
cronctl init backup-db
```

This creates:

```
jobs/backup-db/
├── job.yaml      # Job specification
├── build.sh      # Build script (optional)
├── run.sh        # Main script
└── .gitignore    # Ignores .cronctl/ cache
```

### 3. Edit the job specification

Edit `jobs/backup-db/job.yaml`:

```yaml
---
$schema: https://cronctl.usoltsev.xyz/v0.json

name: backup-db
enabled: true
user: postgres
tags:
  - production
  - db

env:
  PATH: /usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
  MAILTO: admin@example.com

build:
  enabled: false

run:
  entrypoint: run.sh

schedule:
  - cron: "0 2 * * *" # Run at 2 AM daily
    args:
      - --database
      - myapp
    env:
      BACKUP_DIR: /var/backups/db
    silent: false
```

### 4. Implement the script

Edit `jobs/backup-db/run.sh`:

```bash
#!/bin/bash
set -euo pipefail

DB=$1
pg_dump "$DB" > "$BACKUP_DIR/$(date +%Y%m%d).sql"
echo "Backup completed: $BACKUP_DIR/$(date +%Y%m%d).sql"
```

### 5. Validate locally

```bash
cronctl validate
```

### 6. Deploy to server

On your server (requires root):

```bash
# Clone/pull your jobs repo
git clone https://github.com/you/my-cron-jobs.git
cd my-cron-jobs

# Sync jobs to system
sudo cronctl sync

# Or dry-run first to see what would happen
sudo cronctl sync --dry-run
```

This will:

- Deploy job payload to `/opt/cronctl/jobs/backup-db/`
- Create `/etc/cron.d/cronctl-backup-db` with cron entries
- Run as the specified user (`postgres`)

## Job Specification

A job is defined by a `job.yaml` file in `jobs/<job-id>/`:

```yaml
---
$schema: https://cronctl.usoltsev.xyz/v0.json # Required

name: my-job # Required; must match directory name
enabled: true # Required
user: root # Required; user to run cron jobs as
tags: [prod, server-a] # Required (may be empty)

env: # Optional; global env vars for /etc/cron.d file
  PATH: /usr/local/bin:/usr/bin
  MAILTO: admin@example.com

build:
  enabled: true # Required
  entrypoint: build.sh # Optional (default: build.sh)

run:
  entrypoint: run.sh # Optional (default: run.sh)

schedule:
  - cron: "0 */6 * * *" # 5-field cron expression
    args: [--days, "7"] # Optional command arguments
    env: # Optional per-schedule env vars
      MODE: daily
    silent: false # Optional; redirect to /dev/null
```

### Field Reference

**Top-level:**

- `$schema` (required): Schema URL for IDE hints
- `name` (required): Job identifier (kebab-case: `[a-z0-9][a-z0-9-]*`)
- `enabled` (required): Enable/disable the job
- `user` (required): System user to run cron jobs as
- `tags` (required): Tags for filtering (can be empty array)
- `env` (optional): Global environment variables written to cron file header

**build:**

- `enabled` (required): Whether to run build step
- `entrypoint` (optional): Build script name (default: `build.sh`)

**run:**

- `entrypoint` (optional): Main script to execute (default: `run.sh`)

**schedule:** (array)

- `cron` (required): Standard 5-field cron expression
- `args` (optional): Arguments passed to entrypoint
- `env` (optional): Environment variables for this schedule entry
- `silent` (optional): If `true`, appends `>/dev/null 2>&1` to suppress output

## Commands

### `cronctl init <job-id>`

Create a new job with template files.

```bash
cronctl init my-job
```

### `cronctl validate [job-id] [flags]`

Validate job specifications.

```bash
# Validate all jobs
cronctl validate

# Validate specific job
cronctl validate my-job

# Validate jobs with specific tags
cronctl validate --tags prod,db

# Skip jobs with certain tags
cronctl validate --skip-tags dev
```

**Checks:**

- YAML structure (JSON Schema validation)
- Job ID format (kebab-case)
- Name matches directory name
- Required files exist
- Cron expression syntax (5 fields)

### `cronctl build [job-id] [flags]`

Run build steps for jobs (with caching).

```bash
# Build all enabled jobs
cronctl build

# Build specific job
cronctl build my-job

# Force rebuild (ignore cache)
cronctl build --force

# Build with parallelism
cronctl build --parallel 4

# Build only tagged jobs
cronctl build --tags prod
```

**Build cache:**

- Stores hash in `jobs/<id>/.cronctl/filehash`
- Skips rebuild if inputs unchanged
- Respects `.gitignore` files

### `cronctl sync [job-id] [flags]`

Deploy jobs and manage cron entries. **Requires root.**

```bash
# Sync all jobs
sudo cronctl sync

# Dry-run to see what would change
sudo cronctl sync --dry-run

# Sync specific job
sudo cronctl sync my-job

# Sync with tag filtering
sudo cronctl sync --tags prod

# Remove orphaned cron files
sudo cronctl sync --remove-orphans

# Force rebuild during sync
sudo cronctl sync --force-build
```

**What sync does:**

1. Creates target directory (default: `/opt/cronctl/jobs`)
2. For each enabled job:
   - Runs build if needed (with caching)
   - Copies job directory to `/opt/cronctl/jobs/<id>/`
   - Changes ownership to job's user
   - Generates `/etc/cron.d/cronctl-<id>`
3. For disabled jobs:
   - Removes `/etc/cron.d/cronctl-<id>`
   - Optionally removes payload (`--remove-payload-on-disable`)
4. Optionally prunes orphaned cron files (`--remove-orphans`)

**Flags:**

- `--dry-run`: Show what would happen without making changes
- `--cron-dir <path>`: Cron directory (default: `/etc/cron.d`)
- `--target-dir <path>`: Deployment directory (default: `/opt/cronctl/jobs`)
- `--remove-orphans`: Remove `cronctl-*` files not in current selection
- `--remove-payload-on-disable`: Delete payload dir when job disabled
- `--force-build`: Rebuild regardless of cache
- `--tags <tags>`: Only sync jobs with these tags
- `--skip-tags <tags>`: Skip jobs with these tags

## Filtering with Tags

Tags allow managing subsets of jobs (inspired by Ansible).

**Use cases:**

- One repo, multiple servers with different job subsets
- Environment-based deployment (prod, staging, dev)
- Group jobs by function (backup, monitoring, cleanup)

**Syntax:**

```bash
# Include jobs with ANY of these tags
cronctl sync --tags prod,db

# Exclude jobs with ANY of these tags
cronctl sync --skip-tags dev,staging

# Combine: include prod, but skip db
cronctl sync --tags prod --skip-tags db
```

**Example:**

```yaml
# jobs/backup-postgres/job.yaml
tags: [prod, db, backup]

# jobs/cleanup-logs/job.yaml
tags: [prod, maintenance]

# jobs/test-alerts/job.yaml
tags: [dev, testing]
```

```bash
# Deploy only production jobs
sudo cronctl sync --tags prod

# Deploy everything except dev/testing
sudo cronctl sync --skip-tags dev,testing
```

## Build Cache

Build cache prevents unnecessary rebuilds when inputs haven't changed.

**How it works:**

1. **Hash inputs:** All files in `jobs/<id>/` (respecting `.gitignore`)
2. **Compare:** Check if hash matches cached value in `.cronctl/filehash`
3. **Skip or build:** If matched, skip. If different, run build and update hash.

**Location:**

- Cache: `jobs/<id>/.cronctl/filehash`
- Recommended `.gitignore`: `.cronctl/`

**Hashing behavior:**

- **In git repo:** Uses `git ls-files` (respects all `.gitignore` from root)
- **Outside git:** Walks directory with job-local `.gitignore`
- Includes: file paths, file modes (executable bit), file contents

**Force rebuild:**

```bash
cronctl build --force
sudo cronctl sync --force-build
```

## Safety Notes

### Cron File Management

- **Only touches files matching:** `/etc/cron.d/cronctl-*`
- **Never modifies:** Other cron files, user crontabs, system crontab
- **Atomic writes:** Uses temp file + rename for cron file updates
- **Permissions:** Cron files are `0644` owned by `root:root`

### User Permissions

- **`sync` requires root** to:
  - Write to `/etc/cron.d/`
  - Change ownership of deployed payloads
  - Run builds as job user (optional)
- **Other commands** (`init`, `validate`, `build`) run as regular user

### Job User

- Jobs run as the user specified in `job.yaml`
- Deployed payload is owned by that user
- Build step can run as job user (requires root)

### Secrets

- **Never commit secrets** to job YAML
- Use external secret management (env vars, files, Vault, etc.)
- The `env` field is for **non-secret** configuration only

## Schema

Job specification uses a versioned JSON Schema for IDE autocomplete and validation:

**Schema URL:** `https://cronctl.usoltsev.xyz/v0.json`

**VSCode setup** (in `jobs/*/job.yaml`):

```yaml
$schema: https://cronctl.usoltsev.xyz/v0.json
```

This enables:

- Autocomplete for fields
- Inline validation
- Documentation on hover

## Examples

### Simple Scheduled Script

```yaml
---
$schema: https://cronctl.usoltsev.xyz/v0.json
name: hello-world
enabled: true
user: nobody
tags: []
env:
  PATH: /usr/bin:/bin
build:
  enabled: false
run:
  entrypoint: hello.sh
schedule:
  - cron: "*/5 * * * *" # Every 5 minutes
    args: []
    env: {}
    silent: true # Don't email output
```

### Job with Build Step

```yaml
---
$schema: https://cronctl.usoltsev.xyz/v0.json
name: go-scraper
enabled: true
user: scraper
tags: [prod, scraper]
env:
  PATH: /usr/local/go/bin:/usr/bin:/bin
build:
  enabled: true
  entrypoint: build.sh
run:
  entrypoint: scraper
schedule:
  - cron: "0 */2 * * *" # Every 2 hours
    args: [--target, "https://example.com"]
    env:
      LOG_LEVEL: info
```

**build.sh:**

```bash
#!/bin/bash
set -euo pipefail
go build -o scraper .
```

### Multiple Schedules

```yaml
---
$schema: https://cronctl.usoltsev.xyz/v0.json
name: multi-schedule
enabled: true
user: app
tags: [prod]
env:
  PATH: /usr/local/bin:/usr/bin
  MAILTO: ops@example.com
build:
  enabled: false
run:
  entrypoint: task.sh
schedule:
  - cron: "*/10 * * * *" # Every 10 min: quick check
    args: [check]
    env: {}
    silent: true

  - cron: "0 2 * * *" # Daily at 2 AM: full sync
    args: [sync, --full]
    env:
      MODE: full
    silent: false # Email results

  - cron: "0 0 * * 0" # Weekly on Sunday: cleanup
    args: [cleanup]
    env: {}
    silent: false
```

## Versioning

This project uses [Semantic Versioning](https://semver.org).

## Contributing

Pull requests are welcome. For major changes, please [open an issue](https://github.com/yegor-usoltsev/cronctl/issues/new) first to discuss what you would like to change.

Please make sure to update tests as appropriate.

## License

[MIT](https://github.com/yegor-usoltsev/cronctl/blob/main/LICENSE)
