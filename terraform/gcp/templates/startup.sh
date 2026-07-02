#!/usr/bin/env bash
set -euo pipefail

# Write configuration file for the bqmetrics daemon
mkdir -p "$(dirname "${config_path}")"
base64 -d > "${config_path}" <<< "${config_content}"

# Launch the daemon container. Host networking is used so the health check
# endpoint on port 8080 is reachable at the VM IP without explicit port
# publishing, matching the original konlet-managed behaviour.
docker rm -f bqmetricsd 2>/dev/null || true
docker run -d \
  --name=bqmetricsd \
  --restart=always \
  --network=host \
  -v "$(dirname "${config_path}")":"$(dirname "${config_path}")":ro \
  -e "LOG_LEVEL=${log_level}" \
  "${image}" \
  --config-file="${config_path}"
