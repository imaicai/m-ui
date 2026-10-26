#!/usr/bin/env bash
set -Eeuo pipefail

APP="m-ui"
AUTHOR="MiniJet"
REPO="https://github.com/imaicai/m-ui.git"
RAW="https://raw.githubusercontent.com/imaicai/m-ui/main"
BRANCH="${MUI_BRANCH:-main}"
PANEL_PORT="${MUI_PORT:-16800}"
ADMIN_USER="${MUI_ADMIN_USER:-minijet}"
INSTALL_SRC="/opt/m-ui-src"
SERVER_BIN="/usr/local/bin/m-ui-server"
MENU_BIN="/usr/local/bin/m-ui"
CTL="/usr/local/bin/m-ui-ctl"
DATA_DIR="/etc/m-ui"
DATA_FILE="${DATA_DIR}/state.json"
SERVICE="/etc/systemd/system/m-ui.service"
XRAY_CONFIG="/usr/local/etc/xray/config.json"

CURRENT_UPDATE_PROGRESS=0
CURRENT_UPDATE_STAGE="准备更新"

update_progress(){
  local progress="${1:-0}" stage="${2:-更新中}" message="${3:-}" status="${4:-running}"
  [[ -n "${MUI_UPDATE_STATUS_FILE:-}" && -n "${MUI_UPDATE_TOKEN:-}" ]] || return 0
  CURRENT_UPDATE_PROGRESS="$progress"
  CURRENT_UPDATE_STAGE="$stage"
  local dir tmp
  dir="$(dirname "$MUI_UPDATE_STATUS_FILE")"
  mkdir -p "$dir" 2>/dev/null || return 0
  tmp="${MUI_UPDATE_STATUS_FILE}.tmp"
  printf '{"update_id":"%s","status":"%s","progress":%d,"stage":"%s","message":"%s","updated_at":%d}\n' \
    "$MUI_UPDATE_TOKEN" "$status" "$progress" "$stage" "$message" "$(date +%s)" > "$tmp" || return 0
  chmod 600 "$tmp" 2>/dev/null || { rm -f "$tmp"; return 0; }
  mv -f "$tmp" "$MUI_UPDATE_STATUS_FILE" 2>/dev/null || { rm -f "$tmp"; return 0; }
  return 0
}

on_update_error(){
  local rc=$?
  if [[ -n "${MUI_UPDATE_STATUS_FILE:-}" && -n "${MUI_UPDATE_TOKEN:-}" ]]; then
    update_progress "$CURRENT_UPDATE_PROGRESS" "$CURRENT_UPDATE_STAGE" "更新在当前阶段失败，请查看系统日志后重试" "failed" || true
  fi
  exit "$rc"
}
trap on_update_error ERR

SSH_USER="${SUDO_USER:-}"
if [[ -z "$SSH_USER" || "$SSH_USER" == "root" ]]; then
  SSH_USER="$(logname 2>/dev/null || true)"
fi
[[ -n "$SSH_USER" ]] || SSH_USER="root"
SSH_HOME="$(getent passwd "$SSH_USER" 2>/dev/null | cut -d: -f6)"
[[ -n "$SSH_HOME" ]] || SSH_HOME="/root"
CERT_DIR="$SSH_HOME/minijet/ssh/m-ui"

if [[ -z "${MUI_PORT:-}" && -f "$SERVICE" ]]; then
  EXISTING_ADDR="$(sed -n 's/^Environment=MUI_ADDR=//p' "$SERVICE" | tail -n1)"
  EXISTING_PORT="${EXISTING_ADDR##*:}"
  if [[ "$EXISTING_PORT" =~ ^[0-9]+$ ]]; then
    PANEL_PORT="$EXISTING_PORT"
  fi
fi

