#!/usr/bin/env bash

# Uploads binaries created by `build-release.sh` to Docker Hub.
#
# This script needs Docker running, and a Docker registry at
# `http://localhost:5000` to use as a scratch area. It also needs Skopeo
# installed to copy the final multi-arch image from the local registry to
# Docker Hub.

set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo >&2 "Usage: $0 VERSION"
  exit 64
fi

version="$1"
major="$(echo "${version}" | cut -d . -f 1)"
scratch_repo="localhost:5000/lazyssh"
upload_repo="stephank/lazyssh"
manifest_list="${scratch_repo}:${version}"

# Must be a subset of the target architectures in `build-release.sh`.
build_archs=(
  "ARCH=386      VARIANT=   PKGARCH=386     "
  "ARCH=amd64    VARIANT=   PKGARCH=amd64   "
  "ARCH=arm      VARIANT=v5 PKGARCH=arm32v5 "
  "ARCH=arm      VARIANT=v6 PKGARCH=arm32v6 "
  "ARCH=arm      VARIANT=v7 PKGARCH=arm32v7 "
  "ARCH=arm64    VARIANT=v8 PKGARCH=arm64v8 "
  "ARCH=mips64le VARIANT=   PKGARCH=mips64le"
  "ARCH=ppc64le  VARIANT=   PKGARCH=ppc64le "
  "ARCH=s390x    VARIANT=   PKGARCH=s390x   "
)

manifests=()
for build_arch in "${build_archs[@]}"; do
  eval ${build_arch}
  manifest="${scratch_repo}:${PKGARCH}"
  manifests+=("${manifest}")

  echo "- Importing image ${manifest}"
  tar -cC ./release/lazyssh-${version}-linux-${PKGARCH} lazyssh \
    | docker import --change 'CMD ["/lazyssh"]' - "${manifest}"

  echo "- Pushing image ${manifest}"
  docker push "${manifest}"
done

echo "- Creating manifest list ${manifest_list}"
docker manifest create --insecure "${manifest_list}" "${manifests[@]}"

for build_arch in "${build_archs[@]}"; do
  eval ${build_arch}
  manifest="${scratch_repo}:${PKGARCH}"

  flags="--os linux --arch ${ARCH}"
  if [[ -n "${VARIANT}" ]]; then
    flags="$flags --variant ${VARIANT}"
  fi

  echo "- Annotating ${manifest}"
  docker manifest annotate $flags "${manifest_list}" "${manifest}"
done

echo "- Pushing manifest list ${manifest_list}"
docker manifest push --insecure --purge "${manifest_list}"

echo "- Copying to ${upload_repo}:${version}"
for tag in "${version}" "${major}" latest; do
  skopeo --insecure-policy copy --all --src-tls-verify=false \
    "docker://${manifest_list}" \
    "docker://${upload_repo}:${tag}"
done
