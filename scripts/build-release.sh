#!/usr/bin/env bash
set -euo pipefail
export LC_ALL=C
export LANG=C

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST="${DIST:-"$ROOT/dist"}"
VERSION="${VERSION:-$(git -C "$ROOT" describe --tags --always --dirty)}"
rm -rf "$DIST"
mkdir -p "$DIST"

targets=(
  "darwin/amd64"
  "darwin/arm64"
  "linux/amd64"
  "linux/arm64"
  "windows/amd64"
  "windows/arm64"
)

for target in "${targets[@]}"; do
  goos="${target%/*}"
  goarch="${target#*/}"
  name="paxm_${VERSION}_${goos}_${goarch}"
  out_dir="$DIST/$name"
  mkdir -p "$out_dir"

  binary="$out_dir/paxm"
  if [[ "$goos" == "windows" ]]; then
    binary="$out_dir/paxm.exe"
  fi

  echo "building $name"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath \
      -ldflags "-s -w -X main.version=${VERSION}" \
      -o "$binary" \
      "$ROOT/cmd/paxm"

  cp "$ROOT/README.md" "$out_dir/README.md"

  if [[ "$goos" == "windows" ]]; then
    (cd "$DIST" && zip -qr "$name.zip" "$name")
  else
    (cd "$DIST" && tar -czf "$name.tar.gz" "$name")
  fi
  rm -rf "$out_dir"
done

cp "$ROOT/scripts/install.sh" "$DIST/install.sh"
chmod 0755 "$DIST/install.sh"

(cd "$DIST" && shasum -a 256 paxm_*.tar.gz paxm_*.zip > SHA256SUMS)
