#!/usr/bin/env bash

set -euo pipefail

info() {
  printf '[INFO] %s\n' "$*"
}

fail() {
  printf '[ERROR] %s\n' "$*" >&2
  exit 1
}

need_root() {
  if [[ "${EUID}" -ne 0 ]]; then
    fail "请使用 root 或 sudo 执行。"
  fi
}

detect_distro() {
  if [[ ! -r /etc/os-release ]]; then
    fail "无法识别系统，缺少 /etc/os-release"
  fi
  # shellcheck disable=SC1091
  . /etc/os-release
  printf '%s %s %s\n' "${ID:-}" "${VERSION_ID:-}" "${VERSION_CODENAME:-}"
}

rpm_arch() {
  case "$(uname -m)" in
    x86_64|amd64) printf 'x86_64\n' ;;
    aarch64|arm64) printf 'aarch64\n' ;;
    *) fail "当前架构不支持自动安装 MongoDB" ;;
  esac
}

install_apt() {
  local distro_id="$1"
  local codename="$2"

  apt-get update -y
  apt-get install -y curl gnupg ca-certificates

  curl -fsSL https://www.mongodb.org/static/pgp/server-8.0.asc \
    | gpg -o /usr/share/keyrings/mongodb-server-8.0.gpg --dearmor

  case "$distro_id" in
    ubuntu)
      echo "deb [ arch=amd64,arm64 signed-by=/usr/share/keyrings/mongodb-server-8.0.gpg ] https://repo.mongodb.org/apt/ubuntu ${codename}/mongodb-org/8.0 multiverse" \
        > /etc/apt/sources.list.d/mongodb-org-8.0.list
      ;;
    debian)
      echo "deb [ signed-by=/usr/share/keyrings/mongodb-server-8.0.gpg ] https://repo.mongodb.org/apt/debian ${codename}/mongodb-org/8.0 main" \
        > /etc/apt/sources.list.d/mongodb-org-8.0.list
      ;;
    *)
      fail "apt 安装只支持 Ubuntu / Debian"
      ;;
  esac

  apt-get update -y
  DEBIAN_FRONTEND=noninteractive apt-get install -y mongodb-org
}

install_rpm() {
  local version_id="$1"
  local major="${version_id%%.*}"
  local arch
  arch="$(rpm_arch)"

  cat > /etc/yum.repos.d/mongodb-org-8.0.repo <<EOF
[mongodb-org-8.0]
name=MongoDB Repository
baseurl=https://repo.mongodb.org/yum/redhat/${major}/mongodb-org/8.0/${arch}/
gpgcheck=1
enabled=1
gpgkey=https://pgp.mongodb.com/server-8.0.asc
EOF

  if command -v dnf >/dev/null 2>&1; then
    dnf install -y curl ca-certificates mongodb-org
  else
    yum install -y curl ca-certificates mongodb-org
  fi
}

configure_local_only() {
  local conf="/etc/mongod.conf"
  if [[ ! -f "$conf" ]]; then
    fail "没有找到 $conf"
  fi

  if grep -q '^[[:space:]]*bindIp:' "$conf"; then
    sed -i.bak 's/^[[:space:]]*bindIp:.*/  bindIp: 127.0.0.1/' "$conf"
  elif grep -q '^[[:space:]]*port:' "$conf"; then
    awk '
      { print }
      /^[[:space:]]*port:/ && !done {
        print "net:"
        print "  bindIp: 127.0.0.1"
        done=1
      }
    ' "$conf" > "${conf}.tmp" && mv "${conf}.tmp" "$conf"
  fi
}

main() {
  need_root

  if command -v mongod >/dev/null 2>&1; then
    info "检测到 mongod 已安装，跳过安装。"
  else
    local distro_id version_id codename
    read -r distro_id version_id codename < <(detect_distro)
    info "安装 MongoDB 8.0 到 ${distro_id} ${version_id}"
    case "$distro_id" in
      ubuntu|debian)
        install_apt "$distro_id" "$codename"
        ;;
      rhel|centos|rocky|almalinux)
        install_rpm "$version_id"
        ;;
      *)
        fail "当前系统 ${distro_id} 不在自动安装支持范围内"
        ;;
    esac
  fi

  configure_local_only
  systemctl enable mongod
  systemctl restart mongod
  systemctl is-active --quiet mongod || fail "mongod 启动失败，请执行 journalctl -u mongod -n 100 查看日志"
  info "MongoDB 已安装并启动，当前仅监听 127.0.0.1:27017"
}

main "$@"
