#!/usr/bin/env bash

# Creates release packages for all supported platforms.

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

# TODO: Linux ARM builds. Need to build for variants.
# See: https://github.com/golang/go/wiki/GoArm
#
# TODO: Support darwin/arm64, but that currently means iOS.
# Need to wait for Go 1.16 in February.
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
export CGO_ENABLED=0

for build_target in "${build_targets[@]}"; do
  eval export $build_target
  go build .

  pkgname="lazyssh-${version}-${GOOS}-${GOARCH}"
  rm -fr "./release/${pkgname}"
  mkdir -p "./release/${pkgname}"
  cp -r "${extras[@]}" "./release/${pkgname}/"

  if [[ "${GOOS}" = "windows" ]]; then
    mv lazyssh.exe "./release/${pkgname}/"
    (cd ./release/ && zip -r9 "./${pkgname}.zip" "./${pkgname}")
  else
    mv lazyssh "./release/${pkgname}/"
    (cd ./release/ && tar -czf "${pkgname}.tar.gz" "./${pkgname}")
  fi
done
