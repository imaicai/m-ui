# m-ui

**m-ui** 是一个面向新手的轻量 Xray 管理面板，作者 **MiniJet**。

目标：**简单、稳定、易用**。普通用户不需要理解大量底层参数；新建节点默认使用 **VLESS + RAW + REALITY + XTLS Vision**，旧版已经创建的 TLS 节点会继续保留，不会在升级时强制转换。

## v1.4.3

- 紧急修复前端公共事件绑定回归：侧边栏菜单、复制等按钮恢复可点击。
- 修复 `bindCommon()` 中单元素选择器 `$()` 被错误用于 `.forEach()` 的问题。
- CI 增加事件绑定断言，防止后续再次出现“页面能显示但菜单/按钮全部失效”。

## v1.4.2

- 顶部横条高度由约 52px 提升到约 70px，页面标题与说明文字按文字底部对齐。
- 侧边栏底部改为 GitHub 图标 + 版本号 + MiniJet；只有 GitHub 图标可点击打开源码。
- 使用用户提供的绿色纸飞机 PNG 作为唯一应用图标，并生成对应多尺寸 favicon.ico。
- 顶部“登录管理员”改为“管理员账号”，左侧新增“重启面板”按钮。
- 系统设置保存区域新增“重启面板”按钮，并明确说明哪些场景通常无需重启、哪些异常场景可手动重启。
- 节点管理添加入口改为文字按钮“+新建节点”。
- 新增受登录鉴权保护的面板重启接口；仅重启 m-ui 面板进程，不主动重启 Xray。

## v1.4.1

- 使用 MiniJet PNG 图标作为网页 favicon、侧边栏应用图标与登录面板图标。
- 登录页改为单一居中白色卡片，图标、品牌、账号输入全部收纳在卡片内。
- 顶部导航高度缩减并改为单行：页面标题 + 页面说明；右侧显示登录管理员与退出图标。
- 仪表盘移除“一键添加节点”卡片，服务器资源扩展为整页宽度。
- 节点管理把添加按钮放到搜索框右侧，新手说明移动到节点列表下方。
- 订阅管理移除顶部统计卡，改为单一订阅使用说明卡片。
- 继续精修绿色 `#00B176` 视觉体系、字号、间距、表格与操作图标。
- TLS 证书默认目录改为当前 SSH 安装用户的 home：`~/minijet/ssh/m-ui/`。root 登录时即为 `/root/minijet/ssh/m-ui/`。

## v1.4.0

- 默认面板端口改为 `16800`（已有安装升级保留当前端口）。
- 默认证书路径位于当前 SSH 安装用户 home：`~/minijet/ssh/m-ui/fullchain.pem` 与 `~/minijet/ssh/m-ui/privkey.pem`。
- 面板同端口支持 HTTP/HTTPS；证书有效且匹配节点域名时 HTTP 自动跳转 HTTPS，证书失效时 HTTP 仍可访问。
- 订阅名称和订阅地址后缀支持编辑。
- 节点自动生成参数尽量避免数字 `4`（真实 IP/手工域名不做篡改）。
- UI 主色统一为 `#00B176`，操作按钮图标化并统一版式。

## v1.3.1

- 修正 SSH 菜单第 3 项：由“只更新 SSH 脚本”改为“完整更新 m-ui”。
- 完整更新会重新拉取源码并更新/编译：后端、内嵌前端、SSH 菜单、控制脚本和 Xray Core。
- 更新会保留现有 `/etc/m-ui/state.json`、账号、节点、订阅、端口和根路径。
- 更新完成后自动重新进入最新版 `m-ui` 菜单，并显示更新后的运行版本。

## v1.3.0

- 系统设置中的“使用建议 / 基础设置 / 证书路径”统一为与订阅列表一致的全内容区宽度。
- “节点域名”填写后，新建节点优先使用该域名作为分享/订阅连接地址；未填写才使用服务器公网 IP。
- 新建 REALITY 节点默认伪装目标改为 `www.apple.com:443`，SNI 为 `www.apple.com`。
- 已有微软伪装节点不会被强制改写；可通过“编辑节点”自行切换为苹果。
- REALITY 服务端显式设置 `minClientVer: 1.8.2`，保持标准 REALITY + Vision，同时兼顾 Mihomo 客户端版本标识。
- m-ui 服务每次启动都会用当前节点数据重新生成并校验 Xray 配置，确保升级后已有节点也立即获得新配置字段。
- 节点支持编辑：名称、连接地址、端口、REALITY 伪装域名；UUID、密钥、Short ID、协议和 Vision 流控保持只读。
- 订阅地址灰色显示区域加宽，长链接视觉上更完整。

## v1.2.1

- 首次安装默认登录用户名：`minijet`
- 首次安装默认登录密码：`minijet`
- 已有安装升级时继续保留原用户名和密码，不会被覆盖。
- 仍支持通过环境变量 `MUI_ADMIN_USER` / `MUI_ADMIN_PASS` 自定义首次安装账号。

## v1.2.0

- SSH 输入 `m-ui` 直接打开交互式管理菜单。
- 后端程序改为 `/usr/local/bin/m-ui-server`，避免与 SSH 菜单命令冲突。
- 菜单支持一键安装、彻底卸载、更新管理脚本、修改用户名/密码/端口/根路径、还原面板、服务控制和日志查看。
- 增加“一键原版 BBR + FQ 加速”，仅使用当前 Linux 内核原生能力，不自动更换内核。
- 增加“刷新IP配置及参数显示”，重新获取公网 IP、国家、面板地址和运行参数，不擅自改写已创建节点。
- “还原面板”只重置登录访问参数，保留节点与订阅数据。

