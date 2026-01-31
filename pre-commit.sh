#!/usr/bin/env bash
set -euo pipefail

hook_path=".git/hooks/pre-commit"

if [[ "${1:-}" == "--init" ]]; then
	mkdir -p "$(dirname "$hook_path")"
	cat >"$hook_path" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

exec ./pre-commit.sh
EOF
	chmod +x "$hook_path"
	echo "installed $hook_path"
	exit 0
fi

echo "pre-commit: go build"
go build -race ./...

echo "pre-commit: go test"
go test -race ./...

echo "pre-commit: golangci-lint"
golangci-lint run
