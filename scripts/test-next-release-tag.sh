#!/usr/bin/env bash
set -euo pipefail

script="$(cd "$(dirname "$0")" && pwd)/next-release-tag.sh"
repo="$(mktemp -d)"
trap 'rm -rf "$repo"' EXIT

git -C "$repo" init -q
git -C "$repo" config user.name test
git -C "$repo" config user.email test@example.com
git -C "$repo" commit --allow-empty -qm base
if (cd "$repo" && "$script") >/dev/null 2>&1; then
  echo 'Expected a failure without an initial CTG tag' >&2
  exit 1
fi

git -C "$repo" tag -a v6.14.0-ctg.2 -m old
git -C "$repo" commit --allow-empty -qm next
[[ "$(cd "$repo" && "$script")" == v6.14.0-ctg.3 ]]

git -C "$repo" tag -a v6.14.0-ctg.10 -m current
[[ "$(cd "$repo" && "$script")" == v6.14.0-ctg.10 ]]

git -C "$repo" commit --allow-empty -qm later
[[ "$(cd "$repo" && "$script")" == v6.14.0-ctg.11 ]]

git -C "$repo" tag -a v6.15.0-ctg.1 -m upstream
git -C "$repo" commit --allow-empty -qm following
[[ "$(cd "$repo" && "$script")" == v6.15.0-ctg.2 ]]
