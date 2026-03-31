目录说明
========

这个目录是最轻量的 Linux amd64 部署包，包含：

1. CLIProxyAPI
   Linux amd64 二进制

2. config.yaml
   默认配置：
   - host = 0.0.0.0
   - remote-management.allow-remote = true
   - remote-management.disable-control-panel = false
   - mongo-store 指向 127.0.0.1:27017

3. install-mongodb.sh
   自动安装 MongoDB 8.0，并默认只监听 127.0.0.1

4. start-cpa.sh
   启动 / 停止 / 查看状态 / 查看日志

5. static/
   管理页面静态文件（management.html / management-enhancer.js）

6. install-systemd.sh
   把 /root/minimal-linux-amd64 注册成 systemd 服务


推荐部署步骤
===========

1. 上传整个目录到服务器
2. 修改 config.yaml
   最少改这两个：
   - remote-management.secret-key
   - api-keys
3. 安装 MongoDB
   sudo ./install-mongodb.sh
4. 启动 CPA
   ./start-cpa.sh start --daemon

如果你要改成 systemd 守护：
   sudo ./install-systemd.sh


常用命令
========

前台启动：
  ./start-cpa.sh start

后台启动：
  ./start-cpa.sh start --daemon

查看状态：
  ./start-cpa.sh status

查看日志：
  ./start-cpa.sh logs

停止：
  ./start-cpa.sh stop

注册 systemd：
  sudo ./install-systemd.sh

查看 systemd 状态：
  systemctl status cliproxyapi

查看 systemd 日志：
  journalctl -u cliproxyapi -f