## v1.1.0

- 修复 Xray 26.x 无法识别 `.tmp` 临时配置格式的问题；校验文件现在始终以 `.json` 结尾。
- 新建节点默认升级为 **VLESS + RAW + REALITY + XTLS Vision**。
- 新建 REALITY 节点不要求提前配置域名或 TLS 证书。
- 自动识别服务器公网 IP 与国家代码，节点名自动生成，例如 `US01`、`US02`。
- 新增 **面板 URL 根路径**；新安装自动生成随机路径，已有安装升级后先保持 `/`，可在系统设置中修改。
- TLS 兼容模式默认证书路径改为：
  - `<SSH_HOME>/minijet/ssh/m-ui/fullchain.pem`
  - `<SSH_HOME>/minijet/ssh/m-ui/privkey.pem`
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
- 适合与 1Panel 共存；默认面板端口为 `16800`
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

首次安装默认用户名和密码均为 `minijet`，面板根路径仍自动随机生成，终端会直接显示完整访问地址。重复执行同一安装命令会保留 `/etc/m-ui/state.json` 中的账号、节点、订阅和设置。

### 自定义安装参数

```bash
MUI_PORT=3001 MUI_ADMIN_USER=customuser MUI_ADMIN_PASS='YourStrongPassword' \
  bash <(wget -qO- https://raw.githubusercontent.com/imaicai/m-ui/main/install.sh)
```

## 面板 URL 根路径

全新安装会自动创建类似：

```text
/a8f3k2m9x7qz/
```

因此安装结束后地址可能是：

```text
http://1.2.3.4:16800/a8f3k2m9x7qz/
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

1. 识别服务器公网 IP（用于国家代码与节点名称）；
2. 根据公网 IP 识别两位国家代码；
3. 自动生成 `US01` / `JP01` 等名称，同国家节点自动递增；\n4. 如系统设置已填写节点域名，则使用域名作为连接地址；否则使用公网 IP；
5. 生成 UUID；
6. 选择可用监听端口；
7. 调用 `xray x25519` 生成 REALITY 密钥；
8. 生成随机 Short ID；
9. 写入临时 JSON；
10. 调用当前 Xray 自检；
11. 自检通过后替换正式配置并重启 Xray。

### 已有 TLS 节点

升级不会强制转换已有节点。TLS 节点仍使用系统设置中的域名与证书：

```text
<SSH_HOME>/minijet/ssh/m-ui/fullchain.pem
<SSH_HOME>/minijet/ssh/m-ui/privkey.pem
```

如果服务器没有旧 TLS 节点，节点域名可以留空。

## 登录账号修改

在 **系统设置 → 登录账号** 中可以修改：

- 登录名
- 登录密码

修改登录名或密码时必须填写当前密码；修改成功后当前登录会失效，需要使用新账号重新登录。

## SSH 管理菜单

安装或升级到 v1.2.0 后，SSH 直接输入：

```bash
m-ui
```

显示：

```text
 1. 一键安装 m-ui
 2. 删除卸载 m-ui （包含卸载所有配置）
 3. 完整更新 m-ui 【前端、后端、脚本、Xray】
----------------------------------------------------------------------------------
 4. 变更 m-ui 面板设置 【用户名密码、登录端口、根路径、还原面板】
 5. 关闭、重启 m-ui
----------------------------------------------------------------------------------
 6. 查看 m-ui 运行日志
 7. 一键原版BBR+FQ加速
 8. 刷新IP配置及参数显示
```

其中：

- **删除卸载**：删除 m-ui 程序、节点、订阅、登录数据、m-ui 专属证书目录和 m-ui 生成的 Xray 配置；Xray Core 二进制本身保留，但 Xray 服务会停止。
- **还原面板**：保留节点和订阅，重置为用户名 `admin`、随机新密码、端口 `16800`、随机新根路径。
- **BBR + FQ**：写入 `/etc/sysctl.d/99-m-ui-bbr.conf`，只启用当前内核已经支持的原版 BBR 和 FQ，不自动替换 Linux 内核。
- **刷新IP配置及参数显示**：刷新公网 IP、国家、面板地址、用户名、端口、根路径和 BBR/FQ 状态，仅用于显示，不修改已有节点。

## 常用命令

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

## 目录

```text
/etc/m-ui/state.json                     # m-ui 数据
/usr/local/bin/m-ui                      # SSH 交互式管理菜单
/usr/local/bin/m-ui-server               # m-ui 后端主程序
/usr/local/bin/m-ui-ctl                  # 控制命令
/etc/systemd/system/m-ui.service         # systemd 服务
/usr/local/bin/xray                      # Xray Core
/usr/local/etc/xray/config.json          # m-ui 生成的 Xray 配置
<SSH_HOME>/minijet/ssh/m-ui/fullchain.pem           # TLS 兼容节点证书
<SSH_HOME>/minijet/ssh/m-ui/privkey.pem             # TLS 兼容节点私钥
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

CI 还会安装当前官方 Xray，并实际执行 `xray run -test` 验证 m-ui 生成的 REALITY 配置。

## License

MIT License

Author: **MiniJet**
