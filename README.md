# m-ui

**m-ui** 是一个面向新手的轻量 Xray 管理面板，作者 **MiniJet**。

设计目标只有三个：**简单、稳定、易用**。不把大量底层参数直接暴露给新手，默认使用一套经过约束的 VLESS + TCP + TLS + XTLS Vision 配置。

## 主要功能

- Go 前后端一体化，单个服务运行
- 内嵌 Web UI，无 Node.js / npm 依赖
- 管理员登录与登录失败保护
- 真实 CPU / 内存 / 磁盘 / 网络状态
- 一键创建 VLESS + TCP + TLS + XTLS Vision 节点
- UUID 自动生成、监听端口自动选择
- 创建/修改节点前自动执行 Xray 配置校验
- 节点新增、停用、删除后自动重载 Xray
- 自动生成 VLESS 节点链接
- 订阅链接自动同步已启用节点
- 默认 TLS 证书路径：`/maicai/ssh/fullchain.pem`
- 默认 TLS 私钥路径：`/maicai/ssh/privkey.pem`
- 适合与 1Panel 共存，不占用 80/443；默认面板端口为 `54321`
- 数据存放于 `/etc/m-ui/state.json`，重复运行安装脚本会保留数据

## 一键安装

推荐 Ubuntu / Debian / Rocky Linux / AlmaLinux，并要求系统支持 systemd。

```bash
bash <(wget -qO- https://raw.githubusercontent.com/imaicai/m-ui/main/install.sh)
```

也可以使用 curl：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/imaicai/m-ui/main/install.sh)
```

安装脚本会自动：

1. 安装必要依赖；
2. 使用 XTLS 官方安装脚本安装/更新 Xray Core；
3. 拉取本仓库源码；
4. 在服务器本地编译 m-ui；
5. 创建 systemd 服务；
6. 首次安装生成管理员密码；
7. 启动面板并尝试放行面板端口。

### 自定义安装参数

```bash
MUI_PORT=3001 MUI_ADMIN_USER=admin MUI_ADMIN_PASS='YourStrongPassword' \
  bash <(wget -qO- https://raw.githubusercontent.com/imaicai/m-ui/main/install.sh)
```

如果不提供 `MUI_ADMIN_PASS`，首次安装会自动生成随机密码并在终端显示。

## 证书

默认读取：

```text
/maicai/ssh/fullchain.pem
/maicai/ssh/privkey.pem
```

请先将有效域名证书放入以上位置，再到 **系统设置** 填写节点域名。证书不存在时，一键添加节点会停止并显示明确错误，不会写入无效节点。

## 常用命令

```bash
m-ui-ctl status
m-ui-ctl restart
m-ui-ctl logs
m-ui-ctl update
m-ui-ctl xray
```

## 目录

```text
/etc/m-ui/state.json                 # m-ui 数据
/usr/local/bin/m-ui                  # 主程序
/usr/local/bin/m-ui-ctl              # 控制命令
/etc/systemd/system/m-ui.service     # systemd 服务
/usr/local/bin/xray                  # Xray Core
/usr/local/etc/xray/config.json      # m-ui 生成的 Xray 配置
/maicai/ssh/fullchain.pem            # TLS 证书
/maicai/ssh/privkey.pem              # TLS 私钥
```

## 开发

需要 Go 1.18+：

```bash
go test ./...
go build -o m-ui .
```

首次本地初始化：

```bash
MUI_DATA=./state.json ./m-ui --init --user admin --password '12345678'
MUI_DATA=./state.json MUI_ADDR=127.0.0.1:54321 ./m-ui
```

## 说明

m-ui 调用 Xray Core 作为代理核心。Xray Core 及其官方安装脚本属于各自项目，其许可证和版权归对应项目所有。

## License

MIT License

Author: **MiniJet**
