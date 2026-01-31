# AGENTS.md

`cronctl` is a small Go CLI that manages Linux cron jobs from a git repository.

Core idea:

- Jobs live under `jobs/<job-id>/`.
- `cronctl sync` (run on the server) deploys job payloads to `/opt/cronctl/jobs/<job-id>` and writes cron entries to `/etc/cron.d/cronctl-<job-id>`.

## Development Notes

- Go version: see `go.mod` (`go 1.25.6`).
- Logging: use the standard `log` package (this is a CLI; structured logging isn't needed).
- CI runs: `go build -race ./...`, `go test -race ./...`, `golangci-lint`.

## Code Style Preferences

- Aim for idiomatic Go: small focused packages, clear names, minimal indirection.
- Prefer short/simple implementations over clever abstractions.
- Make error handling explicit and actionable (wrap with context; keep messages stable).
- Keep public surface area small; expose only what callers need.
- Avoid pointers in structs and APIs unless they are clearly necessary (prefer zero values and explicit booleans). This reduces nil handling and nil dereference risk.
- Tests: prefer table tests where it helps; avoid global mutable state in tests.

## Research Note

- When looking up library docs, prefer using Context7 MCP (fast, focused API/docs search).

Useful commands:

```bash
go test ./...
go test -race ./...
golangci-lint run
go run .
```

## Product Constraints (v1)

- `sync` is local-only (no SSH mode in cronctl).
- Job ID is the directory name and must be kebab-case: `[a-z0-9][a-z0-9-]*`.
- Build caching is per-job state under `jobs/<id>/.cronctl/filehash` (must be gitignored).
- Filtering: tags like Ansible (`--tags`, `--skip-tags`).
- Safety: only manage `/etc/cron.d/cronctl-*` files; write atomically.
