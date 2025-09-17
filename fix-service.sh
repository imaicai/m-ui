#!/bin/bash

# m-ui服务修复脚本
# 用于修复已安装但无法运行面板和xray的问题

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

# 检查root权限
[[ $EUID -ne 0 ]] && echo -e "${red}错误：${plain} 必须使用root用户运行此脚本！\n" && exit 1

echo -e "${green}开始修复m-ui服务...${plain}"

# 停止现有服务
echo -e "${yellow}停止现有x-ui服务...${plain}"
systemctl stop x-ui

# 检查/usr/local/x-ui目录是否存在
if [[ ! -d "/usr/local/x-ui/" ]]; then
    echo -e "${red}错误：${plain}未找到/usr/local/x-ui/目录，请先运行安装脚本"
    exit 1
fi

# 进入x-ui目录
cd /usr/local/x-ui/

# 设置正确的权限
echo -e "${yellow}设置文件权限...${plain}"
chmod +x x-ui bin/xray-linux-amd64

# 检查并修复服务文件
echo -e "${yellow}检查服务文件...${plain}"
if [[ -f "/etc/systemd/system/x-ui.service" ]]; then
    # 备份现有服务文件
    cp /etc/systemd/system/x-ui.service /etc/systemd/system/x-ui.service.backup
    echo -e "${yellow}已备份现有服务文件到 /etc/systemd/system/x-ui.service.backup${plain}"
fi

# 创建新的服务文件
echo -e "${yellow}创建新的服务文件...${plain}"
cat > /etc/systemd/system/x-ui.service << EOF
[Unit]
Description=m-ui Service
After=network.target
Wants=network.target

[Service]
Type=simple
WorkingDirectory=/usr/local/x-ui/
ExecStart=/usr/local/x-ui/x-ui
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF

# 重新加载systemd
echo -e "${yellow}重新加载systemd配置...${plain}"
systemctl daemon-reload

# 启用并启动服务
echo -e "${yellow}启用并启动x-ui服务...${plain}"
systemctl enable x-ui
systemctl start x-ui

# 检查服务状态
echo -e "${yellow}检查服务状态...${plain}"
sleep 3
systemctl status x-ui --no-pager

echo -e "${green}修复完成！如果服务仍然无法正常运行，请检查以下几点：${plain}"
echo -e "1. 检查端口是否被占用：netstat -tulpn | grep :54321"
echo -e "2. 检查防火墙设置是否允许端口54321通过"
echo -e "3. 检查日志：x-ui log"
echo -e "4. 重新安装：x-ui uninstall 然后重新运行安装脚本"