#!/usr/bin/env bash
set -e
[[ ${EUID:-$(id -u)} -eq 0 ]] || { echo "请使用 root 用户执行"; exit 1; }
systemctl disable --now m-ui 2>/dev/null || true
rm -f /etc/systemd/system/m-ui.service /usr/local/bin/m-ui /usr/local/bin/m-ui-ctl
rm -rf /opt/m-ui-src
systemctl daemon-reload
echo "m-ui 程序已卸载。"
echo "为防止误删，/etc/m-ui 数据、/maicai/ssh 证书以及 Xray Core 均已保留。"
echo "如确认不再需要数据，可手动删除 /etc/m-ui。"
