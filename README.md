# M-Ui

**最简单的代理面板**

M-Ui 是一个面向个人服务器的轻量级 Xray 管理面板，重点是安装简单、节点创建快速，并尽量把安全、稳定和性能优化放在后台自动完成。

项目地址：https://github.com/imaicai/m-ui

## v1.5.0 安全稳定性能版

v1.5.0 保持既定节点预设和客户端使用方式不变：

```text
VLESS + RAW + REALITY + XTLS Vision
```

主要升级集中在服务端底层：

- **Xray 节点热更新**：新增、启停、删除或修改运行参数时优先通过 HandlerService 只替换对应 inbound；热更新失败才回退完整重启。
- **面板重启不再等于 Xray 重启**：M-Ui 启动时先比较配置并做健康检查，运行配置一致时不会无条件重启 Xray。
- **完整健康检查**：配置先经过 `xray run -test`；运行后继续检查 Stats API 和所有已启用节点监听端口。
- **RAW TCP KeepAlive**：服务端入站默认启用保守的 TCP KeepAlive，提升长连接、移动网络和 NAT 环境下的稳定性。
- **REALITY fallback 参数去统一指纹**：每个节点使用稳定但不同的限速参数，不再让所有 M-Ui 节点使用完全相同的一组固定值。
- **Xray 最小权限**：REALITY-only 安装默认使用专用 `xray` 系统用户；检测到历史 TLS 节点时优先兼容，不强制破坏现有证书读取权限。
- **Xray 配置权限收紧**：配置文件默认 `0600`；专用 xray 用户场景使用 `root:xray + 0640`。
- **Xray 稳定版本锁定与回滚**：普通重复安装不会盲目追新；“完整更新”才主动刷新 Xray。升级前保存旧二进制，配置校验或启动失败会自动回滚。
- **BBR + FQ + PMTU**：内核支持时自动启用原版 BBR + FQ，并启用 `tcp_mtu_probing=1`；不安装第三方魔改内核。
- **防火墙生命周期同步**：节点改端口或删除后自动尝试回收旧 UFW / firewalld 端口规则。
- **Argon2id 密码存储**：新密码使用 Argon2id；旧版密码仍可登录，并在成功登录后自动升级哈希。
- **CSRF / 同源保护**：管理操作增加 CSRF token、Origin / Sec-Fetch-Site 校验；HTTPS 响应启用 HSTS。
- **首次随机密码**：全新安装不再使用固定 `minijet/minijet` 密码，终端会生成并显示高强度随机密码。
- **订阅地址更高熵**：新建或主动修改的自定义订阅后缀至少 20 位；历史短地址继续兼容，不会升级后失效。

## 主要优势

- **Go 单服务运行**：前后端一体化，Web UI 直接内嵌，无需 Node.js / npm 运行环境。
- **默认 REALITY + Vision**：新建节点采用 VLESS + RAW + REALITY + XTLS Vision。
- **自动生成节点参数**：公网 IP、国家代码、节点名、UUID、端口、REALITY 密钥、Short ID 自动处理。
- **创建前真实校验**：关键配置写入前使用当前 Xray 做真实配置自检。
- **节点变更尽量无感**：优先热更新目标 inbound，避免操作一个节点影响其他节点。
- **订阅按需分发**：默认订阅自动包含全部节点；自定义订阅可选择节点范围。
- **轻量状态监控**：显示 Xray 状态、节点数量、CPU、内存、磁盘、网络速率和面板运行时间。
- **独立管理入口**：支持随机面板 URL 根路径。
- **适合与 1Panel 共存**：默认独立端口，不要求占用 80 / 443。

## 节点管理

新建节点默认配置：

```text
VLESS + RAW + REALITY + XTLS Vision
```

系统自动完成：

1. 识别服务器公网 IP 和两位国家代码；
2. 自动生成 US01、JP01 等节点名称；
3. 自动选择可用高位端口；
4. 自动生成 UUID；
5. 自动生成 REALITY X25519 密钥与 Short ID；
6. 默认使用 `www.apple.com:443` 作为 REALITY 伪装目标；
7. 生成完整 Xray 配置并真实校验；
8. 保存持久配置；
9. 优先通过 HandlerService 热更新目标 inbound；
10. 热更新不可用时自动回退完整 Xray 重载。

节点页面支持编辑名称、连接地址、端口和 REALITY 伪装域名，支持启停、复制、删除以及累计流量查看/清零。

> 节点名称和分享地址本身不影响 Xray inbound 时，不会为了这类纯展示/分享信息变更而重载节点。

## 订阅管理

M-Ui 首次初始化自动创建一个 **默认订阅**。默认订阅始终排在第一位、不能删除，并自动跟踪当前全部节点。

