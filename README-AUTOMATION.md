# m-ui 自动化发布指南

## CI/CD 自动化流程

本项目配置了完整的自动化构建和发布系统，使用 GitHub Actions 和 GoReleaser。

## 快速开始

### 1. 创建新版本

```bash
# 小版本更新 (修复bug)
./scripts/release.sh patch

# 功能更新 (新增功能)
./scripts/release.sh minor

# 重大更新 (不兼容变更)
./scripts/release.sh major
```

### 2. 查看构建进度

访问 GitHub Actions 页面查看实时构建状态：
```
https://github.com/imaicai/m-ui/actions
```

### 3. 下载发布版本

构建完成后，在 GitHub Releases 页面下载编译好的二进制文件：
```
https://github.com/imaicai/m-ui/releases
```

## 安装使用

用户可以使用以下命令安装最新版本：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/imaicai/m-ui/main/install.sh)
```

## 技术栈

- **GitHub Actions**: 自动化构建和发布
- **GoReleaser**: 专业Go项目发布工具
- **多架构支持**: amd64, arm64
- **语义化版本**: 规范的版本管理

## 文件结构

```
.github/workflows/release.yml    # GitHub Actions 工作流
.goreleaser.yaml                # GoReleaser 配置
scripts/release.sh              # 版本管理脚本
install.sh                      # 用户安装脚本
```

## 注意事项

1. 确保 GitHub Repository 已启用 Actions
2. 首次发布需要手动创建 v0.0.1 标签
3. 需要有写入 Repository 的权限