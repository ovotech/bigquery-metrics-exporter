#!/usr/bin/env bash

mkdir -p "$(dirname "${config_path}")"
base64 -d > "${config_path}" <<< "${config_content}"