自定义订阅可以单独选择节点。系统自动生成高熵订阅地址；新建或主动修改自定义后缀时要求 20–80 位，仅允许字母、数字、`-` 和 `_`。历史版本已经存在的短订阅地址继续保留。

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

或 curl：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/imaicai/m-ui/main/install.sh)
```

重复执行普通安装命令会更新 M-Ui 程序并保留已有数据，同时优先保持已经锁定且验证过的 Xray 版本。

SSH 菜单中的：

```text
完整更新 m-ui 【前端、后端、脚本、Xray】
```

会显式刷新 Xray 到官方稳定版本；更新前会备份旧二进制，新的 Xray 必须通过现有配置校验和启动检查，否则自动尝试回滚。

更新会保留登录账号、节点、订阅、面板设置、证书路径和累计流量。

## 首次登录

全新安装默认：

```text
用户名：minijet
密码：安装时自动生成的高强度随机密码
默认端口：16800
```

终端会直接显示管理员密码和完整面板地址，请保存首次生成的密码。

全新安装还会自动生成随机管理根路径，例如：

```text
/a8f3k2m9x7qz/
```

## 自定义安装参数

```bash
MUI_PORT=3001 MUI_ADMIN_USER=customuser MUI_ADMIN_PASS='YourStrongPassword' \
  bash <(wget -qO- https://raw.githubusercontent.com/imaicai/m-ui/main/install.sh)
```

高级兼容参数：

```bash
MUI_XRAY_USER=root ...
```

可强制保留 root 运行 Xray；一般 REALITY-only 部署无需设置。历史 TLS 节点会由安装器自动优先采用兼容模式。

## 网络性能策略

M-Ui v1.5.0 只使用保守、可解释的网络参数：

```text
net.core.default_qdisc=fq                  # 内核支持 BBR 时
net.ipv4.tcp_congestion_control=bbr        # 内核支持 BBR 时
net.ipv4.tcp_mtu_probing=1
```

不会自动安装第三方魔改内核，也不会写入大批来源不明的 TCP 参数。

节点 RAW 入站还会启用：

```text
tcpKeepAliveIdle     = 60
tcpKeepAliveInterval = 15
```

## 安全机制

- 管理数据文件：`/etc/m-ui/state.json`，权限 `0600`。
- Xray 配置：root 场景 `0600`；专用 xray 用户场景 `root:xray 0640`。
- 新密码：Argon2id。
- 旧密码：兼容验证，成功登录后无感迁移到 Argon2id。
- Session：HttpOnly + SameSite=Strict。
- 写操作：CSRF token + 同源校验。
- HTTPS：有效证书存在时支持 HTTPS，并为 HTTPS 响应发送 HSTS。
- 登录失败：按来源地址限流并限制失败状态缓存规模。
- 健康接口：只返回 `{"ok":true}`，不公开面板版本。
- Xray systemd：增加 NoNewPrivileges、PrivateTmp、ProtectKernelTunables 等防护。

## 系统设置

Web 面板支持调整：

- 面板名称；
- 面板端口；
- 面板 URL 根路径；
- 节点域名；
- TLS 证书路径；
- 登录用户名；
- 登录密码。

修改普通设置直接生效；修改根路径后自动切换入口；修改账号需要重新登录；修改面板端口会重启面板进程。面板自身重启不会再无条件重启 Xray。

## SSH 管理菜单

安装完成后运行：

```bash
m-ui
```

可进行安装/更新、彻底卸载、面板账号与入口修改、服务控制、日志查看、原版 BBR + FQ + PMTU 加速以及参数刷新。

常用命令：

```bash
m-ui
m-ui-ctl status
m-ui-ctl restart
m-ui-ctl logs
m-ui-ctl update
m-ui-ctl xray
```

## 数据与目录

```text
/etc/m-ui/state.json                                # 面板设置、节点、订阅和累计流量
/etc/m-ui/xray-version.lock                         # 已验证 Xray 版本锁
/usr/local/bin/m-ui                                 # SSH 管理菜单
/usr/local/bin/m-ui-server                          # M-Ui 后端
/usr/local/bin/m-ui-ctl                             # 控制命令
/etc/systemd/system/m-ui.service                    # M-Ui systemd 服务
/etc/systemd/system/xray.service.d/20-m-ui-hardening.conf
/etc/sysctl.d/99-m-ui-network.conf                  # 保守网络优化
/usr/local/bin/xray                                 # Xray Core
/usr/local/etc/xray/config.json                     # M-Ui 生成的 Xray 配置
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

GitHub Actions 会安装当前官方 Xray，并用真实的 `xray run -test` 校验 M-Ui 生成的 REALITY + HandlerService 配置，同时检查前端绑定、Shell 脚本和 v1.5.0 安全性能约束。

## License

MIT License

Author: **MiniJet**
