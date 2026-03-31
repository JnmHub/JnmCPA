# 命令总览

这个文件汇总当前项目里可直接使用的启动、构建、打包命令。

## 0. 命令索引

### 0.0 远程一键安装

```bash
curl -fsSL https://raw.githubusercontent.com/JnmHub/JnmCPA/main/install.sh | bash
```

如果要安装指定版本：

```bash
curl -fsSL https://raw.githubusercontent.com/1546079656/CLIProxyAPI/main/install.sh | CLIPROXY_VERSION=v1.0.0 bash
```

说明：
- 默认从当前二开仓库 `1546079656/CLIProxyAPI` 下载
- 只有你显式设置 `CLIPROXY_REPO_OWNER` / `CLIPROXY_REPO_NAME` 时才会改到别的仓库

### 0.0.1 服务器一键安装（先上传 zip）

```bash
scp dist/CLIProxyAPI-*.zip root@your-server:/tmp/
ssh root@your-server
chmod +x server-install.sh
./server-install.sh --package /tmp/CLIProxyAPI-xxxx.zip --management-password '你的密码'
```

### 0.1 `docker compose`

```bash
docker compose up -d
docker compose down
docker compose restart
docker compose ps
docker compose logs -f
docker compose logs -f cli-proxy-api
docker compose logs -f mongo
docker compose config
```

### 0.2 `docker-run-no-compose.sh`

```bash
./docker-run-no-compose.sh
./docker-run-no-compose.sh -h
./docker-run-no-compose.sh --help
CLI_PROXY_BUILD_LOCAL=1 ./docker-run-no-compose.sh
```

### 0.3 `start-local-no-docker.sh`

```bash
./start-local-no-docker.sh start
./start-local-no-docker.sh start --daemon
./start-local-no-docker.sh start --foreground
./start-local-no-docker.sh start --platform linux-amd64
./start-local-no-docker.sh start --platform darwin-arm64
./start-local-no-docker.sh start --binary --platform linux-amd64
./start-local-no-docker.sh start --source
./start-local-no-docker.sh source
./start-local-no-docker.sh source --daemon
./start-local-no-docker.sh stop
./start-local-no-docker.sh status
./start-local-no-docker.sh logs
./start-local-no-docker.sh help
./start-local-no-docker.sh -h
./start-local-no-docker.sh --help
```

### 0.4 `build-binaries.sh`

```bash
./build-binaries.sh
./build-binaries.sh --platform linux-amd64
./build-binaries.sh --platform linux-amd64 --platform darwin-arm64
./build-binaries.sh --platform windows-amd64
./build-binaries.sh --output ./dist/custom-binaries
./build-binaries.sh -h
./build-binaries.sh --help
```

### 0.5 `package-project-zip.sh`

```bash
./package-project-zip.sh
./package-project-zip.sh --output ./dist/custom-name.zip
./package-project-zip.sh --include-config
./package-project-zip.sh -h
./package-project-zip.sh --help
```

### 0.6 `server-install.sh`

```bash
sudo ./server-install.sh --package ./CLIProxyAPI-xxxx.zip
sudo ./server-install.sh --package ./CLIProxyAPI-xxxx.zip --management-password '你的密码'
sudo ./server-install.sh --package-url https://example.com/CLIProxyAPI.zip
sudo ./server-install.sh --no-start --package ./CLIProxyAPI-xxxx.zip
sudo ./server-install.sh -h
sudo ./server-install.sh --help
```

### 0.7 旧脚本

```bash
./docker-build.sh
./docker-build.sh --with-usage
```

Windows PowerShell:

```powershell
./docker-build.ps1
```

## 1. Docker Compose

适合：
- 一条命令启动 `CLIProxyAPI + MongoDB`
- 本机已经安装 Docker / Docker Desktop

可用命令：

```bash
docker compose up -d
docker compose down
docker compose restart
docker compose ps
docker compose logs -f
docker compose logs -f cli-proxy-api
docker compose logs -f mongo
docker compose config
```

常用环境变量：

```bash
export CLI_PROXY_PORT=8317
export CLI_PROXY_CONFIG_PATH=./config.yaml
export CLI_PROXY_AUTH_PATH=./auths
export CLI_PROXY_LOG_PATH=./logs
export CLI_PROXY_MONGO_PORT=27017
export MONGOSTORE_URI=mongodb://mongo:27017
export MONGOSTORE_DATABASE=cliproxy
export MONGOSTORE_COLLECTION=auth_store
docker compose up -d
```

相关文件：
- [docker-compose.yml](/Volumes/Jnm/Code/golang/CLIProxyAPI/docker-compose.yml)
- [Dockerfile](/Volumes/Jnm/Code/golang/CLIProxyAPI/Dockerfile)

## 2. Docker 但不依赖 Compose

脚本：
- [docker-run-no-compose.sh](/Volumes/Jnm/Code/golang/CLIProxyAPI/docker-run-no-compose.sh)

