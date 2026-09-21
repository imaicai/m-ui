#!/usr/bin/env bash
set -Eeuo pipefail

PURGE=false
[[ "${1:-}" == "--purge" ]] && PURGE=true

[[ ${EUID:-$(id -u)} -eq 0 ]] || { echo "请使用 root 用户执行"; exit 1; }

SERVICE="/etc/systemd/system/m-ui.service"
CERT_HOME="$(sed -n 's/^Environment=MUI_CERT_HOME=//p' "$SERVICE" 2>/dev/null | tail -n1 || true)"
[[ -n "$CERT_HOME" ]] || CERT_HOME="/root"
CERT_DIR="$CERT_HOME/minijet/ssh/m-ui"

systemctl disable --now m-ui 2>/dev/null || true
rm -f /etc/systemd/system/m-ui.service
rm -f /usr/local/bin/m-ui /usr/local/bin/m-ui-server /usr/local/bin/m-ui-ctl
rm -rf /opt/m-ui-src
systemctl daemon-reload

if $PURGE; then
  systemctl stop xray 2>/dev/null || true
  rm -rf /etc/m-ui
  rm -rf "$CERT_DIR"
  rm -f /usr/local/etc/xray/config.json
  rm -f /etc/sysctl.d/99-m-ui-bbr.conf /etc/modules-load.d/m-ui-bbr.conf
  echo "m-ui 程序及所有 m-ui 配置已彻底删除。"
  echo "Xray 服务已停止；Xray Core 本体仍保留，未删除二进制文件。"
else
  echo "m-ui 程序已卸载。"
  echo "为防止误删，/etc/m-ui 数据、$CERT_DIR 证书以及 Xray Core 均已保留。"
  echo "如需彻底删除全部 m-ui 配置，请执行在线卸载脚本并添加 --purge 参数。"
fi
