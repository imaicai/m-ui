#!/usr/bin/env bash
set -Eeuo pipefail

APP="m-ui"
AUTHOR="MiniJet"
REPO="https://github.com/imaicai/m-ui.git"
RAW="https://raw.githubusercontent.com/imaicai/m-ui/main"
BRANCH="${MUI_BRANCH:-main}"
PANEL_PORT="${MUI_PORT:-54321}"
ADMIN_USER="${MUI_ADMIN_USER:-admin}"
INSTALL_SRC="/opt/m-ui-src"
BIN="/usr/local/bin/m-ui"
CTL="/usr/local/bin/m-ui-ctl"
DATA_DIR="/etc/m-ui"
DATA_FILE="${DATA_DIR}/state.json"
SERVICE="/etc/systemd/system/m-ui.service"
XRAY_CONFIG="/usr/local/etc/xray/config.json"

BLUE='\033[0;34m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; NC='\033[0m'
info(){ echo -e "${BLUE}[m-ui]${NC} $*"; }
ok(){ echo -e "${GREEN}[m-ui]${NC} $*"; }
warn(){ echo -e "${YELLOW}[m-ui]${NC} $*"; }
die(){ echo -e "${RED}[m-ui]${NC} $*" >&2; exit 1; }

[[ ${EUID:-$(id -u)} -eq 0 ]] || die "请使用 root 用户执行安装。"
command -v systemctl >/dev/null 2>&1 || die "当前系统不支持 systemd，暂不支持安装。"
[[ "$PANEL_PORT" =~ ^[0-9]+$ ]] && (( PANEL_PORT >= 1024 && PANEL_PORT <= 65535 )) || die "MUI_PORT 必须是 1024-65535 的端口。"

install_packages(){
  if command -v apt-get >/dev/null 2>&1; then
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -y >/dev/null
    apt-get install -y ca-certificates curl wget git unzip golang-go >/dev/null
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y ca-certificates curl wget git unzip golang >/dev/null
  elif command -v yum >/dev/null 2>&1; then
    yum install -y ca-certificates curl wget git unzip golang >/dev/null
  else
    die "暂不支持当前包管理器。建议使用 Ubuntu / Debian / Rocky / AlmaLinux。"
  fi
}

info "检查基础环境…"
install_packages
command -v go >/dev/null 2>&1 || die "Go 安装失败。"

mkdir -p "$DATA_DIR" /maicai/ssh /var/log/m-ui
chmod 700 "$DATA_DIR"
chmod 755 /maicai /maicai/ssh 2>/dev/null || true

if [[ -f "$DATA_FILE" ]]; then
  cp -a "$DATA_FILE" "$DATA_FILE.bak.$(date +%Y%m%d-%H%M%S)"
  info "检测到已有 m-ui 数据，将保留账号、节点和订阅。"
fi

if [[ -f "$XRAY_CONFIG" ]]; then
  mkdir -p "$DATA_DIR/backup"
  cp -a "$XRAY_CONFIG" "$DATA_DIR/backup/xray-config.before-m-ui.$(date +%Y%m%d-%H%M%S).json" || true
fi

info "安装/更新 Xray Core…"
XRAY_SCRIPT="$(mktemp)"
curl -fsSL --retry 3 "https://github.com/XTLS/Xray-install/raw/main/install-release.sh" -o "$XRAY_SCRIPT" || die "无法下载 Xray 官方安装脚本。"
bash "$XRAY_SCRIPT" install -u root >/dev/null
rm -f "$XRAY_SCRIPT"
[[ -x /usr/local/bin/xray ]] || die "Xray 安装失败。"

info "获取 m-ui 最新源码…"
rm -rf "$INSTALL_SRC"
git clone --depth 1 --branch "$BRANCH" "$REPO" "$INSTALL_SRC" >/dev/null 2>&1 || die "无法获取 m-ui 源码。"

info "编译 m-ui…"
cd "$INSTALL_SRC"
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=1.0.0" -o /tmp/m-ui-build .
install -m 0755 /tmp/m-ui-build "$BIN"
rm -f /tmp/m-ui-build
install -m 0755 "$INSTALL_SRC/m-ui-ctl" "$CTL"

NEW_PASSWORD=""
if [[ ! -f "$DATA_FILE" ]]; then
  if [[ -n "${MUI_ADMIN_PASS:-}" ]]; then
    NEW_PASSWORD="$MUI_ADMIN_PASS"
  else
    NEW_PASSWORD="$(od -An -N12 -tx1 /dev/urandom | tr -d ' \n')"
    [[ ${#NEW_PASSWORD} -ge 12 ]] || NEW_PASSWORD="MiniJet$(date +%s)Aa!"
  fi
  "$BIN" --init --user "$ADMIN_USER" --password "$NEW_PASSWORD"
  chmod 600 "$DATA_FILE"
fi

cat > "$SERVICE" <<SERVICEEOF
[Unit]
Description=m-ui Proxy Management Panel by MiniJet
After=network-online.target xray.service
Wants=network-online.target

[Service]
Type=simple
User=root
Environment=MUI_DATA=$DATA_FILE
Environment=MUI_ADDR=0.0.0.0:$PANEL_PORT
Environment=MUI_XRAY_BIN=/usr/local/bin/xray
Environment=MUI_XRAY_CONFIG=$XRAY_CONFIG
ExecStart=$BIN
Restart=on-failure
RestartSec=3
LimitNOFILE=1048576
NoNewPrivileges=false

[Install]
WantedBy=multi-user.target
SERVICEEOF

systemctl daemon-reload
systemctl enable m-ui >/dev/null 2>&1
systemctl restart m-ui
sleep 1
systemctl is-active --quiet m-ui || { journalctl -u m-ui --no-pager -n 40; die "m-ui 启动失败。"; }

open_port(){
  local port="$1"
  if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -qi 'Status: active'; then
    ufw allow "${port}/tcp" >/dev/null 2>&1 || true
  elif command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
    firewall-cmd --permanent --add-port="${port}/tcp" >/dev/null 2>&1 || true
    firewall-cmd --reload >/dev/null 2>&1 || true
  fi
}
open_port "$PANEL_PORT"

SERVER_IP="$(curl -4 -fsS --max-time 4 https://api.ipify.org 2>/dev/null || true)"
[[ -n "$SERVER_IP" ]] || SERVER_IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
[[ -n "$SERVER_IP" ]] || SERVER_IP="你的服务器IP"

ok "m-ui 安装完成"
echo
echo "  面板地址: http://${SERVER_IP}:${PANEL_PORT}/"
echo "  程序名称: m-ui"
echo "  作者:     MiniJet"
echo "  数据文件: $DATA_FILE"
echo "  证书目录: /maicai/ssh/"
if [[ -n "$NEW_PASSWORD" ]]; then
  echo
echo "  管理员账号: $ADMIN_USER"
  echo "  管理员密码: $NEW_PASSWORD"
  warn "请立即保存以上密码，登录后可在系统设置中修改。"
else
  echo
  info "这是升级安装，原管理员账号和密码保持不变。"
fi
echo
echo "常用命令:"
echo "  m-ui-ctl status    查看状态"
echo "  m-ui-ctl restart   重启面板"
echo "  m-ui-ctl logs      查看日志"
echo "  m-ui-ctl update    更新程序"
echo
warn "如果云服务器有安全组，请另外放行 TCP ${PANEL_PORT}；后续节点端口也需要允许入站。"
