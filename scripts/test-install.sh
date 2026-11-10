#!/usr/bin/env bash
# Test harness for scripts/install.sh. Expects dist/ from `make release-snapshot`.
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
INSTALL="$ROOT/scripts/install.sh"
DIST="$ROOT/dist"

[ -f "$DIST/checksums.txt" ] || { echo "dist/ missing: run 'make release-snapshot' first" >&2; exit 2; }

WORK="$(mktemp -d)"
PIDS=""
# shellcheck disable=SC2329 # invoked via trap
cleanup() {
  for p in $PIDS; do kill "$p" 2>/dev/null; wait "$p" 2>/dev/null; done
  chmod -R u+rwx "$WORK" 2>/dev/null
  rm -rf "$WORK"
}
trap cleanup EXIT

# serve <dir>: start an HTTP server on a free port, set BASE_URL.
serve() {
  local port
  port="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1])')"
  (cd "$1" && exec python3 -m http.server "$port" --bind 127.0.0.1 >/dev/null 2>&1) &
  PIDS="$PIDS $!"
  BASE_URL="http://127.0.0.1:$port"
  local i=0
  until curl -fs -o /dev/null "$BASE_URL/checksums.txt"; do
    i=$((i + 1)); [ "$i" -gt 50 ] && { echo "server did not start" >&2; exit 2; }
    sleep 0.1
  done
}

case "$(uname -s)" in Darwin) OS=darwin ;; *) OS=linux ;; esac
case "$(uname -m)" in x86_64|amd64) ARCH=amd64 ;; *) ARCH=arm64 ;; esac
ARCHIVE="landrop_${OS}_${ARCH}.tar.gz"

FAILED=0
pass() { echo "PASS $1"; }
fail() { echo "FAIL $1: $2"; FAILED=1; }

# fresh_env: new temp HOME and install dir.
fresh_env() {
  CASE_HOME="$(mktemp -d "$WORK/home.XXXXXX")"
  DIR="$(mktemp -d "$WORK/bin.XXXXXX")"
}

serve "$DIST"
GOOD_URL="$BASE_URL"

# --- happy path
fresh_env
out="$(HOME="$CASE_HOME" LANDROP_INSTALL_DIR="$DIR" LANDROP_DOWNLOAD_BASE="$GOOD_URL" sh "$INSTALL" 2>&1)"; rc=$?
if [ $rc -eq 0 ] && "$DIR/landrop" --version 2>&1 | grep -q '^landrop v'; then
  pass "happy path"
else
  fail "happy path" "rc=$rc output: $out"
fi

# --- reinstall (same dir)
out="$(HOME="$CASE_HOME" LANDROP_INSTALL_DIR="$DIR" LANDROP_DOWNLOAD_BASE="$GOOD_URL" sh "$INSTALL" 2>&1)"; rc=$?
if [ $rc -eq 0 ] && "$DIR/landrop" --version 2>&1 | grep -q '^landrop v'; then
  pass "reinstall"
else
  fail "reinstall" "rc=$rc output: $out"
fi

# --- tampered checksum
TAMPER="$WORK/tampered"
mkdir "$TAMPER"
cp "$DIST"/*.tar.gz "$DIST"/*.zip "$TAMPER"/
bad="0000000000000000000000000000000000000000000000000000000000000000"
sed "s/^[0-9a-f]*  $ARCHIVE\$/$bad  $ARCHIVE/" "$DIST/checksums.txt" > "$TAMPER/checksums.txt"
if cmp -s "$DIST/checksums.txt" "$TAMPER/checksums.txt"; then
  fail "tampered checksum" "could not tamper checksums.txt for $ARCHIVE"
else
  serve "$TAMPER"
  fresh_env
  out="$(HOME="$CASE_HOME" LANDROP_INSTALL_DIR="$DIR" LANDROP_DOWNLOAD_BASE="$BASE_URL" sh "$INSTALL" 2>&1)"; rc=$?
  if [ $rc -ne 0 ] && [ ! -e "$DIR/landrop" ] && [ -z "$(ls -A "$DIR")" ]; then
    pass "tampered checksum"
  else
    fail "tampered checksum" "rc=$rc dir: $(ls -A "$DIR") output: $out"
  fi
fi

# --- missing checksum entry
NOENTRY="$WORK/noentry"
mkdir "$NOENTRY"
cp "$DIST"/*.tar.gz "$NOENTRY"/
grep -v "  $ARCHIVE\$" "$DIST/checksums.txt" > "$NOENTRY/checksums.txt"
serve "$NOENTRY"
fresh_env
out="$(HOME="$CASE_HOME" LANDROP_INSTALL_DIR="$DIR" LANDROP_DOWNLOAD_BASE="$BASE_URL" sh "$INSTALL" 2>&1)"; rc=$?
if [ $rc -ne 0 ] && [ -z "$(ls -A "$DIR")" ]; then
  pass "missing checksum entry"
else
  fail "missing checksum entry" "rc=$rc output: $out"
fi

# --- unsupported platform
fresh_env
out="$(HOME="$CASE_HOME" LANDROP_INSTALL_DIR="$DIR" LANDROP_DOWNLOAD_BASE="$GOOD_URL" LANDROP_UNAME_M=i686 sh "$INSTALL" 2>&1 >/dev/null)"; rc=$?
if [ $rc -ne 0 ] && echo "$out" | grep -q 'i686'; then
  pass "unsupported platform (arch i686)"
else
  fail "unsupported platform (arch i686)" "rc=$rc stderr: $out"
fi
out="$(HOME="$CASE_HOME" LANDROP_INSTALL_DIR="$DIR" LANDROP_DOWNLOAD_BASE="$GOOD_URL" LANDROP_UNAME_S=FreeBSD sh "$INSTALL" 2>&1 >/dev/null)"; rc=$?
if [ $rc -ne 0 ] && echo "$out" | grep -q 'FreeBSD'; then
  pass "unsupported platform (os FreeBSD)"
else
  fail "unsupported platform (os FreeBSD)" "rc=$rc stderr: $out"
fi

# --- fallback dir
fresh_env
RO="$WORK/readonly"
mkdir "$RO"; chmod 555 "$RO"
if [ "$(id -u)" -eq 0 ]; then
  # root ignores chmod 555; use a path that cannot be created under a regular file.
  : > "$WORK/afile"
  RO="$WORK/afile/bin"
fi
out="$(HOME="$CASE_HOME" LANDROP_DEFAULT_DIR="$RO" LANDROP_DOWNLOAD_BASE="$GOOD_URL" sh "$INSTALL" 2>&1)"; rc=$?
if [ $rc -eq 0 ] && [ -x "$CASE_HOME/.local/bin/landrop" ] && echo "$out" | grep -q 'PATH'; then
  pass "fallback dir"
else
  fail "fallback dir" "rc=$rc output: $out"
fi

exit $FAILED
