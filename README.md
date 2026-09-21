# m-ui

**m-ui** 是一个面向新手的轻量 Xray 管理面板，作者 **MiniJet**。

目标：**简单、稳定、易用**。普通用户不需要理解大量底层参数；新建节点默认使用 **VLESS + RAW + REALITY + XTLS Vision**，旧版已经创建的 TLS 节点会继续保留，不会在升级时强制转换。

## v1.1.0

- 修复 Xray 26.x 无法识别 `.tmp` 临时配置格式的问题；校验文件现在始终以 `.json` 结尾。
- 新建节点默认升级为 **VLESS + RAW + REALITY + XTLS Vision**。
- 新建 REALITY 节点不要求提前配置域名或 TLS 证书。
- 自动识别服务器公网 IP 与国家代码，节点名自动生成，例如 `US01`、`US02`。
- 新增 **面板 URL 根路径**；新安装自动生成随机路径，已有安装升级后先保持 `/`，可在系统设置中修改。
- TLS 兼容模式默认证书路径改为：
  - `/maicai/ssh/m-ui/fullchain.pem`
  - `/maicai/ssh/m-ui/privkey.pem`
- 登录名与密码均可在系统设置中修改；账号信息变化要求输入当前密码。
- 系统设置的“使用建议”移动到页面顶部，不再使用右侧独立边栏。
- 增加针对当前官方 Xray 的 REALITY 配置集成测试。

## 主要功能

- Go 前后端一体化，单个服务运行
- 内嵌 Web UI，无 Node.js / npm 运行依赖
- 管理员登录与登录失败保护
- 可配置随机面板 URL 根路径
- 真实 CPU / 内存 / 磁盘 / 网络状态
- 一键创建 VLESS + RAW + REALITY + XTLS Vision 节点
- 自动识别公网 IP 与国家代码
- UUID、端口、REALITY X25519 密钥和 Short ID 自动生成
- 创建/修改节点前自动执行 Xray 配置校验
- 节点新增、停用、删除后自动重载 Xray
- 自动生成 VLESS 分享链接
- 订阅链接自动同步已启用节点
- 旧 TLS 节点向后兼容
- 适合与 1Panel 共存；默认面板端口为 `54321`
- 数据存放于 `/etc/m-ui/state.json`，重复运行安装脚本会保留数据

## 一键安装 / 更新

推荐 Ubuntu / Debian / Rocky Linux / AlmaLinux，并要求系统支持 systemd。

```bash
bash <(wget -qO- https://raw.githubusercontent.com/imaicai/m-ui/main/install.sh)
```

也可以：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/imaicai/m-ui/main/install.sh)
```

首次安装会生成随机管理员密码和随机面板根路径，终端会直接显示完整访问地址。重复执行同一安装命令会保留 `/etc/m-ui/state.json` 中的账号、节点、订阅和设置。

### 自定义安装参数

```bash
MUI_PORT=3001 MUI_ADMIN_USER=admin MUI_ADMIN_PASS='YourStrongPassword' \
  bash <(wget -qO- https://raw.githubusercontent.com/imaicai/m-ui/main/install.sh)
```

## 面板 URL 根路径

全新安装会自动创建类似：

```text
/a8f3k2m9x7qz/
```

因此安装结束后地址可能是：

```text
http://1.2.3.4:54321/a8f3k2m9x7qz/
```

已有 v1.0.0 安装升级到 v1.1.0 时，为避免升级后突然找不到登录页，初始仍保持 `/`；登录后可在 **系统设置 → 面板 URL 根路径** 修改。

订阅地址仍使用独立随机 Token，不受管理面板根路径影响。

## 节点策略

### 新建节点

默认：

```text
VLESS + RAW + REALITY + XTLS Vision
```

系统自动处理：

1. 识别服务器公网 IP；
2. 根据公网 IP 识别两位国家代码；
3. 自动生成 `US01` / `JP01` 等名称，同国家节点自动递增；
4. 生成 UUID；
5. 选择可用监听端口；
6. 调用 `xray x25519` 生成 REALITY 密钥；
7. 生成随机 Short ID；
8. 写入临时 JSON；
9. 调用当前 Xray 自检；
10. 自检通过后替换正式配置并重启 Xray。

### 已有 TLS 节点

升级不会强制转换已有节点。TLS 节点仍使用系统设置中的域名与证书：

```text
/maicai/ssh/m-ui/fullchain.pem
/maicai/ssh/m-ui/privkey.pem
```

如果服务器没有旧 TLS 节点，节点域名可以留空。

## 登录账号修改

在 **系统设置 → 登录账号** 中可以修改：

- 登录名
- 登录密码

修改登录名或密码时必须填写当前密码；修改成功后当前登录会失效，需要使用新账号重新登录。

## 常用命令

```bash
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

## 目录

```text
/etc/m-ui/state.json                     # m-ui 数据
/usr/local/bin/m-ui                      # 主程序
/usr/local/bin/m-ui-ctl                  # 控制命令
/etc/systemd/system/m-ui.service         # systemd 服务
/usr/local/bin/xray                      # Xray Core
/usr/local/etc/xray/config.json          # m-ui 生成的 Xray 配置
/maicai/ssh/m-ui/fullchain.pem           # TLS 兼容节点证书
/maicai/ssh/m-ui/privkey.pem             # TLS 兼容节点私钥
```

## 开发与测试

```bash
go test ./...
go vet ./...
node --check web/assets/app.js
bash -n install.sh
bash -n m-ui-ctl
bash -n uninstall.sh
```

CI 还会安装当前官方 Xray，并实际执行 `xray run -test` 验证 m-ui 生成的 REALITY 配置。

## License

MIT License

Author: **MiniJet**
