#!/usr/bin/env bash
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."
mkdir -p dist
go_cmd="${GO_BINARY:-go}"
for arch in amd64 arm64;do
 work_dir=$(mktemp -d)
 CGO_ENABLED=0 GOOS=linux GOARCH="$arch" "$go_cmd" build -buildvcs=false -trimpath -ldflags='-s -w' -o "$work_dir/namedicmp" ./cmd/namedicmp
 cp README.md README.fa.md LICENSE THIRD_PARTY_NOTICES.md SECURITY.md "$work_dir/"
 cp -R docs "$work_dir/docs"
 tar -czf "dist/namedicmp-linux-$arch.tar.gz" -C "$work_dir" namedicmp README.md README.fa.md LICENSE THIRD_PARTY_NOTICES.md SECURITY.md docs
 rm -rf -- "$work_dir"
done
(cd dist && sha256sum namedicmp-linux-*.tar.gz >SHA256SUMS)
