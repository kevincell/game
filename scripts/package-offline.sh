#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
bundle="$root/dist/sour-lan-deploy-linux-amd64"
mkdir -p "$bundle/systemd" "$bundle/scripts"
docker save sour-lan:0.2.5 -o "$bundle/sour-lan-image.tar"
cp "$root/docker-compose.yml" "$root/DEPLOYMENT.md" "$bundle/"
cp "$root/systemd/sour-lan.service" "$bundle/systemd/"
cp "$root/scripts/lan-smoke.sh" "$bundle/scripts/"
cp "$root/LICENSE" "$bundle/"
(cd "$bundle" && sha256sum sour-lan-image.tar > SHA256SUMS)
tar -C "$root/dist" -czf "$root/dist/sour-lan-deploy-linux-amd64.tar.gz" \
  sour-lan-deploy-linux-amd64
(cd "$root/dist" && sha256sum sour-lan-deploy-linux-amd64.tar.gz \
  > sour-lan-deploy-linux-amd64.tar.gz.sha256)
printf 'Deployment bundle: %s\n' "$root/dist/sour-lan-deploy-linux-amd64.tar.gz"
