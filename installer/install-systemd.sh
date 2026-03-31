#!/usr/bin/env bash

set -euo pipefail

SERVICE_NAME="${CPA_SERVICE_NAME:-cliproxyapi}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="${CPA_DEPLOY_DIR:-${SCRIPT_DIR}}"
UNIT_PATH="/etc/systemd/system/${SERVICE_NAME}.service"

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
  sudo ./install-systemd.sh

可选环境变量:
  CPA_SERVICE_NAME=cliproxyapi
  CPA_DEPLOY_DIR=/root/minimal-linux-amd64

说明:
  默认把当前目录注册成 systemd 服务。
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" || "${1:-}" == "help" ]]; then
  usage
  exit 0
fi

if [[ "${EUID}" -ne 0 ]]; then
  fail "请使用 root 或 sudo 执行。"
fi

if ! command -v systemctl >/dev/null 2>&1; then
  fail "当前系统没有 systemctl，无法注册 systemd 服务。"
fi

if [[ ! -d "${DEPLOY_DIR}" ]]; then
  fail "部署目录不存在: ${DEPLOY_DIR}"
fi

if [[ ! -x "${DEPLOY_DIR}/CLIProxyAPI" ]]; then
  fail "没有找到可执行二进制: ${DEPLOY_DIR}/CLIProxyAPI"
fi

if [[ ! -f "${DEPLOY_DIR}/config.yaml" ]]; then
  fail "没有找到配置文件: ${DEPLOY_DIR}/config.yaml"
fi

cat > "${UNIT_PATH}" <<EOF
[Unit]
Description=CLIProxyAPI
After=network-online.target mongod.service
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=${DEPLOY_DIR}
ExecStart=${DEPLOY_DIR}/CLIProxyAPI -config ${DEPLOY_DIR}/config.yaml
Restart=always
RestartSec=3
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable "${SERVICE_NAME}"
systemctl restart "${SERVICE_NAME}"
systemctl is-active --quiet "${SERVICE_NAME}" || fail "${SERVICE_NAME} 启动失败，请执行 journalctl -u ${SERVICE_NAME} -n 100 查看日志。"

info "systemd 服务已安装并启动。"
printf '\n'
printf '常用命令:\n'
printf '  systemctl status %s\n' "${SERVICE_NAME}"
printf '  systemctl restart %s\n' "${SERVICE_NAME}"
printf '  systemctl stop %s\n' "${SERVICE_NAME}"
printf '  journalctl -u %s -f\n' "${SERVICE_NAME}"
printf '\n'
