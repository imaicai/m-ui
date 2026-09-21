# M-Ui

**最简单的代理面板**

M-Ui 是一个面向个人服务器的轻量级 Xray 管理面板，重点是安装简单、界面直观、节点创建快速，并尽量减少日常维护中不必要的复杂设置。

项目地址：https://github.com/imaicai/m-ui

## 主要优势

- **Go 单服务运行**：前后端一体化，Web UI 直接内嵌，无需 Node.js / npm 运行环境。
- **安装和更新简单**：一条命令即可安装或更新，已有账号、节点、订阅和面板设置会保留。
- **默认使用 REALITY + Vision**：新建节点采用 VLESS + RAW + REALITY + XTLS Vision。
- **自动生成节点参数**：公网 IP、国家代码、节点名、UUID、端口、REALITY 密钥、Short ID 自动处理。
- **创建前真实校验**：节点创建和关键配置修改前，会使用当前 Xray 执行配置自检。
- **节点变更自动生效**：新增、编辑、启停或删除节点后自动重载 Xray。
- **订阅按需分发**：默认订阅自动包含全部节点；自定义订阅可单独选择需要分发的节点。
- **轻量状态监控**：仪表盘显示 Xray 状态、节点数量、CPU、内存、磁盘、网络速率与面板运行时间。
- **独立管理入口**：支持随机面板 URL 根路径，降低管理页被直接扫描的概率。
- **适合与 1Panel 共存**：默认使用独立端口，不要求占用 80 / 443。

## 节点管理

新建节点默认配置：

```text
VLESS + RAW + REALITY + XTLS Vision
```

系统会自动完成：

1. 识别服务器公网 IP 和两位国家代码；
2. 自动生成 US01、JP01 等节点名称；
3. 自动选择可用端口；
4. 自动生成 UUID；
5. 自动生成 REALITY X25519 密钥与 Short ID；
6. 默认使用 `www.apple.com:443` 作为 REALITY 伪装目标；
7. 调用当前 Xray 校验生成的配置；
8. 校验通过后保存并重载 Xray。

节点页面支持：

- 编辑节点名称、连接地址、端口和 REALITY 伪装域名；
- 启用 / 停用节点；
- 复制节点链接；
- 删除节点；
- 查看每个节点累计上传 / 下载流量；
- 单独清空某个节点流量，或清空全部节点流量。

## 订阅管理

M-Ui 首次初始化会自动创建一个 **默认订阅**。

默认订阅具有以下规则：

- 永远排在订阅列表第一位；
- 永远不能删除；
- 自动包含当前全部节点；
- 后续新建节点会自动加入默认订阅；
- 节点启用、停用、编辑或删除后，订阅内容自动同步。

创建自定义订阅时，系统会先弹出编辑窗口：

- 自动生成随机订阅地址；
- 可以在保存前修改订阅地址后缀；
- 可以选择需要加入该订阅的节点；
- 只有点击“保存并创建”后，订阅才会真正写入系统。

自定义订阅后续仍可修改名称、订阅地址和节点范围。

订阅地址格式：

```text
https://你的域名或IP:端口/sub/随机后缀
```

## 一键安装 / 更新

推荐系统：

- Ubuntu
- Debian
- Rocky Linux
- AlmaLinux

要求系统支持 systemd。

使用 wget：

```bash
bash <(wget -qO- https://raw.githubusercontent.com/imaicai/m-ui/main/install.sh)
```

或使用 curl：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/imaicai/m-ui/main/install.sh)
```

重复执行同一个安装命令即可更新 M-Ui。更新时会保留：

- 登录账号；
- 节点；
- 订阅；
- 面板名称；
- 面板端口；
- 面板 URL 根路径；
- 节点域名；
- 证书路径；
- 已累计的节点流量数据。

## 首次登录

全新安装默认：

```text
用户名：minijet
密码：minijet
默认端口：16800
```

全新安装会自动生成随机管理根路径，例如：

```text
/a8f3k2m9x7qz/
```

安装完成后，终端会直接显示完整访问地址。

登录后建议立即在 **系统设置 → 登录账号** 中修改密码。

## 自定义安装参数

例如自定义端口、用户名和密码：

```bash
MUI_PORT=3001 MUI_ADMIN_USER=customuser MUI_ADMIN_PASS='YourStrongPassword' \
  bash <(wget -qO- https://raw.githubusercontent.com/imaicai/m-ui/main/install.sh)
```

## 系统设置

Web 面板中可调整：

- 面板名称；
- 面板端口；
- 面板 URL 根路径；
- 节点域名；
- TLS 证书路径；
- 登录用户名；
- 登录密码。

设置保存逻辑：

- 没有修改时，“保存设置”保持不可操作；
- 修改普通设置后直接生效；
- 修改面板 URL 根路径后自动切换到新入口；
- 修改用户名或密码后需要重新登录；
- 修改面板端口后会自动重启面板并跳转到新端口。

## SSH 管理菜单

安装完成后，在 SSH 中直接运行：

```bash
m-ui
```

可进行：

- 一键安装 / 更新；
- 完整卸载；
- 修改用户名、密码、端口和根路径；
- 还原面板访问参数；
- 启动、停止、重启服务；
- 查看运行日志；
- 一键启用当前 Linux 内核原生 BBR + FQ；
- 刷新服务器 IP 和当前配置显示。

常用命令：

```bash
m-ui
m-ui-ctl status
m-ui-ctl restart
m-ui-ctl logs
m-ui-ctl update
m-ui-ctl xray
```

查看当前面板根路径：

```bash
/usr/local/bin/m-ui --print-panel-path
```

## 数据与目录

```text
/etc/m-ui/state.json                     # 面板设置、节点、订阅和累计流量
/usr/local/bin/m-ui                      # SSH 交互式管理菜单
/usr/local/bin/m-ui-server               # M-Ui 后端程序
/usr/local/bin/m-ui-ctl                  # 控制命令
/etc/systemd/system/m-ui.service         # systemd 服务
/usr/local/bin/xray                      # Xray Core
/usr/local/etc/xray/config.json          # M-Ui 生成的 Xray 配置
<SSH_HOME>/minijet/ssh/m-ui/fullchain.pem
<SSH_HOME>/minijet/ssh/m-ui/privkey.pem
```

## 开发与测试

```bash
go test ./...
go vet ./...
node --check web/assets/app.js
bash -n install.sh
bash -n m-ui
bash -n m-ui-ctl
bash -n uninstall.sh
```

GitHub Actions 还会安装当前官方 Xray，并使用真实的 `xray run -test` 校验 M-Ui 生成的 REALITY 配置。

## License

MIT License

Author: **MiniJet**
