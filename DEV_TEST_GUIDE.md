# 开发环境测试指南

## 使用官方 Codex CLI 获取 Tokens

### 方法 1: 从官方 CLI 提取 Tokens（最简单）

1. **安装官方 Codex CLI**
   ```bash
   # 使用 npm
   npm install -g @openai/codex-cli

   # 或使用 Homebrew (macOS)
   brew install openai/tap/codex-cli
   ```

2. **登录授权**
   ```bash
   codex auth login
   ```

   这会：
   - 打开浏览器
   - 跳转到 OpenAI OAuth 页面（使用官方 Client ID）
   - 完成授权
   - Tokens 自动保存到本地

3. **查看保存的 Tokens**
   ```bash
   # macOS/Linux
   cat ~/.config/codex-cli/credentials.json

   # Windows
   type %APPDATA%\codex-cli\credentials.json
   ```

   示例输出：
   ```json
   {
     "access_token": "sk-proj-...",
     "refresh_token": "...",
     "expires_at": "2025-11-15T12:00:00Z",
     "scope": "openai profile email offline_access"
   }
   ```

4. **复制 Tokens 信息**
   - Access Token
   - Refresh Token
   - 过期时间

### 方法 2: 使用 Codex CLI 的配置文件路径

查找配置文件位置：

```bash
# macOS/Linux
find ~ -name "credentials.json" 2>/dev/null | grep codex

# 或直接查看默认位置
ls -la ~/.config/codex-cli/
ls -la ~/.codex/
```

### 方法 3: 抓包获取 Tokens（高级）

如果无法直接访问配置文件，可以使用抓包工具：

1. **设置代理**
   ```bash
   # 使用 mitmproxy
   mitmproxy -p 8888

   # 配置系统代理为 localhost:8888
   ```

2. **运行 Codex CLI 授权**
   ```bash
   codex auth login
   ```

3. **在 mitmproxy 中查找**
   - 查找发往 `https://auth.openai.com/oauth/token` 的请求
   - 查看响应中的 `access_token` 和 `refresh_token`

## 将 Tokens 导入到 QuotaLane

### 选项 A: 使用数据库直接插入（需要加密）

由于 QuotaLane 使用 AES-256-GCM 加密存储 tokens，我们需要先加密：

```sql
-- 注意：这只是示例，实际需要使用 QuotaLane 的加密服务
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
    'My Codex Account',
    'From official Codex CLI',
    'codex-cli',
    'https://api.openai.com',
    'ENCRYPTED_ACCESS_TOKEN',  -- 需要加密
    'ENCRYPTED_REFRESH_TOKEN', -- 需要加密
    '2025-11-15 12:00:00',
    0,
    0,
    100,
    'active',
    NOW(),
    NOW()
);
```

### 选项 B: 创建测试用的导入脚本（推荐）

创建一个 Go 脚本来正确加密和导入 tokens：

```go
// scripts/import_codex_tokens.go
package main

import (
    "context"
    "flag"
    "fmt"
    "time"

    "QuotaLane/internal/data"
    "QuotaLane/pkg/crypto"
)

func main() {
    var (
        name         = flag.String("name", "", "Account name")
        accessToken  = flag.String("access", "", "Access token")
        refreshToken = flag.String("refresh", "", "Refresh token")
        expiresIn    = flag.Int("expires", 3600, "Expires in seconds")
    )
    flag.Parse()

    if *name == "" || *accessToken == "" || *refreshToken == "" {
        fmt.Println("Usage: go run import_codex_tokens.go -name='My Account' -access='...' -refresh='...'")
        return
    }

    // 初始化加密服务
    cryptoService := crypto.NewAESGCMCrypto("your-encryption-key")

    // 加密 tokens
    accessEncrypted, _ := cryptoService.Encrypt(*accessToken)
    refreshEncrypted, _ := cryptoService.Encrypt(*refreshToken)

    // 创建账户
    account := &data.Account{
        Name:                  *name,
        Provider:              data.ProviderCodexCLI,
        BaseAPI:               "https://api.openai.com",
        AccessTokenEncrypted:  accessEncrypted,
        RefreshTokenEncrypted: refreshEncrypted,
        TokenExpiresAt:        time.Now().Add(time.Duration(*expiresIn) * time.Second),
        HealthScore:           100,
        Status:                data.StatusActive,
    }

    // 保存到数据库
    // repo.CreateAccount(context.Background(), account)

    fmt.Printf("Account created: %+v\n", account)
}
```

