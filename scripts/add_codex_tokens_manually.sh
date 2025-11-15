#!/bin/bash
# 手动添加 Codex CLI OAuth Tokens 到账户
# 使用方法: ./add_codex_tokens_manually.sh

set -e

echo "=========================================="
echo "QuotaLane - 手动添加 Codex CLI Tokens"
echo "=========================================="
echo ""

# 检查是否在 Docker 环境中
if [ -f /.dockerenv ]; then
    DB_HOST="mysql"
else
    DB_HOST="${DB_HOST:-localhost}"
fi

DB_PORT="${DB_PORT:-3306}"
DB_NAME="${DB_NAME:-quotalane}"
DB_USER="${DB_USER:-root}"
DB_PASSWORD="${DB_PASSWORD:-root}"

# 输入提示
echo "请提供以下信息（从真实的 Codex CLI 授权中获取）:"
echo ""

read -p "账户名称: " ACCOUNT_NAME
read -p "账户描述 (可选): " ACCOUNT_DESC
read -p "Access Token: " ACCESS_TOKEN
read -p "Refresh Token: " REFRESH_TOKEN
read -p "Token 过期时间 (秒，默认 3600): " EXPIRES_IN
EXPIRES_IN=${EXPIRES_IN:-3600}

# 计算过期时间
EXPIRES_AT=$(date -u -d "+${EXPIRES_IN} seconds" "+%Y-%m-%d %H:%M:%S" 2>/dev/null || date -u -v+${EXPIRES_IN}S "+%Y-%m-%d %H:%M:%S")

echo ""
echo "准备创建账户:"
echo "  名称: $ACCOUNT_NAME"
echo "  描述: $ACCOUNT_DESC"
echo "  过期时间: $EXPIRES_AT UTC"
echo ""

read -p "确认创建? (y/n): " CONFIRM
if [ "$CONFIRM" != "y" ]; then
    echo "已取消"
    exit 0
fi

# 注意：这里需要使用 QuotaLane 的加密服务加密 tokens
# 实际使用时应该通过 API 或直接调用 Go 程序来加密

echo ""
echo "⚠️  注意: 此脚本需要配合 QuotaLane 的加密服务使用"
echo "推荐使用 API 端点或 Go 程序直接创建账户"
echo ""
echo "SQL 示例 (需要先加密 tokens):"
echo ""

cat <<EOF
INSERT INTO api_accounts (
    name,
    description,
    provider,
    base_api,
    access_token_encrypted,
    refresh_token_encrypted,
    token_expires_at,
    rpm_limit,
    tpm_limit,
    health_score,
    status,
    created_at,
    updated_at
) VALUES (
    '${ACCOUNT_NAME}',
    '${ACCOUNT_DESC}',
    'codex-cli',
    'https://api.openai.com',
    'ENCRYPTED_ACCESS_TOKEN_HERE',  -- 需要加密
    'ENCRYPTED_REFRESH_TOKEN_HERE', -- 需要加密
    '${EXPIRES_AT}',
    0,
    0,
    100,
    'active',
    NOW(),
    NOW()
);
EOF

echo ""
echo "=========================================="
echo "建议使用 gRPC/HTTP API 创建账户以确保正确加密"
echo "=========================================="
