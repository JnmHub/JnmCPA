#!/usr/bin/env bash

set -euo pipefail

REPO_OWNER="${CLIPROXY_REPO_OWNER:-1546079656}"
REPO_NAME="${CLIPROXY_REPO_NAME:-CLIProxyAPI}"
VERSION="${CLIPROXY_VERSION:-latest}"
INSTALL_BASE_DIR="${CLIPROXY_INSTALL_BASE_DIR:-/root}"
SERVICE_NAME="${CLIPROXY_SERVICE_NAME:-cliproxyapi}"
MANAGEMENT_PASSWORD_VALUE="${MANAGEMENT_PASSWORD:-}"
MONGO_URI="${MONGOSTORE_URI:-mongodb://127.0.0.1:27017}"
MONGO_DATABASE="${MONGOSTORE_DATABASE:-cliproxy}"
MONGO_COLLECTION="${MONGOSTORE_COLLECTION:-auth_store}"
SKIP_MONGO=0
SKIP_SYSTEMD=0

TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/cliproxy-install.XXXXXX")"

cleanup() {
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

info() {
  printf '[INFO] %s\n' "$*"
}

warn() {
  printf '[WARN] %s\n' "$*" >&2
}

fail() {
  printf '[ERROR] %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
用法:
  curl -fsSL https://raw.githubusercontent.com/<owner>/<repo>/main/install.sh | bash

可选环境变量:
  CLIPROXY_REPO_OWNER=1546079656
  CLIPROXY_REPO_NAME=CLIProxyAPI
  CLIPROXY_VERSION=latest
  CLIPROXY_INSTALL_BASE_DIR=/root
  CLIPROXY_SERVICE_NAME=cliproxyapi
  MANAGEMENT_PASSWORD=你的密码
  MONGOSTORE_URI=mongodb://127.0.0.1:27017
  MONGOSTORE_DATABASE=cliproxy
  MONGOSTORE_COLLECTION=auth_store

可选参数:
  --skip-mongo
  --skip-systemd
  -h, --help
EOF
}

need_command() {
  local cmd="$1"
  local hint="$2"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    fail "$hint"
  fi
}

require_root() {
  if [[ "${EUID}" -ne 0 ]]; then
    fail "请使用 root 或 sudo 执行。"
  fi
}

detect_linux_platform() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m | tr '[:upper:]' '[:lower:]')"

  [[ "$os" == "linux" ]] || fail "当前安装脚本只支持 Linux 服务器。"

  case "$arch" in
    x86_64|amd64) printf 'linux-amd64\n' ;;
    aarch64|arm64) printf 'linux-arm64\n' ;;
    *) fail "当前 Linux 架构不支持: ${arch}" ;;
  esac
}

release_api_url() {
  if [[ "$VERSION" == "latest" ]]; then
    printf 'https://api.github.com/repos/%s/%s/releases/latest\n' "$REPO_OWNER" "$REPO_NAME"
  else
    printf 'https://api.github.com/repos/%s/%s/releases/tags/%s\n' "$REPO_OWNER" "$REPO_NAME" "$VERSION"
  fi
}

extract_asset_url() {
  local release_json="$1"
  local asset_name="$2"
  printf '%s' "$release_json" \
    | grep -Eo "https://[^\"]+/${asset_name}" \
    | head -n 1
}

download_release_package() {
  local platform="$1"
  local api_url release_json asset_name asset_url output_path

  api_url="$(release_api_url)"
  info "查询发布信息: ${api_url}"
  release_json="$(curl -fsSL -H 'Accept: application/vnd.github+json' -H 'User-Agent: cliproxy-installer' "${api_url}")" \
    || fail "获取 GitHub Release 信息失败。"

  asset_name="minimal-${platform}.zip"
  asset_url="$(extract_asset_url "${release_json}" "${asset_name}")"
  [[ -n "$asset_url" ]] || fail "发布资产里没有找到 ${asset_name}"

  output_path="${TMP_DIR}/${asset_name}"
  info "下载最小部署包: ${asset_name}"
  curl -fsSL "$asset_url" -o "$output_path" || fail "下载部署包失败: ${asset_url}"

  printf '%s\n' "$output_path"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-mongo)
      SKIP_MONGO=1
      shift
      ;;
    --skip-systemd)
      SKIP_SYSTEMD=1
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

main() {
  require_root
  need_command curl "没有检测到 curl，请先安装 curl。"
  need_command unzip "没有检测到 unzip，请先安装 unzip。"

  local platform package_path target_dir extracted_dir
  platform="$(detect_linux_platform)"
  package_path="$(download_release_package "$platform")"

  target_dir="${INSTALL_BASE_DIR}/minimal-${platform}"
  rm -rf "$target_dir"
  mkdir -p "$INSTALL_BASE_DIR"
  unzip -q "$package_path" -d "$INSTALL_BASE_DIR"
  extracted_dir="${INSTALL_BASE_DIR}/minimal-${platform}"
  [[ -d "$extracted_dir" ]] || fail "解压后没有找到目录: ${extracted_dir}"

  chmod +x \
    "${extracted_dir}/CLIProxyAPI" \
    "${extracted_dir}/install-mongodb.sh" \
    "${extracted_dir}/install-systemd.sh" \
    "${extracted_dir}/start-cpa.sh"

  if [[ "$SKIP_MONGO" != "1" ]]; then
    info "安装 MongoDB"
    (cd "$extracted_dir" && ./install-mongodb.sh)
  fi

  if [[ -n "$MANAGEMENT_PASSWORD_VALUE" ]]; then
    export MANAGEMENT_PASSWORD="${MANAGEMENT_PASSWORD_VALUE}"
  fi
  export MONGOSTORE_URI="${MONGO_URI}"
  export MONGOSTORE_DATABASE="${MONGO_DATABASE}"
  export MONGOSTORE_COLLECTION="${MONGO_COLLECTION}"

  if [[ "$SKIP_SYSTEMD" != "1" ]]; then
    info "注册 systemd 服务"
    (cd "$extracted_dir" && CPA_DEPLOY_DIR="$extracted_dir" CPA_SERVICE_NAME="$SERVICE_NAME" ./install-systemd.sh)
  else
    info "跳过 systemd，直接后台启动"
    (cd "$extracted_dir" && ./start-cpa.sh start --daemon)
  fi

  info "安装完成"
  printf '\n'
  printf '部署目录: %s\n' "$extracted_dir"
  printf '配置文件: %s/config.yaml\n' "$extracted_dir"
  printf '\n'
}

main "$@"
