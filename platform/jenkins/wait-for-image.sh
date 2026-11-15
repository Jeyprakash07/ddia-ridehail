#!/usr/bin/env bash
# Waits until CI has published the image for a commit to GHCR.
#
#   platform/jenkins/wait-for-image.sh <git commit sha>
set -euo pipefail

SHA="$1"
REPO="jeyprakash07/ddia-ridehail"
ACCEPT="application/vnd.oci.image.index.v1+json,application/vnd.oci.image.manifest.v1+json,application/vnd.docker.distribution.manifest.v2+json"

for attempt in $(seq 1 40); do
  # The package is public, so an anonymous pull token is enough.
  TOKEN="$(curl -fsS "https://ghcr.io/token?scope=repository:${REPO}:pull" | sed -E 's/.*"token":"([^"]*)".*/\1/')"
  CODE="$(curl -s -o /dev/null -w '%{http_code}' -I \
    -H "Authorization: Bearer ${TOKEN}" -H "Accept: ${ACCEPT}" \
    "https://ghcr.io/v2/${REPO}/manifests/${SHA}")"
  if [ "$CODE" = "200" ]; then
    echo "Image for ${SHA} is published."
    exit 0
  fi
  echo "Attempt ${attempt}: image for ${SHA} not published yet (HTTP ${CODE}); waiting 15 s."
  sleep 15
done

echo "Gave up after 10 minutes. Did CI fail for this commit?" >&2
exit 1
