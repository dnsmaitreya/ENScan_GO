#!/bin/bash
# ENScan_GO 自动登录功能测试脚本

set -e

echo "======================================"
echo "ENScan_GO 自动登录功能测试"
echo "======================================"
echo ""

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 配置文件路径
CONFIG_PATH="$HOME/.claude/config.yaml"

# 检查配置文件是否存在
if [ ! -f "$CONFIG_PATH" ]; then
    echo -e "${RED}错误: 配置文件不存在 $CONFIG_PATH${NC}"
    echo "请先运行: ./enscan -v 生成配置文件"
    exit 1
fi

echo -e "${GREEN}✓${NC} 配置文件存在"

# 检查配置文件版本
VERSION=$(grep "^version:" "$CONFIG_PATH" | awk '{print $2}')
echo "配置文件版本: $VERSION"

if [ "$VERSION" != "0.8" ]; then
    echo -e "${YELLOW}⚠${NC} 配置文件版本不是 0.8，可能需要更新"
fi

# 检查是否启用了自动登录
AUTO_LOGIN_ENABLED=$(grep -A 1 "^auto_login:" "$CONFIG_PATH" | grep "enabled:" | awk '{print $2}')

if [ "$AUTO_LOGIN_ENABLED" = "true" ]; then
    echo -e "${GREEN}✓${NC} 自动登录已启用"

    # 检查账号配置
    USERNAME=$(grep -A 3 "aiqicha:" "$CONFIG_PATH" | grep "username:" | awk '{print $2}' | tr -d "'")
    if [ -n "$USERNAME" ] && [ "$USERNAME" != "''" ]; then
        echo -e "${GREEN}✓${NC} 爱企查账号已配置: $USERNAME"
    else
        echo -e "${RED}✗${NC} 爱企查账号未配置"
        echo "请在 $CONFIG_PATH 中配置 auto_login.aiqicha.username"
        exit 1
    fi

    # 检查密码配置（不显示实际密码）
    PASSWORD=$(grep -A 4 "aiqicha:" "$CONFIG_PATH" | grep "password:" | awk '{print $2}')
    if [ -n "$PASSWORD" ] && [ "$PASSWORD" != "''" ]; then
        echo -e "${GREEN}✓${NC} 爱企查密码已配置"
    else
        echo -e "${RED}✗${NC} 爱企查密码未配置"
        echo "请在 $CONFIG_PATH 中配置 auto_login.aiqicha.password"
        exit 1
    fi
else
    echo -e "${YELLOW}⚠${NC} 自动登录未启用"
    echo "如需启用，请在 $CONFIG_PATH 中设置 auto_login.enabled: true"
fi

echo ""
echo "======================================"
echo "依赖检查"
echo "======================================"
echo ""

# 检查 chromium 是否安装
if command -v chromium &> /dev/null; then
    echo -e "${GREEN}✓${NC} Chromium 已安装: $(chromium --version 2>&1 | head -n1)"
elif command -v chromium-browser &> /dev/null; then
    echo -e "${GREEN}✓${NC} Chromium 已安装: $(chromium-browser --version 2>&1 | head -n1)"
elif command -v google-chrome &> /dev/null; then
    echo -e "${GREEN}✓${NC} Chrome 已安装: $(google-chrome --version 2>&1 | head -n1)"
else
    echo -e "${RED}✗${NC} Chromium/Chrome 未安装"
    echo ""
    echo "安装方法："
    echo "  Ubuntu/Debian: sudo apt-get install chromium"
    echo "  Alpine:        apk add chromium"
    echo "  macOS:         brew install chromium"
    echo ""
    if [ "$AUTO_LOGIN_ENABLED" = "true" ]; then
        echo -e "${YELLOW}警告: 自动登录功能需要 Chromium${NC}"
    fi
fi

echo ""
echo "======================================"
echo "测试建议"
echo "======================================"
echo ""

if [ "$AUTO_LOGIN_ENABLED" = "true" ]; then
    echo "1. 测试自动登录（清空Cookie）:"
    echo "   编辑 $CONFIG_PATH，将 cookies.aiqicha 设为空"
    echo "   运行: ./enscan -n 测试公司 -type aqc"
    echo ""
    echo "2. 测试Cookie失效自动重登录:"
    echo "   编辑 $CONFIG_PATH，将 cookies.aiqicha 设为无效值"
    echo "   运行: ./enscan -n 测试公司 -type aqc"
    echo ""
    echo "3. 查看详细日志:"
    echo "   运行: ./enscan -n 测试公司 -type aqc --debug"
else
    echo "如需测试自动登录功能，请先在配置文件中启用并配置账号密码"
fi

echo ""
echo "======================================"
echo "配置文件示例"
echo "======================================"
echo ""
echo "version: 0.8"
echo "auto_login:"
echo "  enabled: true"
echo "  aiqicha:"
echo "    username: '13800138000'"
echo "    password: 'your_password'"
echo "cookies:"
echo "  aiqicha: ''  # 留空让程序自动获取"
echo ""
