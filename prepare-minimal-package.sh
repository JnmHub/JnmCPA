#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DIST_DIR="${ROOT_DIR}/dist"
INSTALLER_DIR="${ROOT_DIR}/installer"
STATIC_DIR="${ROOT_DIR}/static"
DEFAULT_PLATFORMS=(
  "linux-amd64"
  "linux-arm64"
)
PLATFORMS=()
SKIP_BUILD=0

info() {
  printf '[INFO] %s\n' "$*"
}

fail() {
  printf '[ERROR] %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
用法:
  ./prepare-minimal-package.sh
  ./prepare-minimal-package.sh --platform linux-amd64
  ./prepare-minimal-package.sh --platform linux-arm64
  ./prepare-minimal-package.sh --skip-build

说明:
  生成最小部署目录和稳定名称的 zip：
  - dist/minimal-linux-amd64/
  - dist/minimal-linux-amd64.zip
  - dist/minimal-linux-arm64/
  - dist/minimal-linux-arm64.zip
EOF
}

need_command() {
  local cmd="$1"
  local hint="$2"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    fail "$hint"
  fi
}

platform_binary_name() {
  printf 'CLIProxyAPI\n'
}

normalize_platform() {
  local value="$1"
  case "$value" in
    linux-amd64|linux-arm64) printf '%s\n' "$value" ;;
    *) fail "不支持的平台: $value。当前只支持 linux-amd64 / linux-arm64" ;;
  esac
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --platform)
      [[ $# -ge 2 ]] || fail "--platform 后面需要跟平台值"
      PLATFORMS+=("$(normalize_platform "$2")")
      shift 2
      ;;
    --skip-build)
      SKIP_BUILD=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "未知参数: $1"
      ;;
  esac
done

need_command rsync "没有检测到 rsync，请先安装 rsync。"
need_command zip "没有检测到 zip，请先安装 zip。"

if [[ ${#PLATFORMS[@]} -eq 0 ]]; then
  PLATFORMS=("${DEFAULT_PLATFORMS[@]}")
fi

if [[ "$SKIP_BUILD" != "1" ]]; then
  build_args=()
  for platform in "${PLATFORMS[@]}"; do
    build_args+=(--platform "$platform")
  done
  "${ROOT_DIR}/build-binaries.sh" "${build_args[@]}"
fi

for platform in "${PLATFORMS[@]}"; do
  binary_path="${DIST_DIR}/binaries/${platform}/$(platform_binary_name)"
  [[ -f "${binary_path}" ]] || fail "缺少平台二进制: ${binary_path}"

  package_dir="${DIST_DIR}/minimal-${platform}"
  package_zip="${DIST_DIR}/minimal-${platform}.zip"
  stage_dir="$(mktemp -d "${TMPDIR:-/tmp}/minimalpkg.XXXXXX")"
  trap 'rm -rf "${stage_dir}"' RETURN

  info "准备最小部署目录: ${package_dir}"
  rm -rf "${package_dir}"
  mkdir -p "${package_dir}/static"

  cp "${binary_path}" "${package_dir}/CLIProxyAPI"
  cp "${INSTALLER_DIR}/config.yaml" "${package_dir}/config.yaml"
  cp "${INSTALLER_DIR}/install-mongodb.sh" "${package_dir}/install-mongodb.sh"
  cp "${INSTALLER_DIR}/install-systemd.sh" "${package_dir}/install-systemd.sh"
  cp "${INSTALLER_DIR}/start-cpa.sh" "${package_dir}/start-cpa.sh"
  cp "${INSTALLER_DIR}/README.txt" "${package_dir}/README.txt"
  cp "${STATIC_DIR}/management.html" "${package_dir}/static/management.html"
  cp "${STATIC_DIR}/management-enhancer.js" "${package_dir}/static/management-enhancer.js"

  chmod +x \
    "${package_dir}/CLIProxyAPI" \
    "${package_dir}/install-mongodb.sh" \
    "${package_dir}/install-systemd.sh" \
    "${package_dir}/start-cpa.sh"

  mkdir -p "${stage_dir}/minimal-${platform}"
  rsync -a --exclude='.DS_Store' "${package_dir}/" "${stage_dir}/minimal-${platform}/"

  info "生成最小部署 zip: ${package_zip}"
  (
    cd "${stage_dir}" >/dev/null 2>&1
    zip -qr "${package_zip}" "minimal-${platform}"
  )

  rm -rf "${stage_dir}"
  trap - RETURN
done

info "最小部署包已生成完成。"
