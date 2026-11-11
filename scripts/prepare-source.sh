#!/usr/bin/env bash
set -Eeuo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_dir=$(cd -- "${script_dir}/.." && pwd)
upstream_repo=${UPSTREAM_REPO:-https://github.com/v03413/bepusdt.git}
upstream_ref=${UPSTREAM_REF:-${1:-main}}
output_dir=${2:-${repo_dir}/upstream-src}

if [[ -e "${output_dir}" ]]; then
  echo "Refusing to overwrite existing path: ${output_dir}" >&2
  exit 1
fi

git clone --filter=blob:none --no-checkout "${upstream_repo}" "${output_dir}"
git -C "${output_dir}" checkout --detach "${upstream_ref}"

shopt -s nullglob
patch_files=("${repo_dir}"/patches/*.patch)
if (( ${#patch_files[@]} == 0 )); then
  echo "No patch files were found in ${repo_dir}/patches." >&2
  exit 2
fi

patch_results=()
for patch_file in "${patch_files[@]}"; do
  patch_name=$(basename "${patch_file}")
  if git -C "${output_dir}" apply --check "${patch_file}"; then
    git -C "${output_dir}" apply "${patch_file}"
    patch_results+=("${patch_name}:applied")
  elif git -C "${output_dir}" apply --reverse --check "${patch_file}"; then
    patch_results+=("${patch_name}:already-in-upstream")
  else
    echo "Patch ${patch_name} is incompatible with ${upstream_ref}." >&2
    echo "Review the upstream changes before building or deploying this version." >&2
    exit 2
  fi
done

install -m 644 "${repo_dir}/patches/query.go" "${output_dir}/app/handler/epay/query.go"
install -m 644 "${repo_dir}/patches/query_test.go" "${output_dir}/app/handler/epay/query_test.go"
install -m 644 "${repo_dir}/patches/order_test.go" "${output_dir}/app/model/order_test.go"
install -m 644 "${repo_dir}/patches/tron_http.go" "${output_dir}/app/task/tron_http.go"
install -m 644 "${repo_dir}/patches/tron_http_test.go" "${output_dir}/app/task/tron_http_test.go"

patch_status=$(IFS=,; echo "${patch_results[*]}")
source_revision=$(git -C "${output_dir}" rev-parse HEAD)
printf 'UPSTREAM_REF=%s\nUPSTREAM_REVISION=%s\nPATCH_STATUS=%s\n' \
  "${upstream_ref}" "${source_revision}" "${patch_status}" \
  > "${output_dir}/.bepusdt-patch-info"

echo "Prepared ${upstream_ref} (${source_revision}) with patch status: ${patch_status}"
