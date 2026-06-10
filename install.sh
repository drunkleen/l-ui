#!/bin/bash
set -euo pipefail

# ── Thin installer — downloads the hub binary and hands off to the
#    Go interactive installer wizard (`l-ui install`).
# ─────────────────────────────────────────────────────────────────────

DEBUG=false
for arg in "$@"; do
    [ "$arg" = "-d" ] || [ "$arg" = "--debug" ] && DEBUG=true
done

dbg() { $DEBUG && echo "[debug] $*" >&2 || true; }

ARCH="${LUI_ARCH:-$(uname -m)}"
dbg "raw arch: $ARCH"
case "$ARCH" in
    x86_64|x64|amd64) ARCH="amd64" ;;
    aarch64|arm64)    ARCH="arm64" ;;
    armv7l|armv7)     ARCH="armv7" ;;
    armv6l|armv6)     ARCH="armv6" ;;
    armv5tel|armv5)   ARCH="armv5" ;;
    i*86|x86)         ARCH="386"   ;;
    s390x)            ARCH="s390x" ;;
    *) echo "unsupported arch: $(uname -m)"; exit 1 ;;
esac
dbg "normalized arch: $ARCH"

TAG="${LUI_VERSION:-}"
dbg "LUI_VERSION env: ${LUI_VERSION:-unset}"

if [[ -z "$TAG" ]]; then
    dbg "fetching tag from GitHub API..."
    TAG="$(curl -sfL "https://api.github.com/repos/drunkleen/l-ui/releases/latest" | grep '"tag_name":' | cut -d'"' -f4 || true)"
    dbg "API result: [${TAG:-empty}]"
fi
if [[ -z "$TAG" ]]; then
    dbg "fallback: scraping releases page..."
    TAG="$(curl -sfL "https://github.com/drunkleen/l-ui/releases" | grep -o '/drunkleen/l-ui/releases/tag/[^\"<[:space:]]*' | head -1 | sed 's|.*/tag/||' || true)"
    dbg "fallback result: [${TAG:-empty}]"
fi
if [[ -z "$TAG" ]]; then
    echo "failed to detect latest version"
    exit 1
fi

TARBALL="l-ui-hub-linux-${ARCH}.tar.gz"
URL="https://github.com/drunkleen/l-ui/releases/download/${TAG}/${TARBALL}"

dbg "TAG=$TAG  TARBALL=$TARBALL"
dbg "URL=$URL"

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

printf "downloading %s\n" "${TARBALL}"

# Get content-length from response headers using a byte-range request
SIZE=$(curl -sfL -r 0-0 -D - -o /dev/null "$URL" 2>/dev/null | grep -i content-range | awk -F/ '{print $NF}' | tr -d '\r')
[ -z "$SIZE" ] && SIZE=0

{
    curl -sfL -o "${TMPDIR}/${TARBALL}" "$URL" &
    CURL_PID=$!
    while kill -0 "$CURL_PID" 2>/dev/null; do
        DOWN=$(stat -c%s "${TMPDIR}/${TARBALL}" 2>/dev/null || echo 0)
        if [ -n "$SIZE" ] && [ "$SIZE" -gt 0 ] 2>/dev/null; then
            PCT=$(( DOWN * 100 / SIZE ))
            [ "$PCT" -gt 100 ] && PCT=100
            FILL=$(( PCT * 20 / 100 ))
            BAR=""; i=0
            while [ "$i" -lt "$FILL" ]; do BAR="${BAR}█"; i=$((i+1)); done
            while [ "$i" -lt 20 ]; do BAR="${BAR}░"; i=$((i+1)); done
            MB=$(awk "BEGIN{printf \"%.1f\", $DOWN/1048576}" </dev/null)
            printf "\r[%s] %3d%% | %s MB " "$BAR" "$PCT" "$MB"
        else
            MB=$(awk "BEGIN{printf \"%.1f\", $DOWN/1048576}" </dev/null)
            printf "\rdownloading... %s MB" "$MB"
        fi
        sleep 0.2
    done
    wait "$CURL_PID" || { printf "\ndownload failed\n"; exit 1; }
    # Final stat after download completes
    DOWN=$(stat -c%s "${TMPDIR}/${TARBALL}" 2>/dev/null || echo 0)
    if [ -n "$SIZE" ] && [ "$SIZE" -gt 0 ] 2>/dev/null; then
        PCT=100; FILL=20; BAR=""; i=0
        while [ "$i" -lt 20 ]; do BAR="${BAR}█"; i=$((i+1)); done
        MB=$(awk "BEGIN{printf \"%.1f\", $DOWN/1048576}" </dev/null)
        printf "\r[%s] 100%% | %s MB  " "$BAR" "$MB"
    fi
} 2>&1
printf "\n"
tar -xzf "${TMPDIR}/${TARBALL}" -C "$TMPDIR"

BINARY="${TMPDIR}/l-ui-hub/l-ui"
if [[ ! -x "$BINARY" ]]; then
    echo "binary not found in tarball"
    exit 1
fi

# Filter out debug flags before passing to Go installer
GO_ARGS=()
for arg in "$@"; do
    case "$arg" in -d|--debug) ;; *) GO_ARGS+=("$arg") ;; esac
done

# Hand off to the Go installer wizard with the pre-downloaded tarball.
exec "$BINARY" install --tarball "${TMPDIR}/${TARBALL}" "${GO_ARGS[@]}"
