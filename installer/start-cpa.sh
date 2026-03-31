#!/usr/bin/env bash

set -euo pipefail

BASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_PATH="${BASE_DIR}/CLIProxyAPI"
CONFIG_PATH="${BASE_DIR}/config.yaml"
LOG_DIR="${BASE_DIR}/logs"
LOG_FILE="${LOG_DIR}/cliproxyapi.log"
PID_FILE="${BASE_DIR}/cliproxyapi.pid"
COMMAND="${1:-start}"

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
  ./start-cpa.sh start
  ./start-cpa.sh start --daemon
  ./start-cpa.sh stop
  ./start-cpa.sh status
  ./start-cpa.sh logs
  ./start-cpa.sh help
EOF
}

process_running() {
  local pid="$1"
  [[ -n "$pid" ]] && kill -0 "$pid" >/dev/null 2>&1
}

read_pid() {
  [[ -f "$PID_FILE" ]] || return 1
  cat "$PID_FILE" 2>/dev/null || true
}

check_files() {
  [[ -x "$BIN_PATH" ]] || fail "没有找到可执行二进制: $BIN_PATH"
  [[ -f "$CONFIG_PATH" ]] || fail "没有找到配置文件: $CONFIG_PATH"
}

ensure_dirs() {
  mkdir -p "$LOG_DIR"
}

start_fg() {
  check_files
  ensure_dirs
  exec "$BIN_PATH" -config "$CONFIG_PATH"
}

start_bg() {
  check_files
  ensure_dirs

  local pid
  pid="$(read_pid || true)"
  if process_running "$pid"; then
    fail "CLIProxyAPI 已经在运行，PID=${pid}"
  fi

  nohup "$BIN_PATH" -config "$CONFIG_PATH" >"$LOG_FILE" 2>&1 &
  echo $! > "$PID_FILE"
  sleep 2

  pid="$(read_pid || true)"
  if process_running "$pid"; then
    info "后台启动成功，PID=${pid}"
    info "日志文件: $LOG_FILE"
    return
  fi

  tail -n 120 "$LOG_FILE" 2>/dev/null || true
  fail "后台启动失败"
}

stop_app() {
  local pid
  pid="$(read_pid || true)"
  if ! process_running "$pid"; then
    rm -f "$PID_FILE"
    info "CLIProxyAPI 当前未运行"
    return
  fi

  kill "$pid" >/dev/null 2>&1 || true
  for _ in {1..10}; do
    if ! process_running "$pid"; then
      break
    fi
    sleep 1
  done
  if process_running "$pid"; then
    kill -9 "$pid" >/dev/null 2>&1 || true
  fi
  rm -f "$PID_FILE"
  info "CLIProxyAPI 已停止"
}

show_status() {
  local pid
  pid="$(read_pid || true)"
  if process_running "$pid"; then
    info "CLIProxyAPI 运行中，PID=${pid}"
  else
    info "CLIProxyAPI 未运行"
  fi
  info "配置文件: $CONFIG_PATH"
  info "日志文件: $LOG_FILE"
}

show_logs() {
  [[ -f "$LOG_FILE" ]] || fail "日志文件不存在: $LOG_FILE"
  exec tail -f "$LOG_FILE"
}

case "$COMMAND" in
  start)
    if [[ "${2:-}" == "--daemon" || "${2:-}" == "-d" ]]; then
      start_bg
    else
      start_fg
    fi
    ;;
  stop)
    stop_app
    ;;
  status)
    show_status
    ;;
  logs)
    show_logs
    ;;
  help|-h|--help)
    usage
    ;;
  *)
    fail "未知命令: $COMMAND"
    ;;
esac
