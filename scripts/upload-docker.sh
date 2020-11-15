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
build_archs=("386" "amd64")

manifests=()
for build_arch in "${build_archs[@]}"; do
  manifest="${scratch_repo}:${build_arch}"
  manifests+=("${manifest}")

  echo "- Importing image ${manifest}"
  tar -cC ./release/lazyssh-0.0-linux-${build_arch} lazyssh \
    | docker import --change 'CMD ["/lazyssh"]' - "${manifest}"

  echo "- Pushing image ${manifest}"
  docker push "${manifest}"
done

echo "- Creating manifest list ${manifest_list}"
docker manifest create --insecure "${manifest_list}" "${manifests[@]}"

for build_arch in "${build_archs[@]}"; do
  manifest="${scratch_repo}:${build_arch}"

  echo "- Annotating ${manifest}"
  docker manifest annotate \
    --os linux --arch "${build_arch}" \
    "${manifest_list}" "${manifest}"
done

echo "- Pushing manifest list ${manifest_list}"
docker manifest push --insecure --purge "${manifest_list}"

echo "- Copying to ${upload_repo}:${version}"
for tag in "${version}" "${major}" latest; do
  skopeo --insecure-policy copy --all --src-tls-verify=false \
    "docker://${manifest_list}" \
    "docker://${upload_repo}:${tag}"
done
