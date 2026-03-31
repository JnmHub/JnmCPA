#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_NAME="$(basename "$ROOT_DIR")"
DIST_DIR="${ROOT_DIR}/dist"
TIMESTAMP="$(date +%Y%m%d-%H%M%S)"
OUTPUT_PATH="${DIST_DIR}/${PROJECT_NAME}-${TIMESTAMP}.zip"
INCLUDE_CONFIG=1
BINARY_OUTPUT_ROOT="${ROOT_DIR}/dist/binaries"

info() {
  printf '[INFO] %s\n' "$*"
}

fail() {
  printf '[ERROR] %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<EOF
用法:
  ./package-project-zip.sh
  ./package-project-zip.sh --output ./dist/custom-name.zip
  ./package-project-zip.sh --include-config

说明:
  默认会打包整个项目为 zip，并排除：
  - 前端源码目录: static/Cli-Proxy-API-Management-Center-main
  - .git
  - dist（但会把 dist/binaries 下的全平台二进制单独带进包里）
  - logs / mongo-data / temp 等本机运行产物
  - .env 等本地敏感配置

可选参数:
  --output <path>       指定输出 zip 路径
  --include-config      显式包含本地 config.yaml（默认已包含）
  -h, --help            显示帮助
EOF
}

need_command() {
  local cmd="$1"
  local hint="$2"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    fail "$hint"
  fi
}

abs_path() {
  local target="$1"
  local dir
  local base

  if [[ -d "$target" ]]; then
    (
      cd "$target" >/dev/null 2>&1 && pwd
    )
    return
  fi

  dir="$(dirname "$target")"
  base="$(basename "$target")"
  mkdir -p "$dir"
  (
    cd "$dir" >/dev/null 2>&1 && printf '%s/%s\n' "$(pwd)" "$base"
  )
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --output)
      [[ $# -ge 2 ]] || fail "--output 后面需要跟文件路径"
      OUTPUT_PATH="$2"
      shift 2
      ;;
    --include-config)
      INCLUDE_CONFIG=1
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
need_command go "没有检测到 go，请先安装 Go 1.26+。打包 zip 默认会先构建全平台二进制。"

if command -v zip >/dev/null 2>&1; then
  ZIP_MODE="zip"
elif command -v ditto >/dev/null 2>&1; then
  ZIP_MODE="ditto"
else
  fail "没有检测到 zip 或 ditto，请先安装 zip（或在 macOS 上使用系统自带 ditto）。"
fi

OUTPUT_PATH="$(abs_path "$OUTPUT_PATH")"
mkdir -p "$(dirname "$OUTPUT_PATH")"

STAGING_DIR="$(mktemp -d "${TMPDIR:-/tmp}/cliproxy-package.XXXXXX")"
cleanup() {
  rm -rf "$STAGING_DIR"
}
trap cleanup EXIT

STAGING_PROJECT_DIR="${STAGING_DIR}/${PROJECT_NAME}"
mkdir -p "$STAGING_PROJECT_DIR"

info "先构建全平台二进制..."
bash "${ROOT_DIR}/build-binaries.sh" --output "${BINARY_OUTPUT_ROOT}" >/dev/null

RSYNC_EXCLUDES=(
  "--exclude=.git/"
  "--exclude=dist/"
  "--exclude=logs/"
  "--exclude=mongo-data/"
  "--exclude=temp/"
  "--exclude=.env"
  "--exclude=.env.local"
  "--exclude=.env.*.local"
  "--exclude=.DS_Store"
  "--exclude=static/.DS_Store"
  "--exclude=static/Cli-Proxy-API-Management-Center-main/"
  "--exclude=root@*"
)

if [[ "$INCLUDE_CONFIG" != "1" ]]; then
  RSYNC_EXCLUDES+=("--exclude=config.yaml")
fi

info "准备打包项目..."
rsync -a "${RSYNC_EXCLUDES[@]}" "${ROOT_DIR}/" "${STAGING_PROJECT_DIR}/"
mkdir -p "${STAGING_PROJECT_DIR}/binaries"
rsync -a "${BINARY_OUTPUT_ROOT}/" "${STAGING_PROJECT_DIR}/binaries/"

if [[ "$ZIP_MODE" == "zip" ]]; then
  info "使用 zip 生成压缩包..."
  (
    cd "$STAGING_DIR" >/dev/null 2>&1
    zip -qr "$OUTPUT_PATH" "$PROJECT_NAME"
  )
else
  info "使用 ditto 生成压缩包..."
  ditto -c -k --sequesterRsrc --keepParent "$STAGING_PROJECT_DIR" "$OUTPUT_PATH"
fi

info "打包完成: $OUTPUT_PATH"
printf '\n'
printf '已排除:\n'
printf '  - static/Cli-Proxy-API-Management-Center-main/\n'
printf '  - .git/\n'
printf '  - dist/（已单独包含 dist/binaries 的全平台二进制）\n'
printf '  - logs/ mongo-data/ temp/\n'
printf '  - 已包含 config.yaml\n'
printf '\n'
