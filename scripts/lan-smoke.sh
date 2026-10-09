#!/usr/bin/env bash
set -euo pipefail

base_url="${1:-http://127.0.0.1:1337}"
page="$(curl --fail --silent --show-error "$base_url/")"
grep -qi '<!doctype html>' <<<"$page"
for path in /index.js /background.png /game/api.js /game/sauerbraten.js /game/sauerbraten.wasm; do
  curl --fail --silent --show-error "$base_url$path" -o /dev/null
done
echo "HTTP client root responds at $base_url"
echo "Bundled client, background, engine loaders, and WASM all respond"
