#!/bin/bash

# 版本发布脚本
# 使用方法: ./scripts/release.sh [major|minor|patch]

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 获取当前版本
current_version=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
echo -e "${BLUE}当前版本: ${current_version}${NC}"

# 解析版本号
if [[ $current_version =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
    major=${BASH_REMATCH[1]}
    minor=${BASH_REMATCH[2]}
    patch=${BASH_REMATCH[3]}
else
    echo -e "${RED}无法解析版本号: ${current_version}${NC}"
    exit 1
fi

# 确定版本升级类型
if [ $# -eq 0 ]; then
    echo -e "${YELLOW}请指定版本升级类型:${NC}"
    echo "  major - 主版本号 (v${major}.${minor}.${patch} -> v$((major+1)).0.0)"
    echo "  minor - 次版本号 (v${major}.${minor}.${patch} -> v${major}.$((minor+1)).0)"
    echo "  patch - 修订版本号 (v${major}.${minor}.${patch} -> v${major}.${minor}.$((patch+1)))"
    exit 1
fi

case $1 in
    major)
        new_major=$((major+1))
        new_version="v${new_major}.0.0"
        ;;
    minor)
        new_minor=$((minor+1))
        new_version="v${major}.${new_minor}.0"
        ;;
    patch)
        new_patch=$((patch+1))
        new_version="v${major}.${minor}.${new_patch}"
        ;;
    *)
        echo -e "${RED}无效的版本类型: $1${NC}"
        echo "可用类型: major, minor, patch"
        exit 1
        ;;
esac

echo -e "${GREEN}新版本: ${new_version}${NC}"

# 确认发布
read -p "确认创建版本 ${new_version}? (y/N): " confirm
if [[ $confirm != "y" && $confirm != "Y" ]]; then
    echo -e "${YELLOW}已取消${NC}"
    exit 0
fi

# 创建标签并推送
git tag -a "${new_version}" -m "Release ${new_version}"
git push origin "${new_version}"

echo -e "${GREEN}版本 ${new_version} 已创建并推送，GitHub Actions 将自动开始构建和发布流程${NC}"
echo -e "${BLUE}查看构建进度: https://github.com/imaicai/m-ui/actions${NC}"