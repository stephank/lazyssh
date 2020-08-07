#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo >&2 "Usage: $0 VERSION"
  exit 64
fi

version="$1"

extras=(
  "README.md"
  "COPYING"
  "doc"
)

build_targets=(
  "GOOS=darwin  GOARCH=amd64"
  "GOOS=freebsd GOARCH=386  "
  "GOOS=freebsd GOARCH=amd64"
  "GOOS=linux   GOARCH=386  "
  "GOOS=linux   GOARCH=amd64"
  "GOOS=openbsd GOARCH=386  "
  "GOOS=openbsd GOARCH=amd64"
  "GOOS=solaris GOARCH=amd64"
  "GOOS=windows GOARCH=386  "
  "GOOS=windows GOARCH=amd64"
)

set -x
for build_target in "${build_targets[@]}"; do
  eval export $build_target
  go build .

  pkgname="lazyssh-${version}-${GOOS}-${GOARCH}"
  rm -fr "${pkgname}"
  mkdir "${pkgname}"
  cp -r "${extras[@]}" "./${pkgname}/"

  if [[ "${GOOS}" = "windows" ]]; then
    mv lazyssh.exe "./${pkgname}/"
    zip -r9 "${pkgname}.zip" "./${pkgname}"
  else
    mv lazyssh "./${pkgname}/"
    tar -czf "${pkgname}.tar.gz" "./${pkgname}"
  fi
done
