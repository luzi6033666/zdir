#!/bin/bash
# zdir 一键更新脚本
# 只替换二进制和静态资源，保留 data/ 目录和配置文件不变
# 使用方式：bash update.sh

set -e

# ========== 配置 ==========
INSTALL_DIR="/opt/zdir"                          # zdir 安装目录
GITHUB_REPO="luzi6033666/zdir"                   # GitHub 仓库
ARCH=$(uname -m)                                 # 自动检测架构
SERVICE_NAME="zdir"                              # systemd 服务名
# ==========================

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()    { echo -e "${GREEN}[INFO]${NC} $1"; }
warn()    { echo -e "${YELLOW}[WARN]${NC} $1"; }
error()   { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# 检查安装目录
[ -d "$INSTALL_DIR" ] || error "安装目录 $INSTALL_DIR 不存在，请确认 zdir 已安装"
[ -f "$INSTALL_DIR/zdir" ] || error "未找到 $INSTALL_DIR/zdir 二进制文件"

# 检测架构
case "$ARCH" in
    x86_64)  PKG_ARCH="amd64" ;;
    aarch64) PKG_ARCH="arm64" ;;
    armv7l)  PKG_ARCH="arm"   ;;
    *)       error "不支持的架构：$ARCH" ;;
esac

info "安装目录：$INSTALL_DIR"
info "系统架构：$ARCH -> $PKG_ARCH"

# 获取最新 Release 信息
info "正在获取最新版本信息..."
RELEASE_JSON=$(curl -sf "https://api.github.com/repos/${GITHUB_REPO}/releases/latest") \
    || error "无法获取 Release 信息，请检查网络或仓库地址"

LATEST_TAG=$(echo "$RELEASE_JSON" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\(.*\)".*/\1/')
DOWNLOAD_URL=$(echo "$RELEASE_JSON" | grep "browser_download_url" | grep "${PKG_ARCH}" | sed 's/.*"browser_download_url": *"\(.*\)".*/\1/')
PKG_NAME=$(basename "$DOWNLOAD_URL")

[ -z "$LATEST_TAG" ]    && error "无法解析最新版本号"
[ -z "$DOWNLOAD_URL" ]  && error "未找到适合 $PKG_ARCH 的下载包"

# 检查当前版本（读取二进制内嵌版本）
CURRENT_VERSION=$("$INSTALL_DIR/zdir" version 2>/dev/null || echo "unknown")
info "当前版本：$CURRENT_VERSION"
info "最新版本：$LATEST_TAG"

if [ "$CURRENT_VERSION" = "$LATEST_TAG" ]; then
    warn "当前已是最新版本 ($LATEST_TAG)，无需更新"
    read -p "仍要强制更新？[y/N] " force
    [[ "$force" =~ ^[Yy]$ ]] || { info "已取消"; exit 0; }
fi

# 下载到临时目录
TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

info "正在下载 $PKG_NAME ..."
curl -L --progress-bar "$DOWNLOAD_URL" -o "$TMP_DIR/$PKG_NAME" \
    || error "下载失败，请检查网络"

# 解压，只取需要更新的文件（不解压 data/ config.simple.ini 已有配置）
info "正在解压..."
tar -zxf "$TMP_DIR/$PKG_NAME" -C "$TMP_DIR/" \
    zdir \
    templates/ \
    sql/ \
    sh/ \
    2>/dev/null || error "解压失败"

# 停止服务
if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
    info "停止服务 $SERVICE_NAME ..."
    systemctl stop "$SERVICE_NAME"
    NEED_RESTART=true
else
    warn "服务 $SERVICE_NAME 未在运行，跳过停止步骤"
    NEED_RESTART=false
fi

# 备份旧二进制
BACKUP="$INSTALL_DIR/zdir.bak"
cp "$INSTALL_DIR/zdir" "$BACKUP"
info "旧二进制已备份到 $BACKUP"

# 替换文件
info "正在替换文件..."
cp "$TMP_DIR/zdir"          "$INSTALL_DIR/zdir"
chmod +x                    "$INSTALL_DIR/zdir"
cp -r "$TMP_DIR/templates/" "$INSTALL_DIR/templates/"
cp -r "$TMP_DIR/sql/"       "$INSTALL_DIR/sql/"
cp -r "$TMP_DIR/sh/"        "$INSTALL_DIR/sh/"

info "文件替换完成，以下文件未改动（保留你的配置和数据）："
echo "    $INSTALL_DIR/data/"
echo "    $INSTALL_DIR/data/config/config.ini"

# 启动服务
if [ "$NEED_RESTART" = true ]; then
    info "启动服务 $SERVICE_NAME ..."
    systemctl start "$SERVICE_NAME"
    sleep 2
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        info "服务启动成功 ✓"
        rm -f "$BACKUP"
    else
        error "服务启动失败，正在回滚..."
        cp "$BACKUP" "$INSTALL_DIR/zdir"
        systemctl start "$SERVICE_NAME"
        error "已回滚到旧版本，请检查日志：journalctl -u $SERVICE_NAME -n 50"
    fi
else
    warn "请手动启动服务：systemctl start $SERVICE_NAME"
fi

info "更新完成！当前版本：$LATEST_TAG"
