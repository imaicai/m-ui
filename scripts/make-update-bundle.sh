#!/usr/bin/env bash
# Builds the files to upload to the update directory (default http://n5.870403.xyz:3060/m-ui):
#   VERSION, install.sh, uninstall.sh, m-ui-src.tar.gz, m-ui-src.tar.gz.sha256
#
#   scripts/make-update-bundle.sh [output-dir]      (default: 发布包/<version>)
#
# The source package is `git archive HEAD`, so commit first.
set -euo pipefail
cd "$(dirname "$0")/.."
VERSION="$(tr -d '[:space:]' < VERSION)"
OUT="${1:-发布包/$VERSION}"
mkdir -p "$OUT"
git archive --format=tar --prefix="m-ui-$VERSION/" HEAD | gzip -9n > "$OUT/m-ui-src.tar.gz"
( cd "$OUT" && sha256sum m-ui-src.tar.gz > m-ui-src.tar.gz.sha256 )
cp VERSION install.sh uninstall.sh "$OUT/"
echo "bundle for v$VERSION in $OUT:"
ls -l "$OUT"