可用命令：

```bash
./docker-run-no-compose.sh
./docker-run-no-compose.sh -h
./docker-run-no-compose.sh --help
CLI_PROXY_BUILD_LOCAL=1 ./docker-run-no-compose.sh
```

说明：
- 默认使用远程镜像 `eceasy/cli-proxy-api:latest`
- `CLI_PROXY_BUILD_LOCAL=1` 时会先用本地 `Dockerfile` 构建镜像，再启动
- 只依赖 `docker`，不依赖 `docker compose`

常用环境变量：

```bash
CLI_PROXY_BUILD_LOCAL=1
CLI_PROXY_IMAGE=eceasy/cli-proxy-api:latest
CLI_PROXY_CONFIG_PATH=./config.yaml
CLI_PROXY_AUTH_PATH=./auths
CLI_PROXY_LOG_PATH=./logs
CLI_PROXY_MONGO_IMAGE=mongo:8.0
CLI_PROXY_MONGO_PORT=27017
CLI_PROXY_MONGO_DATA_PATH=./mongo-data
MONGOSTORE_URI=mongodb://cli-proxy-mongo:27017
MONGOSTORE_DATABASE=cliproxy
MONGOSTORE_COLLECTION=auth_store
```

## 3. 完全不依赖 Docker

脚本：
- [start-local-no-docker.sh](/Volumes/Jnm/Code/golang/CLIProxyAPI/start-local-no-docker.sh)

### 3.1 前台启动

```bash
./start-local-no-docker.sh start
./start-local-no-docker.sh start --platform linux-amd64
./start-local-no-docker.sh start --platform darwin-arm64
./start-local-no-docker.sh start --binary --platform linux-amd64
./start-local-no-docker.sh start --source
```

说明：
- 默认优先二进制模式
- 默认平台是 `linux-amd64`
- 如果指定的平台二进制不存在，会尝试自动构建
- 如果指定的平台和当前宿主机不匹配，会直接报错

### 3.2 后台启动

```bash
./start-local-no-docker.sh start --daemon
./start-local-no-docker.sh start --binary --platform darwin-arm64 --daemon
./start-local-no-docker.sh source --daemon
```

说明：
- 后台模式会把应用日志写到 `temp/local-run/cliproxy.log`
- MongoDB 日志写到 `temp/local-run/mongod.log`

### 3.3 源码模式

```bash
./start-local-no-docker.sh source
./start-local-no-docker.sh source --daemon
./start-local-no-docker.sh start --source
```

说明：
- 源码模式使用 `go run ./cmd/server -config ./config.yaml`
- 源码模式一定需要本机安装 Go

### 3.4 状态与停止

```bash
./start-local-no-docker.sh status
./start-local-no-docker.sh logs
./start-local-no-docker.sh stop
./start-local-no-docker.sh help
./start-local-no-docker.sh -h
./start-local-no-docker.sh --help
```

说明：
- `status`：查看 CLIProxyAPI 和 MongoDB 状态
- `logs`：跟随查看后台日志
- `stop`：停止后台应用，并尝试停止本脚本拉起的 MongoDB

### 3.5 本地启动脚本可用环境变量

```bash
CLI_PROXY_CONFIG_PATH=./config.yaml
CLI_PROXY_AUTH_PATH=./auths
CLI_PROXY_LOG_PATH=./logs
CLI_PROXY_MONGO_DATA_PATH=./mongo-data
CLI_PROXY_MONGO_HOST=127.0.0.1
CLI_PROXY_MONGO_PORT=27017
MONGOSTORE_URI=mongodb://127.0.0.1:27017
MONGOSTORE_DATABASE=cliproxy
MONGOSTORE_COLLECTION=auth_store
CLI_PROXY_BINARY_PLATFORM=linux-amd64
CLI_PROXY_BINARY_OUTPUT_ROOT=./dist/binaries
CLI_PROXY_USE_GO_RUN=0
```

### 3.6 MongoDB 检测顺序

本地脚本的 MongoDB 检测顺序：

1. 如果显式设置了 `MONGOSTORE_URI`，优先按这个地址探测
2. 如果没设置，先探测默认 `127.0.0.1:27017`
3. 默认端口没有时，交互式终端下提示你输入端口
4. 还找不到时，尝试本机启动 `mongod`

## 4. 构建全平台二进制

脚本：
- [build-binaries.sh](/Volumes/Jnm/Code/golang/CLIProxyAPI/build-binaries.sh)

可用命令：

```bash
./build-binaries.sh
./build-binaries.sh --platform linux-amd64
./build-binaries.sh --platform linux-amd64 --platform darwin-arm64
./build-binaries.sh --output ./dist/custom-binaries
./build-binaries.sh -h
./build-binaries.sh --help
```

默认构建平台：

```text
linux-amd64
linux-arm64
darwin-amd64
darwin-arm64
windows-amd64
windows-arm64
```

