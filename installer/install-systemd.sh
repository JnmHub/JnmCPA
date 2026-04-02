#!/usr/bin/env bash

set -euo pipefail

SERVICE_NAME="${CPA_SERVICE_NAME:-cliproxyapi}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="${CPA_DEPLOY_DIR:-${SCRIPT_DIR}}"
UNIT_PATH="/etc/systemd/system/${SERVICE_NAME}.service"
KEEP_FREE_PERCENT="${CPA_RESOURCE_KEEP_FREE_PERCENT:-9}"
CPU_QUOTA_OVERRIDE="${CPA_CPU_QUOTA:-}"
MEMORY_MAX_OVERRIDE="${CPA_MEMORY_MAX:-}"
GOMEMLIMIT_OVERRIDE="${CPA_GOMEMLIMIT:-}"

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
  CPA_RESOURCE_KEEP_FREE_PERCENT=9
  CPA_CPU_QUOTA=182%
  CPA_MEMORY_MAX=4294967296
  CPA_GOMEMLIMIT=4080218931

说明:
  默认把当前目录注册成 systemd 服务。
  未手动指定时，会自动按“保留 9% 资源”计算 CPUQuota 和 MemoryMax。
EOF
}

validate_keep_free_percent() {
  if [[ ! "$KEEP_FREE_PERCENT" =~ ^[0-9]+$ ]]; then
    fail "CPA_RESOURCE_KEEP_FREE_PERCENT 必须是 0-99 的整数。"
  fi
  if (( KEEP_FREE_PERCENT < 0 || KEEP_FREE_PERCENT > 99 )); then
    fail "CPA_RESOURCE_KEEP_FREE_PERCENT 必须是 0-99 的整数。"
  fi
}

compute_cpu_quota() {
  if [[ -n "$CPU_QUOTA_OVERRIDE" ]]; then
    printf '%s\n' "$CPU_QUOTA_OVERRIDE"
    return
  fi

  local cpu_count usable_percent quota_value
  cpu_count="$(nproc 2>/dev/null || printf '1\n')"
  [[ "$cpu_count" =~ ^[0-9]+$ ]] || cpu_count=1
  (( cpu_count > 0 )) || cpu_count=1

  usable_percent=$((100 - KEEP_FREE_PERCENT))
  quota_value=$((cpu_count * usable_percent))
  (( quota_value > 0 )) || quota_value=1
  printf '%s%%\n' "$quota_value"
}

detect_total_memory_bytes() {
  local mem_total_kib
  mem_total_kib="$(awk '/MemTotal:/ {print $2; exit}' /proc/meminfo 2>/dev/null || true)"
  [[ "$mem_total_kib" =~ ^[0-9]+$ ]] || fail "无法从 /proc/meminfo 读取总内存。"
  printf '%s\n' "$((mem_total_kib * 1024))"
}

compute_memory_max() {
  if [[ -n "$MEMORY_MAX_OVERRIDE" ]]; then
    printf '%s\n' "$MEMORY_MAX_OVERRIDE"
    return
  fi

  local total_memory_bytes usable_percent memory_max_bytes
  total_memory_bytes="$(detect_total_memory_bytes)"
  usable_percent=$((100 - KEEP_FREE_PERCENT))
  memory_max_bytes=$((total_memory_bytes * usable_percent / 100))
  (( memory_max_bytes > 0 )) || fail "自动计算 MemoryMax 失败。"
  printf '%s\n' "$memory_max_bytes"
}

compute_gomemlimit() {
  local memory_max_value="$1"
  if [[ -n "$GOMEMLIMIT_OVERRIDE" ]]; then
    printf '%s\n' "$GOMEMLIMIT_OVERRIDE"
    return
  fi

  if [[ ! "$memory_max_value" =~ ^[0-9]+$ ]]; then
    printf '\n'
    return
  fi

  local gomemlimit_value
  gomemlimit_value=$((memory_max_value * 95 / 100))
  (( gomemlimit_value > 0 )) || fail "自动计算 GOMEMLIMIT 失败。"
  printf '%s\n' "$gomemlimit_value"
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

validate_keep_free_percent

CPU_QUOTA_VALUE="$(compute_cpu_quota)"
MEMORY_MAX_VALUE="$(compute_memory_max)"
GOMEMLIMIT_VALUE="$(compute_gomemlimit "${MEMORY_MAX_VALUE}")"
GOMEMLIMIT_LINE=""
if [[ -n "${GOMEMLIMIT_VALUE}" ]]; then
  GOMEMLIMIT_LINE="Environment=GOMEMLIMIT=${GOMEMLIMIT_VALUE}"
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
CPUAccounting=true
MemoryAccounting=true
CPUQuota=${CPU_QUOTA_VALUE}
MemoryMax=${MEMORY_MAX_VALUE}
${GOMEMLIMIT_LINE}

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable "${SERVICE_NAME}"
systemctl restart "${SERVICE_NAME}"
systemctl is-active --quiet "${SERVICE_NAME}" || fail "${SERVICE_NAME} 启动失败，请执行 journalctl -u ${SERVICE_NAME} -n 100 查看日志。"

info "systemd 服务已安装并启动。"
if [[ -n "${GOMEMLIMIT_VALUE}" ]]; then
  info "资源限制: CPUQuota=${CPU_QUOTA_VALUE}, MemoryMax=${MEMORY_MAX_VALUE}, GOMEMLIMIT=${GOMEMLIMIT_VALUE}"
else
  info "资源限制: CPUQuota=${CPU_QUOTA_VALUE}, MemoryMax=${MEMORY_MAX_VALUE}"
fi
printf '\n'
printf '常用命令:\n'
printf '  systemctl status %s\n' "${SERVICE_NAME}"
printf '  systemctl restart %s\n' "${SERVICE_NAME}"
printf '  systemctl stop %s\n' "${SERVICE_NAME}"
printf '  journalctl -u %s -f\n' "${SERVICE_NAME}"
printf '\n'
