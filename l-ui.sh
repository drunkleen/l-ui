#!/bin/bash
# Thin wrapper — delegates everything to the Go binary.
LUI_BIN="${LUI_MAIN_FOLDER:-/usr/local/l-ui}/l-ui"
if [[ -x "$LUI_BIN" ]] && [[ "$LUI_SKIP_BINARY" != "true" ]]; then
    exec "$LUI_BIN" "$@"
fi
# Fallback: if the Go binary is not available, show instructions.
echo "L-UI binary not found. Install it with:"
echo "  bash <(curl -Ls https://raw.githubusercontent.com/drunkleen/l-ui/master/install.sh)"
exit 1