输出目录默认是：

```text
./dist/binaries
```

## 5. 打包项目为 ZIP

脚本：
- [package-project-zip.sh](/Volumes/Jnm/Code/golang/CLIProxyAPI/package-project-zip.sh)

可用命令：

```bash
./package-project-zip.sh
./package-project-zip.sh --output ./dist/custom-name.zip
./package-project-zip.sh --include-config
./package-project-zip.sh -h
./package-project-zip.sh --help
```

说明：
- 默认会先构建全平台二进制，再打包 ZIP
- 默认包含 `config.yaml`
- 默认排除前端源码目录 `static/Cli-Proxy-API-Management-Center-main/`

ZIP 默认排除：

```text
static/Cli-Proxy-API-Management-Center-main/
.git/
dist/         （但会单独带上 dist/binaries 里的全平台二进制）
logs/
mongo-data/
temp/
.env
.env.local
.env.*.local
```

ZIP 默认包含：

```text
config.yaml
static/management.html
binaries/linux-amd64/CLIProxyAPI
binaries/linux-arm64/CLIProxyAPI
binaries/darwin-amd64/CLIProxyAPI
binaries/darwin-arm64/CLIProxyAPI
binaries/windows-amd64/CLIProxyAPI.exe
binaries/windows-arm64/CLIProxyAPI.exe
```

## 6. 服务器安装（Linux / Ubuntu）

脚本：
- [server-install.sh](/Volumes/Jnm/Code/golang/CLIProxyAPI/server-install.sh)

适合：
- 你已经把项目 zip 上传到服务器
- 服务器是 Ubuntu / Debian / RHEL / Rocky / AlmaLinux
- 服务器使用 systemd

最短流程：

```bash
scp dist/CLIProxyAPI-*.zip root@your-server:/tmp/
scp server-install.sh root@your-server:/root/
ssh root@your-server
chmod +x /root/server-install.sh
/root/server-install.sh --package /tmp/CLIProxyAPI-xxxx.zip --management-password '你的密码'
```

可用命令：

```bash
sudo ./server-install.sh --package ./CLIProxyAPI-xxxx.zip
sudo ./server-install.sh --package ./CLIProxyAPI-xxxx.zip --management-password '你的密码'
sudo ./server-install.sh --package-url https://example.com/CLIProxyAPI.zip
sudo ./server-install.sh --mongo-uri mongodb://127.0.0.1:27017
sudo ./server-install.sh --mongo-database cliproxy
sudo ./server-install.sh --mongo-collection auth_store
sudo ./server-install.sh --no-start --package ./CLIProxyAPI-xxxx.zip
sudo ./server-install.sh -h
sudo ./server-install.sh --help
```

脚本行为：
- 自动安装 MongoDB 8.0
- 自动启用并启动 `mongod`
- 从 zip 中提取当前 Linux 平台二进制
- 安装到：
  - 二进制：`/opt/cliproxyapi/bin/cliproxy-api`
  - 配置：`/etc/cliproxyapi/config.yaml`
  - 环境变量：`/etc/cliproxyapi/cliproxyapi.env`
- 自动注册 systemd 服务：
  - `cliproxyapi.service`

安装完成后常用命令：

```bash
systemctl status mongod
systemctl status cliproxyapi
journalctl -u cliproxyapi -f
```

## 7. 旧脚本

说明：
- `docker-build.sh` 是交互式脚本
- 会让你选择“直接用远程镜像启动”或“本地构建后启动”
- `--with-usage` 会在重建前后导出/恢复 usage 统计

## 8. 最常用命令速查

### 启动

```bash
docker compose up -d
./docker-run-no-compose.sh
./start-local-no-docker.sh start
./start-local-no-docker.sh start --daemon
./start-local-no-docker.sh source
```

### 停止 / 查看

```bash
docker compose down
docker compose logs -f
./start-local-no-docker.sh status
./start-local-no-docker.sh logs
./start-local-no-docker.sh stop
```

### 构建 / 打包

```bash
./build-binaries.sh
./build-binaries.sh --platform linux-amd64
./package-project-zip.sh
./package-project-zip.sh --output ./dist/custom-name.zip
./server-install.sh --package ./CLIProxyAPI-xxxx.zip
```

## 9. 备注

- 支持 `curl | bash` 的远程安装脚本是 [install.sh](/Volumes/Jnm/Code/golang/CLIProxyAPI/install.sh)
- 支持“上传 zip 到服务器再一键装 MongoDB + CPA”的脚本是 [server-install.sh](/Volumes/Jnm/Code/golang/CLIProxyAPI/server-install.sh)
- `docker compose` 最省事
- `docker-run-no-compose.sh` 适合没有 compose 的环境
- `start-local-no-docker.sh` 适合本机调试，且默认偏向二进制启动
- `build-binaries.sh` 负责统一构建平台二进制
- `package-project-zip.sh` 默认会带上全平台二进制和 `config.yaml`
