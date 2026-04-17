# JnmCPA

当前仓库只维护当前这套运行方式：

- Linux 最小部署包
- `install.sh` 一键安装
- 服务端主程序 + 内置管理页
- 默认 `sqlite-store`

不再把旧的多套启动方式当主文档入口。

## 安装

一键安装：

```bash
curl -fsSL https://raw.githubusercontent.com/JnmHub/JnmCPA/main/install.sh | bash
```

常用可选参数：

```bash
curl -fsSL https://raw.githubusercontent.com/JnmHub/JnmCPA/main/install.sh | bash -s -- --skip-systemd
curl -fsSL https://raw.githubusercontent.com/JnmHub/JnmCPA/main/install.sh | bash -s -- --skip-mongo
```

常用环境变量：

```bash
CLIPROXY_REPO_OWNER=JnmHub
CLIPROXY_REPO_NAME=JnmCPA
CLIPROXY_VERSION=cpa
CLIPROXY_INSTALL_BASE_DIR=/root
CLIPROXY_SERVICE_NAME=cliproxyapi
MANAGEMENT_PASSWORD=你的管理密码
```

## 当前部署结构

安装后默认目录类似：

```text
/root/minimal-linux-amd64/
```

主要文件：

- `CLIProxyAPI`
- `config.yaml`
- `static/management.html`
- `install-systemd.sh`
- `install-mongodb.sh`
- `start-cpa.sh`

## 当前管理页入口

当前管理页访问地址：

```text
/lijinmu
```

例如：

```text
http://127.0.0.1:8317/lijinmu
```

## 当前推荐配置

默认推荐使用 SQLite：

```yaml
sqlite-store:
  path: ./data/cliproxy.db
```

认证测试默认模型可配置：

```yaml
auth-probe-models:
  codex: gpt-4.1
```

模型不支持时是否继续轮询可配置：

```yaml
retry-model-not-supported: false
```

上传限制可配置：

```yaml
auth-upload:
  max-json-size-mb: 10
  max-archive-size-mb: 100
  max-archive-entries: 10000
  max-expanded-size-mb: 512
```

## 更新

同一台机器更新，直接重新执行：

```bash
curl -fsSL https://raw.githubusercontent.com/JnmHub/JnmCPA/main/install.sh | bash
```

如果你保留了自己的 `config.yaml`，更新前先确认配置文件路径和 systemd 服务路径。

## 自动发布

仓库已支持自动发布最小部署包到 GitHub Releases。

触发方式：

```bash
git tag cpa
git push origin cpa
```

或者在 GitHub Actions 里手动执行：

- `Release Minimal Packages`

自动发布产物：

- `minimal-linux-amd64.zip`
- `minimal-linux-arm64.zip`

发布流程会自动：

- 构建前端
- 同步 `static/management.html`
- 跑核心 Go 测试
- 生成最小部署包
- 上传到 GitHub Releases

## 开发

本地开发最常用命令：

```bash
MANAGEMENT_PASSWORD=你的管理密码 go run ./cmd/server -config ./config.yaml
```

前端源码目录：

```text
static/Cli-Proxy-API-Management-Center-main
```

打包后运行页文件：

```text
static/management.html
```
