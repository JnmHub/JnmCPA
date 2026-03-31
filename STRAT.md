# 唯一启动方式

当前项目只保留一种启动/部署方式：

- 上传并使用 [dist/minimal-linux-amd64](/Volumes/Jnm/Code/golang/CLIProxyAPI/dist/minimal-linux-amd64)

不再保留其它 Docker、本地源码直接启动方案。
根目录只保留“生成最小包”和“远程安装最小包”的辅助脚本。

## 1. 部署目录内容

[dist/minimal-linux-amd64](/Volumes/Jnm/Code/golang/CLIProxyAPI/dist/minimal-linux-amd64) 里包含：

- [CLIProxyAPI](/Volumes/Jnm/Code/golang/CLIProxyAPI/dist/minimal-linux-amd64/CLIProxyAPI)
- [config.yaml](/Volumes/Jnm/Code/golang/CLIProxyAPI/dist/minimal-linux-amd64/config.yaml)
- [install-mongodb.sh](/Volumes/Jnm/Code/golang/CLIProxyAPI/dist/minimal-linux-amd64/install-mongodb.sh)
- [start-cpa.sh](/Volumes/Jnm/Code/golang/CLIProxyAPI/dist/minimal-linux-amd64/start-cpa.sh)
- [install-systemd.sh](/Volumes/Jnm/Code/golang/CLIProxyAPI/dist/minimal-linux-amd64/install-systemd.sh)
- [README.txt](/Volumes/Jnm/Code/golang/CLIProxyAPI/dist/minimal-linux-amd64/README.txt)
- [static/management.html](/Volumes/Jnm/Code/golang/CLIProxyAPI/dist/minimal-linux-amd64/static/management.html)
- [static/management-enhancer.js](/Volumes/Jnm/Code/golang/CLIProxyAPI/dist/minimal-linux-amd64/static/management-enhancer.js)

## 2. 推荐上传包

推荐直接上传这个干净包：

- [dist/minimal-linux-amd64-clean-final.zip](/Volumes/Jnm/Code/golang/CLIProxyAPI/dist/minimal-linux-amd64-clean-final.zip)

如果你要做 `curl | bash`：

- 根入口脚本是 [install.sh](/Volumes/Jnm/Code/golang/CLIProxyAPI/install.sh)
- 它会自动识别 Linux 架构并下载：
  - `minimal-linux-amd64.zip`
  - 或 `minimal-linux-arm64.zip`
- 默认仓库和发布标签是：
  - `JnmHub/JnmCPA`
  - `cpa`

对应资产下载路径示例：

```text
https://github.com/JnmHub/JnmCPA/releases/download/cpa/minimal-linux-amd64.zip
https://github.com/JnmHub/JnmCPA/releases/download/cpa/minimal-linux-arm64.zip
```

上传命令：

```bash
scp dist/minimal-linux-amd64-clean-final.zip root@your-server:/root/
```

服务器上解压：

```bash
ssh root@your-server
cd /root
unzip minimal-linux-amd64-clean-final.zip
cd minimal-linux-amd64
chmod +x CLIProxyAPI install-mongodb.sh start-cpa.sh install-systemd.sh
```

## 3. 启动前必须修改

编辑：

- `/root/minimal-linux-amd64/config.yaml`

至少改这两个值：

```yaml
remote-management:
  secret-key: "你自己的强密码"

api-keys:
  - "你自己的API Key"
```

默认配置已经是：

```yaml
host: "0.0.0.0"
remote-management.allow-remote: true
remote-management.disable-control-panel: false
mongo-store.uri: "mongodb://127.0.0.1:27017"
mongo-store.database: "cliproxy"
mongo-store.collection: "auth_store"
```

## 4. MongoDB 安装

命令：

```bash
cd /root/minimal-linux-amd64
sudo ./install-mongodb.sh
```

脚本行为：

- 安装 MongoDB 8.0
- 默认只监听 `127.0.0.1`
- 自动 `enable` 和 `restart` `mongod`

查看状态：

```bash
systemctl status mongod
journalctl -u mongod -f
```

## 5. 直接启动

前台启动：

```bash
cd /root/minimal-linux-amd64
./start-cpa.sh start
```

后台启动：

```bash
cd /root/minimal-linux-amd64
./start-cpa.sh start --daemon
```

状态：

```bash
./start-cpa.sh status
```

日志：

```bash
./start-cpa.sh logs
```

停止：

```bash
./start-cpa.sh stop
```

## 6. systemd 守护

注册为 systemd 服务：

```bash
cd /root/minimal-linux-amd64
sudo ./install-systemd.sh
```

默认会把：

- `/root/minimal-linux-amd64`

注册成：

- `cliproxyapi.service`

常用命令：

```bash
systemctl status cliproxyapi
systemctl restart cliproxyapi
systemctl stop cliproxyapi
journalctl -u cliproxyapi -f
```

如果目录不是 `/root/minimal-linux-amd64`：

```bash
CPA_DEPLOY_DIR=/your/path sudo ./install-systemd.sh
```

如果服务名要改：

```bash
CPA_SERVICE_NAME=mycpa sudo ./install-systemd.sh
```

## 7. 最短部署流程

```bash
scp dist/minimal-linux-amd64-clean-final.zip root@your-server:/root/
ssh root@your-server
cd /root
unzip minimal-linux-amd64-clean-final.zip
cd minimal-linux-amd64
chmod +x CLIProxyAPI install-mongodb.sh start-cpa.sh install-systemd.sh
vim config.yaml
sudo ./install-mongodb.sh
sudo ./install-systemd.sh
```

管理页面：

```text
http://你的服务器IP:8317/management.html
```

## 8. 构建辅助脚本

这些脚本不是启动方式，只是用来更新交付物：

- [install.sh](/Volumes/Jnm/Code/golang/CLIProxyAPI/install.sh)
- [prepare-minimal-package.sh](/Volumes/Jnm/Code/golang/CLIProxyAPI/prepare-minimal-package.sh)
- [build-binaries.sh](/Volumes/Jnm/Code/golang/CLIProxyAPI/build-binaries.sh)
- [package-project-zip.sh](/Volumes/Jnm/Code/golang/CLIProxyAPI/package-project-zip.sh)

常用命令：

```bash
./build-binaries.sh --platform linux-amd64
./prepare-minimal-package.sh
./prepare-minimal-package.sh --platform linux-amd64
./package-project-zip.sh
```

如果你要更新最小部署目录，基本动作是：

1. 重新构建 Linux amd64 二进制
2. 重新构建前端 `management.html`
3. 执行 `./prepare-minimal-package.sh`
4. 如有需要，再单独重打其它归档

## 9. 结论

以后只认这一条路径：

- `dist/minimal-linux-amd64/`

所有服务器部署、后台守护、管理端访问，都从这个目录出发。  
`install.sh` 只是帮你把这个最小包拉到服务器并自动安装，不是第二套启动方式。