使用方法：
```bash
cd QuotaLane
go run scripts/import_codex_tokens.go \
  -name="My Codex Account" \
  -access="sk-proj-..." \
  -refresh="..." \
  -expires=3600
```

### 选项 C: 创建临时测试 API（最简单）

在开发环境中，可以临时添加一个不验证 OAuth 的测试端点：

```bash
curl -X POST http://localhost:8000/v1/accounts/openai/import-tokens-dev \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Test Codex Account",
    "access_token": "sk-proj-...",
    "refresh_token": "...",
    "expires_in": 3600
  }'
```

## 为什么浏览器直接打开授权链接会失败？

### OpenAI 的安全检测机制

```
┌─────────────────────────────────────────────┐
│  OpenAI OAuth 服务器                        │
│                                             │
│  接收到授权请求                             │
│  ↓                                          │
│  检查 Client ID: app_EMoamEEZ73f0CkXaXp7... │
│  ↓                                          │
│  检查请求来源:                              │
│    • User-Agent: Chrome/Safari/Firefox ❌  │
│    •   期望: Codex CLI Official Client ✅  │
│    • Referer: 浏览器直接访问 ❌            │
│    •   期望: 来自 Codex CLI 应用 ✅        │
│  ↓                                          │
│  安全检查失败 → 返回 unknown_error          │
└─────────────────────────────────────────────┘
```

### 官方 Codex CLI 的授权流程

```
┌─────────────────────────────────────────────┐
│  用户运行: codex auth login                │
└──────────────┬──────────────────────────────┘
               ↓
┌─────────────────────────────────────────────┐
│  Codex CLI 本地应用                         │
│  • 生成 PKCE 参数                           │
│  • 启动本地 HTTP 服务器 (localhost:1455)   │
│  • 使用官方 User-Agent 打开浏览器           │
└──────────────┬──────────────────────────────┘
               ↓
┌─────────────────────────────────────────────┐
│  浏览器访问 OpenAI OAuth                    │
│  • User-Agent: Codex CLI Official ✅       │
│  • 通过安全检查                             │
│  • 用户登录并授权                           │
└──────────────┬──────────────────────────────┘
               ↓
┌─────────────────────────────────────────────┐
│  重定向回 localhost:1455/auth/callback     │
│  • Codex CLI 本地服务器接收 code           │
│  • 使用 code_verifier 交换 tokens          │
│  • 保存到本地配置文件                       │
└─────────────────────────────────────────────┘
```

## 推荐的开发流程

1. **使用官方 Codex CLI 授权** → 获取真实的 tokens
2. **提取 tokens** → 从配置文件或抓包获取
3. **导入到 QuotaLane** → 使用脚本或直接数据库操作
4. **测试 Token 刷新** → 验证自动刷新功能
5. **测试定时任务** → 验证 5 分钟定时刷新

这样可以：
- ✅ 绕过 OAuth Client ID 限制
- ✅ 获得真实有效的 tokens
- ✅ 测试完整的 token 生命周期管理
- ✅ 验证加密存储功能
- ✅ 测试自动刷新机制

## 总结

**不要直接在浏览器中打开生成的授权链接**，因为：

1. OpenAI 检测到不是官方 Codex CLI 应用
2. Client ID 有使用限制
3. 安全检测会拒绝请求

**正确的做法**：

- 使用官方 Codex CLI 完成授权
- 提取获得的 tokens
- 导入到 QuotaLane 系统
- 测试 token 管理功能
