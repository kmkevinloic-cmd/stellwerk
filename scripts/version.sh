#!/usr/bin/env bash
# Keeps every component's version in sync with the VERSION file.
#
#   scripts/version.sh check          fail if any component disagrees with VERSION
#   scripts/version.sh bump X.Y.Z     write X.Y.Z to VERSION and every component
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

semver='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'

# file|sed pattern capturing the version|replacement template (@V@ = version)
targets=(
  'edge/CMakeLists.txt|^project\(stellwerk_edge VERSION ([0-9.]+)|project(stellwerk_edge VERSION @V@'
  'sim/pyproject.toml|^version = "([0-9.]+)"|version = "@V@"'
  'sim/stellwerk_sim/__init__.py|^__version__ = "([0-9.]+)"|__version__ = "@V@"'
  'control-plane/cmd/stellwerkd/main.go|^var version = "([0-9.]+)"|var version = "@V@"'
)

current() { tr -d '[:space:]' < VERSION; }

check() {
  local want status=0
  want="$(current)"
  [[ "$want" =~ $semver ]] || { echo "VERSION '$want' is not X.Y.Z" >&2; exit 1; }
  for t in "${targets[@]}"; do
    IFS='|' read -r file pattern _ <<<"$t"
    got="$(sed -nE "s/${pattern}.*/\1/p" "$file" | head -n1)"
    if [[ "$got" != "$want" ]]; then
      echo "version mismatch: $file has '${got:-<none>}', VERSION has '$want'" >&2
      status=1
    fi
  done
  if ! grep -qE "^## \[$want\]" CHANGELOG.md; then
    echo "CHANGELOG.md has no '## [$want]' section" >&2
    status=1
  fi
  [[ $status -eq 0 ]] && echo "all components at $want"
  return $status
}

bump() {
  local new="$1"
  [[ "$new" =~ $semver ]] || { echo "usage: $0 bump X.Y.Z" >&2; exit 2; }
  echo "$new" > VERSION
  for t in "${targets[@]}"; do
    IFS='|' read -r file pattern template <<<"$t"
    sed -i -E "s/${pattern}/${template//@V@/$new}/" "$file"
  done
  echo "bumped to $new. Now move the [Unreleased] notes in CHANGELOG.md to a '## [$new]' section."
}

case "${1:-}" in
  check) check ;;
  bump) bump "${2:-}" ;;
  *) echo "usage: $0 check | bump X.Y.Z" >&2; exit 2 ;;
esac
