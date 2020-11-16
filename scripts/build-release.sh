#!/usr/bin/env bash

# Creates release packages for all supported platforms.

set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo >&2 "Usage: $0 VERSION"
  exit 64
fi

version="$1"

# Archive extras.
extras=(
  "README.md"
  "COPYING"
  "doc"
)

# Build variants.
#
# PKGARCH here matches Docker naming for convenience.
#
# TODO: Support darwin/arm64, but that currently means iOS.
# Need to wait for Go 1.16 in February.
build_targets=(
  "GOOS=darwin  GOARCH=amd64       PKGARCH=amd64   "
  "GOOS=freebsd GOARCH=386         PKGARCH=386     "
  "GOOS=freebsd GOARCH=amd64       PKGARCH=amd64   "
  "GOOS=linux   GOARCH=386         PKGARCH=386     "
  "GOOS=linux   GOARCH=amd64       PKGARCH=amd64   "
  "GOOS=linux   GOARCH=arm GOARM=5 PKGARCH=arm32v5 "
  "GOOS=linux   GOARCH=arm GOARM=6 PKGARCH=arm32v6 "
  "GOOS=linux   GOARCH=arm GOARM=7 PKGARCH=arm32v7 "
  "GOOS=linux   GOARCH=arm64       PKGARCH=arm64v8 "
  "GOOS=linux   GOARCH=mips64le    PKGARCH=mips64le"
  "GOOS=linux   GOARCH=ppc64le     PKGARCH=ppc64le "
  "GOOS=linux   GOARCH=s390x       PKGARCH=s390x   "
  "GOOS=openbsd GOARCH=386         PKGARCH=386     "
  "GOOS=openbsd GOARCH=amd64       PKGARCH=amd64   "
  "GOOS=solaris GOARCH=amd64       PKGARCH=amd64   "
  "GOOS=windows GOARCH=386         PKGARCH=386     "
  "GOOS=windows GOARCH=amd64       PKGARCH=amd64   "
)

export CGO_ENABLED=0

for build_target in "${build_targets[@]}"; do
  eval export ${build_target}
  pkgname="lazyssh-${version}-${GOOS}-${PKGARCH}"

  echo "- Building ${pkgname}"
  go build .

  rm -fr "./release/${pkgname}"
  mkdir -p "./release/${pkgname}"
  cp -r "${extras[@]}" "./release/${pkgname}/"

  if [[ "${GOOS}" = "windows" ]]; then
    mv lazyssh.exe "./release/${pkgname}/"
    (cd ./release/ && zip -qr9 "./${pkgname}.zip" "./${pkgname}")
  else
    mv lazyssh "./release/${pkgname}/"
    (cd ./release/ && tar -czf "${pkgname}.tar.gz" "./${pkgname}")
  fi
done
