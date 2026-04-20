#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEFAULT_BRANCH="main"
DEFAULT_TAG="cpa"

BRANCH="${RELEASE_BRANCH:-${DEFAULT_BRANCH}}"
TAG_NAME="${RELEASE_TAG:-${DEFAULT_TAG}}"
REMOTE_NAME="${RELEASE_REMOTE:-origin}"
COMMIT_MESSAGE="${RELEASE_COMMIT_MESSAGE:-}"
SKIP_CHECKS=0
ALLOW_EMPTY_COMMIT=0

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
  ./push-and-release.sh
  ./push-and-release.sh --message "你的提交说明"
  ./push-and-release.sh --tag cpa-2026-04-20
  ./push-and-release.sh --branch main --tag cpa --skip-checks

默认行为:
  1. git add -A
  2. 工作区有变化时自动提交
  3. push 到指定分支（默认 main）
  4. 强制更新并推送发布 tag（默认 cpa）
  5. 由 GitHub Actions 的 release-minimal.yml 自动发布最小部署包

可选参数:
  --message <text>       指定提交信息
  --branch <name>        指定推送分支，默认 main
  --tag <name>           指定发布 tag，默认 cpa
  --remote <name>        指定 git remote，默认 origin
  --skip-checks          跳过本地检查
  --allow-empty-commit   即使没有变更也创建空提交
  -h, --help             显示帮助

可选环境变量:
  RELEASE_BRANCH
  RELEASE_TAG
  RELEASE_REMOTE
  RELEASE_COMMIT_MESSAGE
EOF
}

need_command() {
  local cmd="$1"
  local hint="$2"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    fail "$hint"
  fi
}

default_commit_message() {
  printf 'Release %s\n\nTriggered by push-and-release.sh on %s.' \
    "$TAG_NAME" \
    "$(date '+%Y-%m-%d %H:%M:%S %z')"
}

current_branch() {
  git -C "$ROOT_DIR" rev-parse --abbrev-ref HEAD
}

has_changes() {
  [[ -n "$(git -C "$ROOT_DIR" status --porcelain)" ]]
}

run_checks() {
  info "运行本地检查"
  (
    cd "$ROOT_DIR"
    TOOlCHAIN_GO_BIN="$(go env GOROOT)/bin/go"
    mkdir -p .cache/go-build .cache/go-mod .cache/go-path
    GOTOOLCHAIN=local \
    GOCACHE="$ROOT_DIR/.cache/go-build" \
    GOMODCACHE="$ROOT_DIR/.cache/go-mod" \
    GOPATH="$ROOT_DIR/.cache/go-path" \
    "$TOOlCHAIN_GO_BIN" test ./internal/api/handlers/management ./internal/authdeletestats ./internal/config
    GOTOOLCHAIN=local \
    GOCACHE="$ROOT_DIR/.cache/go-build" \
    GOMODCACHE="$ROOT_DIR/.cache/go-mod" \
    GOPATH="$ROOT_DIR/.cache/go-path" \
    "$TOOlCHAIN_GO_BIN" test ./sdk/cliproxy/auth
    GOTOOLCHAIN=local \
    GOCACHE="$ROOT_DIR/.cache/go-build" \
    GOMODCACHE="$ROOT_DIR/.cache/go-mod" \
    GOPATH="$ROOT_DIR/.cache/go-path" \
    "$TOOlCHAIN_GO_BIN" build ./cmd/server
    if [[ -d "$ROOT_DIR/static/Cli-Proxy-API-Management-Center-main" ]]; then
      (
        cd "$ROOT_DIR/static/Cli-Proxy-API-Management-Center-main"
        npm run type-check
        npm run build
      )
      cp -f "$ROOT_DIR/static/Cli-Proxy-API-Management-Center-main/dist/index.html" "$ROOT_DIR/static/management.html"
    fi
  )
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --message)
      [[ $# -ge 2 ]] || fail "--message 后面需要跟提交信息"
      COMMIT_MESSAGE="$2"
      shift 2
      ;;
    --branch)
      [[ $# -ge 2 ]] || fail "--branch 后面需要跟分支名"
      BRANCH="$2"
      shift 2
      ;;
    --tag)
      [[ $# -ge 2 ]] || fail "--tag 后面需要跟 tag 名称"
      TAG_NAME="$2"
      shift 2
      ;;
    --remote)
      [[ $# -ge 2 ]] || fail "--remote 后面需要跟 remote 名称"
      REMOTE_NAME="$2"
      shift 2
      ;;
    --skip-checks)
      SKIP_CHECKS=1
      shift
      ;;
    --allow-empty-commit)
      ALLOW_EMPTY_COMMIT=1
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
  need_command git "没有检测到 git，请先安装 git。"
  need_command go "没有检测到 go，请先安装 Go 1.26+。"

  cd "$ROOT_DIR"

  local branch_now
  branch_now="$(current_branch)"
  if [[ "$branch_now" != "$BRANCH" ]]; then
    fail "当前分支是 ${branch_now}，不是 ${BRANCH}。请切到目标分支后再执行。"
  fi

  if [[ "$SKIP_CHECKS" != "1" ]]; then
    run_checks
  else
    warn "已跳过本地检查"
  fi

  info "暂存变更"
  git add -A

  if has_changes; then
    if [[ -z "$COMMIT_MESSAGE" ]]; then
      COMMIT_MESSAGE="$(default_commit_message)"
    fi
    info "创建提交"
    git commit -m "$COMMIT_MESSAGE"
  elif [[ "$ALLOW_EMPTY_COMMIT" == "1" ]]; then
    if [[ -z "$COMMIT_MESSAGE" ]]; then
      COMMIT_MESSAGE="$(default_commit_message)"
    fi
    info "创建空提交"
    git commit --allow-empty -m "$COMMIT_MESSAGE"
  else
    warn "工作区没有变更，跳过提交"
  fi

  info "推送分支 ${BRANCH} -> ${REMOTE_NAME}"
  git push "$REMOTE_NAME" "$BRANCH"

  info "更新发布 tag: ${TAG_NAME}"
  git tag -f "$TAG_NAME"

  info "推送发布 tag: ${TAG_NAME}"
  git push "$REMOTE_NAME" -f "$TAG_NAME"

  printf '\n'
  info "已完成 push + tag 发布触发"
  printf '分支: %s/%s\n' "$REMOTE_NAME" "$BRANCH"
  printf 'Tag: %s\n' "$TAG_NAME"
  printf 'Release 页面: https://github.com/JnmHub/JnmCPA/releases/tag/%s\n' "$TAG_NAME"
  printf 'Actions 页面: https://github.com/JnmHub/JnmCPA/actions/workflows/release-minimal.yml\n'
}

main "$@"