BLUE='\033[0;34m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; NC='\033[0m'
info(){ echo -e "${BLUE}[m-ui]${NC} $*"; }
ok(){ echo -e "${GREEN}[m-ui]${NC} $*"; }
warn(){ echo -e "${YELLOW}[m-ui]${NC} $*"; }
err(){ echo -e "${RED}[m-ui]${NC} $*" >&2; }
die(){ err "$*"; update_progress "$CURRENT_UPDATE_PROGRESS" "$CURRENT_UPDATE_STAGE" "更新失败，请查看系统日志后重试" "failed" || true; exit 1; }

curl_fetch(){
  local url="$1" output="$2"
  curl -fL --connect-timeout 8 --max-time 180 --retry 5 --retry-delay 2 --retry-connrefused "$url" -o "$output"
}

[[ ${EUID:-$(id -u)} -eq 0 ]] || die "请使用 root 用户执行安装。"
command -v systemctl >/dev/null 2>&1 || die "当前系统不支持 systemd，暂不支持安装。"
[[ "$PANEL_PORT" =~ ^[0-9]+$ ]] && (( PANEL_PORT >= 1024 && PANEL_PORT <= 65535 )) || die "MUI_PORT 必须是 1024-65535 的端口。"

install_packages(){
  local missing=0 cmd
  for cmd in curl wget git unzip go jq; do
    command -v "$cmd" >/dev/null 2>&1 || { missing=1; break; }
  done
  if (( missing == 0 )); then
    info "基础依赖已完整，跳过系统软件源更新。"
    return 0
  fi

  warn "检测到缺失基础依赖，正在通过系统软件源补齐…"
  if command -v apt-get >/dev/null 2>&1; then
    export DEBIAN_FRONTEND=noninteractive
    local n
    for n in 1 2 3; do
      if apt-get update -y >/dev/null 2>&1 && apt-get install -y ca-certificates curl wget git unzip golang-go jq >/dev/null 2>&1; then
        return 0
      fi
      sleep $((n*2))
    done
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y ca-certificates curl wget git unzip golang jq >/dev/null 2>&1 && return 0
  elif command -v yum >/dev/null 2>&1; then
    yum install -y ca-certificates curl wget git unzip golang jq >/dev/null 2>&1 && return 0
  else
    die "暂不支持当前包管理器。建议使用 Ubuntu / Debian / Rocky / AlmaLinux。"
  fi
  die "基础依赖安装失败，请检查系统软件源网络。"
}

ensure_xray_user(){
  if ! getent group xray >/dev/null 2>&1; then groupadd --system xray; fi
  if ! id xray >/dev/null 2>&1; then
    local shell
    shell="$(command -v nologin 2>/dev/null || true)"
    [[ -n "$shell" ]] || shell="/usr/sbin/nologin"
    useradd --system --gid xray --no-create-home --home-dir /nonexistent --shell "$shell" xray
  fi
}

detect_xray_run_user(){
  if [[ -n "${MUI_XRAY_USER:-}" ]]; then printf '%s' "$MUI_XRAY_USER"; return; fi
  if [[ -f "$DATA_FILE" ]]; then
    if ! jq -e . "$DATA_FILE" >/dev/null 2>&1; then printf 'root'; return; fi
    if jq -e '.nodes[]? | select((.security // "tls") != "reality")' "$DATA_FILE" >/dev/null 2>&1; then
      printf 'root'; return
    fi
  fi
  printf 'xray'
}

apply_network_profile(){
  info "应用安全稳定网络参数…"
  mkdir -p /etc/sysctl.d /etc/modules-load.d
  local bbr=0 available
  modprobe tcp_bbr 2>/dev/null || true
  available="$(sysctl -n net.ipv4.tcp_available_congestion_control 2>/dev/null || true)"
  if [[ " $available " == *" bbr "* ]]; then
    bbr=1
    sysctl -w net.core.default_qdisc=fq >/dev/null 2>&1 || bbr=0
    (( bbr == 1 )) && sysctl -w net.ipv4.tcp_congestion_control=bbr >/dev/null 2>&1 || true
  fi
  sysctl -w net.ipv4.tcp_mtu_probing=1 >/dev/null 2>&1 || true
  {
    echo '# m-ui v1.5.0: conservative network performance profile'
    echo 'net.ipv4.tcp_mtu_probing=1'
    if (( bbr == 1 )); then
      echo 'net.core.default_qdisc=fq'
      echo 'net.ipv4.tcp_congestion_control=bbr'
      printf 'tcp_bbr\n' > /etc/modules-load.d/m-ui-bbr.conf
    fi
  } > /etc/sysctl.d/99-m-ui-network.conf
  if (( bbr == 1 )); then
    ok "已启用 BBR + FQ，并开启 PMTU 黑洞自动探测。"
  else
    warn "当前内核未提供 BBR；已保留现有拥塞控制并开启 PMTU 黑洞自动探测。"
  fi
}

configure_xray_run_user(){
  mkdir -p /etc/systemd/system/xray.service.d
  cat > /etc/systemd/system/xray.service.d/10-m-ui-user.conf <<EOF
[Service]
User=$XRAY_RUN_USER
Group=$XRAY_RUN_USER
EOF
  systemctl daemon-reload
}

harden_xray_service(){
  mkdir -p /etc/systemd/system/xray.service.d
  cat > /etc/systemd/system/xray.service.d/20-m-ui-hardening.conf <<'EOF'
[Service]
NoNewPrivileges=true
PrivateTmp=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
ProtectHostname=true
RestrictSUIDSGID=true
LockPersonality=true
RestrictRealtime=true
UMask=0027
LimitNOFILE=1048576
EOF
  systemctl daemon-reload
}

xray_version(){
  [[ -x /usr/local/bin/xray ]] || return 0
  /usr/local/bin/xray version 2>/dev/null | awk 'NR==1{v=$2; sub(/^v/,"",v); print "v" v}'
}

update_progress 10 "准备环境" "检查系统依赖与运行环境"
info "检查基础环境…"
install_packages
command -v go >/dev/null 2>&1 || die "Go 安装失败。"

update_progress 18 "系统准备" "基础依赖完成，正在应用网络配置与备份"
apply_network_profile

mkdir -p "$DATA_DIR" "$CERT_DIR" /var/log/m-ui
chmod 700 "$DATA_DIR"
chmod 700 "$SSH_HOME/minijet" "$SSH_HOME/minijet/ssh" "$CERT_DIR" 2>/dev/null || true

# Upgrade compatibility: copy previous default certificate files into the SSH user's m-ui directory.
if [[ ! -f "$CERT_DIR/fullchain.pem" ]]; then
  for src in /root/minijet/ssh/m-ui/fullchain.pem /maicai/ssh/m-ui/fullchain.pem /maicai/ssh/fullchain.pem; do
    [[ -f "$src" ]] && { cp -a "$src" "$CERT_DIR/fullchain.pem"; break; }
  done
fi
if [[ ! -f "$CERT_DIR/privkey.pem" ]]; then
  for src in /root/minijet/ssh/m-ui/privkey.pem /maicai/ssh/m-ui/privkey.pem /maicai/ssh/privkey.pem; do
    [[ -f "$src" ]] && { cp -a "$src" "$CERT_DIR/privkey.pem"; break; }
  done
fi

if [[ -f "$DATA_FILE" ]]; then
  cp -a "$DATA_FILE" "$DATA_FILE.bak.$(date +%Y%m%d-%H%M%S)"
  info "检测到已有 m-ui 数据，将保留账号、节点和订阅。"
fi

if [[ -f "$XRAY_CONFIG" ]]; then
  mkdir -p "$DATA_DIR/backup"
  cp -a "$XRAY_CONFIG" "$DATA_DIR/backup/xray-config.before-m-ui.$(date +%Y%m%d-%H%M%S).json" || true
fi

update_progress 28 "更新 Xray" "正在备份并更新 Xray Core"
info "安装/校验 Xray Core…"
ensure_xray_user
XRAY_RUN_USER="$(detect_xray_run_user)"
if [[ "$XRAY_RUN_USER" != "xray" && "$XRAY_RUN_USER" != "root" ]]; then
  die "MUI_XRAY_USER 仅支持 xray 或 root，避免配置权限与运行用户不一致。"
fi
XRAY_LOCK_FILE="$DATA_DIR/xray-version.lock"
XRAY_REFRESH="${MUI_XRAY_REFRESH:-0}"
XRAY_SCRIPT="$(mktemp)"
XRAY_BACKUP_DIR="$(mktemp -d)"
[[ -x /usr/local/bin/xray ]] && cp -a /usr/local/bin/xray "$XRAY_BACKUP_DIR/xray" || true
[[ -f /etc/systemd/system/xray.service ]] && cp -a /etc/systemd/system/xray.service "$XRAY_BACKUP_DIR/xray.service" || true
restore_xray(){
  warn "Xray 更新或健康检查失败，正在恢复上一版本…"
  [[ -f "$XRAY_BACKUP_DIR/xray" ]] && install -m 0755 "$XRAY_BACKUP_DIR/xray" /usr/local/bin/xray || true
  [[ -f "$XRAY_BACKUP_DIR/xray.service" ]] && cp -a "$XRAY_BACKUP_DIR/xray.service" /etc/systemd/system/xray.service || true
  systemctl daemon-reload || true
  systemctl restart xray || true
}
XRAY_SCRIPT_AVAILABLE=1
if ! curl_fetch "https://github.com/XTLS/Xray-install/raw/main/install-release.sh" "$XRAY_SCRIPT"; then
  if [[ -x /usr/local/bin/xray ]]; then
    XRAY_SCRIPT_AVAILABLE=0
    warn "暂时无法下载 Xray 更新脚本，将保留当前稳定 Xray 并继续更新面板。"
    update_progress 34 "保留 Xray" "Xray 更新源暂不可用，已保留当前稳定版本并继续"
  else
    die "无法下载 Xray 官方安装脚本，且本机没有可用 Xray。"
  fi
fi

CURRENT_XRAY="$(xray_version || true)"
LOCKED_XRAY=""
[[ -f "$XRAY_LOCK_FILE" ]] && LOCKED_XRAY="$(tr -d '[:space:]' < "$XRAY_LOCK_FILE")"
CURRENT_XRAY_USER=""
if [[ -f /etc/systemd/system/xray.service.d/10-m-ui-user.conf ]]; then
  CURRENT_XRAY_USER="$(sed -n 's/^[[:space:]]*User[[:space:]]*=[[:space:]]*//p' /etc/systemd/system/xray.service.d/10-m-ui-user.conf 2>/dev/null | tail -n1 | awk '{print $1}' || true)"
elif [[ -f /etc/systemd/system/xray.service ]]; then
  CURRENT_XRAY_USER="$(sed -n 's/^[[:space:]]*User[[:space:]]*=[[:space:]]*//p' /etc/systemd/system/xray.service 2>/dev/null | tail -n1 | awk '{print $1}' || true)"
fi
# XTLS/Xray-install removed custom -u/--install-user support on 2026-10-18.
# Install with the upstream defaults, then enforce m-ui's required runtime user
# through a dedicated systemd drop-in that we control.
XRAY_INSTALL_ARGS=(install)
NEED_XRAY_INSTALL=1
if [[ "$XRAY_REFRESH" != "1" && -n "$LOCKED_XRAY" && "$CURRENT_XRAY" == "$LOCKED_XRAY" && "$CURRENT_XRAY_USER" == "$XRAY_RUN_USER" ]]; then
  NEED_XRAY_INSTALL=0
elif [[ "$XRAY_REFRESH" != "1" && -n "$LOCKED_XRAY" ]]; then
  XRAY_INSTALL_ARGS+=(--version "$LOCKED_XRAY" --force)
else
  XRAY_INSTALL_ARGS+=(--force)
fi
if (( XRAY_SCRIPT_AVAILABLE == 0 )); then
  NEED_XRAY_INSTALL=0
fi

if (( NEED_XRAY_INSTALL == 1 )); then
  info "Xray 运行用户: $XRAY_RUN_USER"
  if ! bash "$XRAY_SCRIPT" "${XRAY_INSTALL_ARGS[@]}" >/dev/null; then
    if [[ -f "$XRAY_BACKUP_DIR/xray" ]]; then
      restore_xray
      warn "Xray 在线更新失败，已恢复上一稳定版本；继续更新 m-ui 面板。"
      update_progress 42 "Xray 已回滚" "Xray 在线更新失败，已恢复稳定版本并继续更新面板"
    else
      rm -f "$XRAY_SCRIPT"; rm -rf "$XRAY_BACKUP_DIR"
      die "Xray 安装失败，且没有可回滚版本。"
    fi
  fi
else
  if (( XRAY_SCRIPT_AVAILABLE == 1 )); then
    info "Xray 已处于锁定稳定版本 $CURRENT_XRAY，本次不追新。"
  fi
fi
rm -f "$XRAY_SCRIPT"
update_progress 46 "校验 Xray" "Xray Core 已更新，正在校验现有配置与服务"
[[ -x /usr/local/bin/xray ]] || { restore_xray; rm -rf "$XRAY_BACKUP_DIR"; die "Xray 安装失败。"; }

configure_xray_run_user
harden_xray_service
if [[ -f "$XRAY_CONFIG" ]]; then
  if [[ "$XRAY_RUN_USER" == "xray" ]]; then
    chown root:xray "$XRAY_CONFIG" 2>/dev/null || true
    chmod 0640 "$XRAY_CONFIG" 2>/dev/null || true
  else
    chmod 0600 "$XRAY_CONFIG" 2>/dev/null || true
  fi
  if ! /usr/local/bin/xray run -test -config "$XRAY_CONFIG" >/dev/null 2>&1; then
    restore_xray; rm -rf "$XRAY_BACKUP_DIR"; die "新 Xray 无法通过现有节点配置校验，已回滚。"
  fi
  if ! systemctl restart xray >/dev/null 2>&1 || ! systemctl is-active --quiet xray; then
    restore_xray; rm -rf "$XRAY_BACKUP_DIR"; die "新 Xray 启动健康检查失败，已回滚。"
  fi
fi
INSTALLED_XRAY="$(xray_version || true)"
[[ -n "$INSTALLED_XRAY" ]] && { printf '%s\n' "$INSTALLED_XRAY" > "$XRAY_LOCK_FILE"; chmod 0600 "$XRAY_LOCK_FILE"; }
rm -rf "$XRAY_BACKUP_DIR"

update_progress 60 "获取源码" "正在获取最新 m-ui 源码"
info "获取 m-ui 最新源码…"
rm -rf "$INSTALL_SRC"

fetch_source_tree(){
  local n archive tmpdir srcdir
  for n in 1 2 3; do
    rm -rf "$INSTALL_SRC"
    if git clone --depth 1 --branch "$BRANCH" "$REPO" "$INSTALL_SRC" >/dev/null 2>&1; then
      return 0
    fi
    sleep $((n*2))
  done

  warn "Git 克隆失败，正在改用 GitHub ZIP 源码包…"
  archive="$(mktemp)"
  tmpdir="$(mktemp -d)"
  if ! curl_fetch "https://github.com/imaicai/m-ui/archive/refs/heads/${BRANCH}.zip" "$archive"; then
    rm -f "$archive"; rm -rf "$tmpdir"
    return 1
  fi
  if ! unzip -q "$archive" -d "$tmpdir"; then
    rm -f "$archive"; rm -rf "$tmpdir"
    return 1
  fi
  srcdir="$(find "$tmpdir" -mindepth 1 -maxdepth 1 -type d | head -n1)"
  [[ -n "$srcdir" && -d "$srcdir" ]] || { rm -f "$archive"; rm -rf "$tmpdir"; return 1; }
  mv "$srcdir" "$INSTALL_SRC"
  rm -f "$archive"; rm -rf "$tmpdir"
}
fetch_source_tree || die "多次尝试后仍无法获取 m-ui 源码。"

info "准备编译 m-ui…"
cd "$INSTALL_SRC"
APP_VERSION="$(tr -d '[:space:]' < VERSION)"
[[ -n "$APP_VERSION" ]] || APP_VERSION="1.5.0"
BUILD_LOG="/var/log/m-ui/full-update-build.log"
GOCACHE_DIR="/var/cache/m-ui/go-build"
GOMODCACHE_DIR="/var/cache/m-ui/go-mod"
mkdir -p "$GOCACHE_DIR" "$GOMODCACHE_DIR"
export HOME="${HOME:-/root}"
export GOCACHE="$GOCACHE_DIR"
export GOMODCACHE="$GOMODCACHE_DIR"
: > "$BUILD_LOG"
{
  printf 'm-ui target version: %s\n' "$APP_VERSION"
  printf 'GOCACHE=%s\nGOMODCACHE=%s\n' "$GOCACHE" "$GOMODCACHE"
  go version
} >>"$BUILD_LOG" 2>&1 || true

update_progress 66 "准备编译" "正在使用本地缓存并自动选择可用 Go 模块源"
rm -f /tmp/m-ui-server-build
BUILD_OK=0
GO_PROXIES=(
  "off"
  "https://proxy.golang.org,direct"
  "https://goproxy.cn,direct"
  "https://goproxy.io,direct"
  "direct"
)
for proxy in "${GO_PROXIES[@]}"; do
  printf '\n===== GOPROXY=%s =====\n' "$proxy" >>"$BUILD_LOG"
  if GOMAXPROCS=1 GOGC=50 GOPROXY="$proxy" GOSUMDB=off CGO_ENABLED=0 go build -p=1 -trimpath -ldflags "-s -w -X main.version=${APP_VERSION}" -o /tmp/m-ui-server-build . >>"$BUILD_LOG" 2>&1; then
    if GOSUMDB=off go mod verify >>"$BUILD_LOG" 2>&1; then
      BUILD_OK=1
      break
    fi
    rm -f /tmp/m-ui-server-build
  fi
done

update_progress 72 "编译程序" "Go 依赖与源码校验完成，正在完成低资源编译"
if (( BUILD_OK != 1 )) || [[ ! -s /tmp/m-ui-server-build ]]; then
  update_progress 72 "编译失败" "已尝试本地缓存与多个 Go 模块源，仍无法完成编译" "failed"
  err "m-ui 编译失败，已经依次尝试本地缓存、proxy.golang.org、goproxy.cn、goproxy.io 和直连。最近日志："
  tail -n 40 "$BUILD_LOG" >&2 || true
  exit 1
fi
update_progress 82 "安装程序" "编译完成，正在替换前端、后端与管理脚本"
install -m 0755 /tmp/m-ui-server-build "$SERVER_BIN"
rm -f /tmp/m-ui-server-build
install -m 0755 "$INSTALL_SRC/m-ui" "$MENU_BIN"
install -m 0755 "$INSTALL_SRC/m-ui-ctl" "$CTL"

NEW_PASSWORD=""
if [[ ! -f "$DATA_FILE" ]]; then
  NEW_PASSWORD="${MUI_ADMIN_PASS:-$(od -An -N18 -tx1 /dev/urandom | tr -d '[:space:]')}"
  MUI_INIT_PASS="$NEW_PASSWORD" "$SERVER_BIN" --init --user "$ADMIN_USER"
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
Environment=MUI_CERT_HOME=$SSH_HOME
ExecStart=$SERVER_BIN
Restart=on-failure
RestartSec=3
LimitNOFILE=1048576
NoNewPrivileges=true
PrivateTmp=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
ProtectHostname=true
RestrictSUIDSGID=true
LockPersonality=true
RestrictRealtime=true
UMask=0077

[Install]
WantedBy=multi-user.target
SERVICEEOF

update_progress 90 "重启服务" "程序安装完成，正在重新加载 systemd 并重启面板"
systemctl daemon-reload
systemctl enable m-ui >/dev/null 2>&1
systemctl restart m-ui
sleep 1
systemctl is-active --quiet m-ui || { journalctl -u m-ui --no-pager -n 40; die "m-ui 启动失败。"; }
update_progress 97 "验证服务" "面板已重新启动，正在完成最后检查"

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

PANEL_PATH="$("$SERVER_BIN" --print-panel-path 2>/dev/null || true)"
[[ -n "$PANEL_PATH" ]] || PANEL_PATH="/"

SERVER_IP="$(curl -4 -fsS --max-time 4 https://api.ipify.org 2>/dev/null || true)"
[[ -n "$SERVER_IP" ]] || SERVER_IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
[[ -n "$SERVER_IP" ]] || SERVER_IP="你的服务器IP"

update_progress 99 "完成安装" "安装流程已完成，正在返回最终状态"
ok "m-ui 安装完成"
echo
echo "  面板地址: http://${SERVER_IP}:${PANEL_PORT}${PANEL_PATH}"
echo "  程序名称: m-ui"
echo "  作者:     MiniJet"
echo "  数据文件: $DATA_FILE"
echo "  证书目录: $CERT_DIR/"
if [[ -n "$NEW_PASSWORD" ]]; then
  echo
echo "  管理员账号: $ADMIN_USER"
  echo "  管理员密码: $NEW_PASSWORD"
  warn "首次安装已生成高强度随机密码；请妥善保存上方密码，登录后也可自行修改。"
else
  echo
  info "这是升级安装，原管理员账号和密码保持不变。"
fi
echo
echo "SSH 管理:"
echo "  m-ui    打开交互式管理菜单"
echo
warn "如果云服务器有安全组，请另外放行 TCP ${PANEL_PORT}；后续节点端口也需要允许入站。"
