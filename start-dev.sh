#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FRONTEND_DIR="${ROOT_DIR}/static/Cli-Proxy-API-Management-Center-main"
CONFIG_PATH="${CLI_PROXY_CONFIG_PATH:-${ROOT_DIR}/config.yaml}"
MODE="backend"
MANAGEMENT_PASSWORD_VALUE="${MANAGEMENT_PASSWORD:-}"

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
  ./start-dev.sh
  ./start-dev.sh --frontend
  ./start-dev.sh --all
  ./start-dev.sh --config ./config.yaml
  MANAGEMENT_PASSWORD=你的密码 ./start-dev.sh

默认行为:
  - 启动后端开发模式:
    go run ./cmd/server -config ./config.yaml

可选参数:
  --backend      仅启动后端（默认）
  --frontend     仅启动前端 Vite dev
  --all          同时启动后端和前端
  --config PATH  指定配置文件
  -h, --help     显示帮助

可选环境变量:
  MANAGEMENT_PASSWORD
  CLI_PROXY_CONFIG_PATH
EOF
}

need_command() {
  local cmd="$1"
  local hint="$2"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    fail "$hint"
  fi
}

run_backend() {
  need_command go "没有检测到 go，请先安装 Go 1.26+。"
  [[ -f "$CONFIG_PATH" ]] || fail "配置文件不存在: $CONFIG_PATH"

  info "启动后端开发模式"
  info "配置文件: $CONFIG_PATH"

  if [[ -n "$MANAGEMENT_PASSWORD_VALUE" ]]; then
    MANAGEMENT_PASSWORD="$MANAGEMENT_PASSWORD_VALUE" \
      go run ./cmd/server -config "$CONFIG_PATH"
  else
    warn "未设置 MANAGEMENT_PASSWORD，若本地管理接口依赖环境变量密码，请自行导出后再启动。"
    go run ./cmd/server -config "$CONFIG_PATH"
  fi
}

run_frontend() {
  need_command npm "没有检测到 npm，请先安装 Node.js / npm。"
  [[ -d "$FRONTEND_DIR" ]] || fail "前端目录不存在: $FRONTEND_DIR"

  info "启动前端 Vite dev"
  (
    cd "$FRONTEND_DIR"
    npm run dev
  )
}

run_all() {
  need_command go "没有检测到 go，请先安装 Go 1.26+。"
  need_command npm "没有检测到 npm，请先安装 Node.js / npm。"
  [[ -f "$CONFIG_PATH" ]] || fail "配置文件不存在: $CONFIG_PATH"
  [[ -d "$FRONTEND_DIR" ]] || fail "前端目录不存在: $FRONTEND_DIR"

  info "同时启动前后端开发模式"
  info "后端配置: $CONFIG_PATH"
  info "前端目录: $FRONTEND_DIR"

  (
    cd "$FRONTEND_DIR"
    npm run dev
  ) &
  FRONTEND_PID=$!

  cleanup() {
    if kill -0 "$FRONTEND_PID" >/dev/null 2>&1; then
      kill "$FRONTEND_PID" >/dev/null 2>&1 || true
      wait "$FRONTEND_PID" 2>/dev/null || true
    fi
  }
  trap cleanup EXIT INT TERM

  if [[ -n "$MANAGEMENT_PASSWORD_VALUE" ]]; then
    MANAGEMENT_PASSWORD="$MANAGEMENT_PASSWORD_VALUE" \
      go run ./cmd/server -config "$CONFIG_PATH"
  else
    warn "未设置 MANAGEMENT_PASSWORD，若本地管理接口依赖环境变量密码，请自行导出后再启动。"
    go run ./cmd/server -config "$CONFIG_PATH"
  fi
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --backend)
      MODE="backend"
      shift
      ;;
    --frontend)
      MODE="frontend"
      shift
      ;;
    --all)
      MODE="all"
      shift
      ;;
    --config)
      [[ $# -ge 2 ]] || fail "--config 后面需要跟文件路径"
      CONFIG_PATH="$2"
      shift 2
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

cd "$ROOT_DIR"

case "$MODE" in
  backend)
    run_backend
    ;;
  frontend)
    run_frontend
    ;;
  all)
    run_all
    ;;
  *)
    fail "未知启动模式: $MODE"
    ;;
esac
