#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo >&2 "Usage: $0 VERSION"
  exit 64
fi

version="$1"

flags=("-m" "")
for pkg in ./lazyssh-${version}-*.{tar.gz,zip}; do
  flags+=("-a" "${pkg}")
done

set -x
hub release edit "${flags[@]}" "${version}"
