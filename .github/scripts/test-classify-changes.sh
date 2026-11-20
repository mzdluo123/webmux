#!/usr/bin/env bash
set -euo pipefail

root=$(git rev-parse --show-toplevel)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
repo="$tmp/repo"
mkdir -p "$repo"
cd "$repo"
git init -q
git config user.name "Classifier Test"
git config user.email "classifier@example.invalid"
git commit --allow-empty -qm base
base=$(git rev-parse HEAD)

check() {
  local name=$1 expected=$2
  shift 2
  local path
  for path in "$@"; do
    mkdir -p "$(dirname "$path")"
    printf '%s\n' "$name" >> "$path"
    git add "$path"
  done
  git commit -qm "$name"
  local head output
  head=$(git rev-parse HEAD)
  output="$tmp/output"
  : > "$output"
  BASE_SHA="$base" HEAD_SHA="$head" GITHUB_OUTPUT="$output" bash "$root/.github/scripts/classify-changes.sh" > /dev/null
  local actual
  actual=$(grep -E '^(go|web|docs_only|full)=' "$output" | tr '\n' ' ')
  if [[ "$actual" != "$expected" ]]; then
    printf 'classifier %s: got %s, want %s\n' "$name" "$actual" "$expected" >&2
    exit 1
  fi
  git reset -q --hard "$base"
  git clean -qfd
}

check docs-only 'go=false web=false docs_only=true full=false ' docs/usage.md
check pr-assets-only 'go=false web=false docs_only=true full=false ' .github/pr-assets/demo/after.png
check web-with-pr-assets 'go=false web=true docs_only=false full=false ' web/src/App.svelte .github/pr-assets/demo/after.png
check go-only 'go=true web=false docs_only=false full=false ' internal/session/manager.go
check go-with-docs 'go=true web=false docs_only=false full=false ' internal/session/manager.go docs/usage.md
check go-mod 'go=true web=false docs_only=false full=false ' go.mod
check go-lint-config 'go=true web=false docs_only=false full=false ' .golangci.yml
check protocol-relay 'go=true web=false docs_only=false full=false ' protocol/relay/access.go
check install-script 'go=true web=false docs_only=false full=false ' install.sh
check web-embed-go 'go=true web=false docs_only=false full=false ' web/embed.go
check web-only 'go=false web=true docs_only=false full=false ' web/src/platform/api.ts
check web-lockfile 'go=false web=true docs_only=false full=false ' web/pnpm-lock.yaml
check web-config 'go=false web=true docs_only=false full=false ' web/vitest.config.ts
check api-wire 'go=true web=true docs_only=false full=false ' internal/api/testdata/wire/example.json
check conversion-wire 'go=true web=true docs_only=false full=false ' internal/wire/testdata/response-arrays.json
check fakeagent 'go=true web=true docs_only=false full=false ' internal/acp/testdata/fakeagent/main.go
check embed-assets-script 'go=true web=true docs_only=false full=false ' web/scripts/embed-assets.ts
check service-worker 'go=true web=true docs_only=false full=false ' web/public/sw.js
check fake-agent-names 'go=true web=true docs_only=false full=false ' web/test/e2e/fake-agent-names.txt
check unknown-path 'go=true web=true docs_only=false full=true ' quality.toml
check tooling 'go=true web=true docs_only=false full=true ' .github/scripts/classify-changes.sh
check workflow 'go=true web=true docs_only=false full=true ' .github/workflows/test.yml
check makefile 'go=true web=true docs_only=false full=true ' Makefile
echo 'classifier tests passed'
