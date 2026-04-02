#!/usr/bin/env bash

set -euo pipefail

REPO_OWNER="${CLIPROXY_REPO_OWNER:-JnmHub}"
REPO_NAME="${CLIPROXY_REPO_NAME:-JnmCPA}"
VERSION="${CLIPROXY_VERSION:-cpa}"
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
  printf '[INFO] %s\n' "$*" >&2
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
  CLIPROXY_REPO_OWNER=JnmHub
  CLIPROXY_REPO_NAME=JnmCPA
  CLIPROXY_VERSION=cpa
  CLIPROXY_INSTALL_BASE_DIR=/root
  CLIPROXY_SERVICE_NAME=cliproxyapi
  MANAGEMENT_PASSWORD=你的密码
  MONGOSTORE_URI=mongodb://127.0.0.1:27017
  MONGOSTORE_DATABASE=cliproxy
  MONGOSTORE_COLLECTION=auth_store
  CPA_RESOURCE_KEEP_FREE_PERCENT=9
  CPA_CPU_QUOTA=182%
  CPA_MEMORY_MAX=4294967296
  CPA_GOMEMLIMIT=4080218931

可选参数:
  --skip-mongo
  --skip-systemd
  -h, --help

说明:
  - 脚本运行后会自动安装基础依赖：unzip / ca-certificates（以及缺失时的 curl）
  - 如果你使用的是 `curl | bash`，机器本身必须先有 curl 才能把脚本拉下来
  - 使用 systemd 安装时，默认会为 CPA 预留 9% 的 CPU 和内存
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

detect_distro() {
  if [[ ! -r /etc/os-release ]]; then
    fail "缺少 /etc/os-release，无法识别系统发行版。"
  fi
  # shellcheck disable=SC1091
  . /etc/os-release
  printf '%s %s %s\n' "${ID:-}" "${VERSION_ID:-}" "${VERSION_CODENAME:-}"
}

install_base_packages() {
  local distro_id="$1"

  case "$distro_id" in
    ubuntu|debian)
      apt-get update -y
      DEBIAN_FRONTEND=noninteractive apt-get install -y curl unzip ca-certificates
      ;;
    rhel|centos|rocky|almalinux)
      if command -v dnf >/dev/null 2>&1; then
        dnf install -y curl unzip ca-certificates
      elif command -v yum >/dev/null 2>&1; then
        yum install -y curl unzip ca-certificates
      else
        fail "当前系统没有 dnf/yum，无法自动安装 unzip。"
      fi
      ;;
    *)
      fail "当前系统 ${distro_id} 不在自动安装依赖支持范围内。"
      ;;
  esac
}

ensure_base_dependencies() {
  local distro_id="$1"
  local missing=0

  command -v curl >/dev/null 2>&1 || missing=1
  command -v unzip >/dev/null 2>&1 || missing=1
  command -v systemctl >/dev/null 2>&1 || missing=1

  if [[ "$missing" == "0" ]]; then
    return
  fi

  info "自动安装基础依赖（curl / unzip / ca-certificates）"
  install_base_packages "$distro_id"

  command -v curl >/dev/null 2>&1 || fail "自动安装后仍然没有 curl。"
  command -v unzip >/dev/null 2>&1 || fail "自动安装后仍然没有 unzip。"
  command -v systemctl >/dev/null 2>&1 || fail "当前系统没有 systemctl，脚本只支持 systemd 服务器。"
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
  local distro_id version_id codename platform package_path target_dir extracted_dir
  read -r distro_id version_id codename < <(detect_distro)
  ensure_base_dependencies "$distro_id"

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
    warn "未使用 systemd 时，不会自动应用 CPUQuota / MemoryMax 资源限制。"
    (cd "$extracted_dir" && ./start-cpa.sh start --daemon)
  fi

  info "安装完成"
  printf '\n'
  printf '部署目录: %s\n' "$extracted_dir"
  printf '配置文件: %s/config.yaml\n' "$extracted_dir"
  printf '\n'
}

main "$@"
