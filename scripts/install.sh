#!/bin/sh
# LanDrop installer for macOS and Linux.
#
#   curl -fsSL https://raw.githubusercontent.com/lupguo/landrop/main/scripts/install.sh | sh
#
# Environment:
#   LANDROP_VERSION       tag to install, e.g. v0.1.0 (default: latest)
#   LANDROP_INSTALL_DIR   install directory (default: /usr/local/bin if writable, else ~/.local/bin)
set -eu

RELEASES_URL="https://github.com/lupguo/landrop/releases"

err() {
  echo "error: $*" >&2
  exit 1
}

# --- platform
uname_s="${LANDROP_UNAME_S:-$(uname -s)}"
uname_m="${LANDROP_UNAME_M:-$(uname -m)}"

case "$uname_s" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) err "unsupported OS '$uname_s'. Download a build manually from $RELEASES_URL" ;;
esac

case "$uname_m" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) err "unsupported architecture '$uname_m'. Download a build manually from $RELEASES_URL" ;;
esac

archive="landrop_${os}_${arch}.tar.gz"

# --- download base
if [ -n "${LANDROP_DOWNLOAD_BASE:-}" ]; then
  base="$LANDROP_DOWNLOAD_BASE"
elif [ -n "${LANDROP_VERSION:-}" ]; then
  base="$RELEASES_URL/download/$LANDROP_VERSION"
else
  base="$RELEASES_URL/latest/download"
fi

# --- downloader
if command -v curl >/dev/null 2>&1; then
  fetch() { curl -fsSL -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
  fetch() { wget -q -O "$2" "$1"; }
else
  err "neither curl nor wget found; install one of them and retry"
fi

# --- sha256 tool
if command -v sha256sum >/dev/null 2>&1; then
  sha256() { sha256sum "$1" | cut -d ' ' -f 1; }
elif command -v shasum >/dev/null 2>&1; then
  sha256() { shasum -a 256 "$1" | cut -d ' ' -f 1; }
else
  err "neither sha256sum nor shasum found; cannot verify the download"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "Downloading $archive from $base ..."
fetch "$base/$archive" "$tmp/$archive" || err "failed to download $base/$archive"
fetch "$base/checksums.txt" "$tmp/checksums.txt" || err "failed to download $base/checksums.txt"

# --- verify
expected="$(awk -v f="$archive" '$2 == f { print $1; exit }' "$tmp/checksums.txt")"
[ -n "$expected" ] || err "no checksum entry for $archive in checksums.txt"
actual="$(sha256 "$tmp/$archive")"
if [ "$expected" != "$actual" ]; then
  err "checksum mismatch for $archive (expected $expected, got $actual); nothing installed"
fi
echo "Checksum OK."

# --- extract
mkdir "$tmp/extract"
tar -xzf "$tmp/$archive" -C "$tmp/extract"
[ -f "$tmp/extract/landrop" ] || err "archive does not contain a landrop binary"
chmod +x "$tmp/extract/landrop"

# --- choose install dir
writable_dir() {
  [ -d "$1" ] || return 1
  probe="$1/.landrop.probe.$$"
  if : >"$probe" 2>/dev/null; then
    rm -f "$probe"
    return 0
  fi
  return 1
}

if [ -n "${LANDROP_INSTALL_DIR:-}" ]; then
  dir="$LANDROP_INSTALL_DIR"
  mkdir -p "$dir" || err "cannot create $dir"
else
  default_dir="${LANDROP_DEFAULT_DIR:-/usr/local/bin}"
  if writable_dir "$default_dir"; then
    dir="$default_dir"
  else
    dir="$HOME/.local/bin"
    echo "$default_dir is not writable; installing to $dir instead."
    echo "(To install system-wide, re-run with LANDROP_INSTALL_DIR set to a writable directory, or use sudo yourself.)"
    mkdir -p "$dir" || err "cannot create $dir"
  fi
fi

# --- install atomically
cp "$tmp/extract/landrop" "$dir/.landrop.tmp" || err "cannot write to $dir"
chmod +x "$dir/.landrop.tmp"
mv -f "$dir/.landrop.tmp" "$dir/landrop"

echo "Installed to $dir/landrop"
"$dir/landrop" --version

case ":$PATH:" in
  *":$dir:"*) ;;
  *)
    echo
    echo "$dir is not on your PATH. Add it with:"
    echo "  export PATH=\"$dir:\$PATH\""
    ;;
esac

echo
echo "Run: landrop"
