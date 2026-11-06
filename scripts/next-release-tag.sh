#!/usr/bin/env bash
set -euo pipefail

# Reuse a tag on HEAD when rerunning a failed release or when triggered by a
# manually pushed tag. Otherwise increment the CTG suffix of the latest tag.
for tag in $(git tag --points-at HEAD --sort=-version:refname); do
  if [[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+-ctg\.[0-9]+$ ]]; then
    printf '%s\n' "$tag"
    exit 0
  fi
done

for tag in $(git tag --list 'v*-ctg.*' --sort=-version:refname); do
  if [[ "$tag" =~ ^v([0-9]+\.[0-9]+\.[0-9]+)-ctg\.([0-9]+)$ ]]; then
    printf 'v%s-ctg.%d\n' "${BASH_REMATCH[1]}" "$((10#${BASH_REMATCH[2]} + 1))"
    exit 0
  fi
done

echo 'No CTG release tag exists; create the initial tag explicitly.' >&2
exit 1
