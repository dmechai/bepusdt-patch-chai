#!/usr/bin/env bash
set -Eeuo pipefail

api_url=${UPSTREAM_RELEASE_API:-https://api.github.com/repos/v03413/bepusdt/releases/latest}
auth_args=()
if [[ -n "${GITHUB_TOKEN:-}" ]]; then
  auth_args=(-H "Authorization: Bearer ${GITHUB_TOKEN}")
fi

release_json=$(curl --fail --silent --show-error --location "${auth_args[@]}" "${api_url}")
tag=$(printf '%s' "${release_json}" | jq --exit-status --raw-output '.tag_name // empty')

if [[ -z "${tag}" ]]; then
  echo "Could not determine the latest upstream release tag." >&2
  exit 1
fi

printf '%s\n' "${tag}"
