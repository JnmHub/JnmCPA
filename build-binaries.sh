#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_ROOT="${ROOT_DIR}/dist/binaries"
DEFAULT_PLATFORMS=(
  "linux-amd64"
  "linux-arm64"
  "darwin-amd64"
  "darwin-arm64"
  "windows-amd64"
  "windows-arm64"
)

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
  ./build-binaries.sh
  ./build-binaries.sh --platform linux-amd64
  ./build-binaries.sh --platform linux-amd64 --platform darwin-arm64
  ./build-binaries.sh --output ./dist/custom-binaries

说明:
  默认构建常用平台二进制：
  - linux-amd64
  - linux-arm64
  - darwin-amd64
  - darwin-arm64
  - windows-amd64
  - windows-arm64
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
  local platform="$1"
  case "$platform" in
    windows-*) printf 'CLIProxyAPI.exe\n' ;;
    *) printf 'CLIProxyAPI\n' ;;
  esac
}

normalize_platform() {
  local value="$1"
  if [[ ! "$value" =~ ^[a-z0-9]+-[a-z0-9_]+$ ]]; then
    fail "平台格式无效: $value，正确格式示例: linux-amd64"
  fi
  printf '%s\n' "$value"
}

VERSION="$(git -C "$ROOT_DIR" describe --tags --always --dirty 2>/dev/null || printf 'dev')"
COMMIT="$(git -C "$ROOT_DIR" rev-parse --short HEAD 2>/dev/null || printf 'none')"
BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

PLATFORMS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --platform)
      [[ $# -ge 2 ]] || fail "--platform 后面需要跟平台值"
      PLATFORMS+=("$(normalize_platform "$2")")
      shift 2
      ;;
    --output)
      [[ $# -ge 2 ]] || fail "--output 后面需要跟目录路径"
      OUTPUT_ROOT="$2"
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

need_command go "没有检测到 go，请先安装 Go 1.26+。"

if [[ ${#PLATFORMS[@]} -eq 0 ]]; then
  PLATFORMS=("${DEFAULT_PLATFORMS[@]}")
fi

mkdir -p "$OUTPUT_ROOT"

for platform in "${PLATFORMS[@]}"; do
  GOOS="${platform%%-*}"
  GOARCH="${platform#*-}"
  OUT_DIR="${OUTPUT_ROOT}/${platform}"
  OUT_FILE="${OUT_DIR}/$(platform_binary_name "$platform")"

  mkdir -p "$OUT_DIR"
  info "构建 ${platform} -> ${OUT_FILE}"
  (
    cd "$ROOT_DIR"
    CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" \
      go build \
      -ldflags="-s -w -X 'main.Version=${VERSION}' -X 'main.Commit=${COMMIT}' -X 'main.BuildDate=${BUILD_DATE}'" \
      -o "$OUT_FILE" \
      ./cmd/server
  )

  if [[ "$platform" != windows-* ]]; then
    chmod +x "$OUT_FILE"
  fi
done

info "二进制构建完成，输出目录: $OUTPUT_ROOT"
