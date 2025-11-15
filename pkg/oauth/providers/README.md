# OAuth Providers 扩展指南

本目录包含各 AI 平台的 OAuth Provider 实现。

## 已实现的 Providers

- **Claude Official** (`claude.go`) - Claude Code OAuth PKCE Flow
  - PKCE: 32 字节 base64url
  - 特殊参数: `code=true`
  
- **Codex CLI** (`codex.go`) - OpenAI Codex CLI OAuth PKCE Flow
  - PKCE: 64 字节 hex
  - ID Token 解析: 提取 ChatGPT Account ID
  - Refresh Token 回退逻辑

## 如何添加新的 OAuth Provider

### 步骤 1: 实现 `OAuthProvider` 接口

创建新文件 `pkg/oauth/providers/[platform].go`：

```go
package providers

import (
    "context"
    "QuotaLane/internal/data"
    "QuotaLane/pkg/oauth"
    "github.com/go-kratos/kratos/v2/log"
)

type [Platform]Provider struct {
    logger *log.Helper
}

func New[Platform]Provider(logger log.Logger) *[Platform]Provider {
    return &[Platform]Provider{
        logger: log.NewHelper(logger),
    }
}

// 实现 OAuthProvider 接口的所有方法
func (p *[Platform]Provider) GenerateAuthURL(ctx context.Context, params *oauth.OAuthParams) (*oauth.OAuthURLResponse, error) {
    // TODO: 实现授权 URL 生成
}

func (p *[Platform]Provider) ExchangeCode(ctx context.Context, code string, session *oauth.OAuthSession) (*oauth.TokenResponse, error) {
    // TODO: 实现授权码交换
}

func (p *[Platform]Provider) RefreshToken(ctx context.Context, refreshToken string, metadata *oauth.AccountMetadata) (*oauth.TokenResponse, error) {
    // TODO: 实现 Token 刷新
}

func (p *[Platform]Provider) ValidateToken(ctx context.Context, token string, metadata *oauth.AccountMetadata) error {
    // TODO: 实现 Token 验证
}

func (p *[Platform]Provider) ProviderType() data.AccountProvider {
    return data.Provider[Platform]
}
```

### 步骤 2: 注册到 OAuthManager

在 `cmd/QuotaLane/wire.go` 中注册 Provider：

```go
func NewOAuthManager(redis *redis.Client, logger log.Logger) *oauth.OAuthManager {
    mgr := oauth.NewOAuthManager(redis, logger)
    mgr.RegisterProvider(providers.NewClaudeProvider(logger))
    mgr.RegisterProvider(providers.NewCodexProvider(logger))
    mgr.RegisterProvider(providers.New[Platform]Provider(logger))  // 新增
    return mgr
}
```

### 步骤 3: 添加 Proto 枚举

在 `api/v1/account.proto` 中添加枚举值：

```protobuf
enum AccountProvider {
    CLAUDE_OFFICIAL = 1;
    CODEX_CLI = 2;
    [PLATFORM] = X;  // 新增
}
```

运行 `make proto` 生成代码。

### 步骤 4: 更新数据库枚举

在 `internal/data/account.go` 中添加常量：

```go
const (
    ProviderClaudeOfficial AccountProvider = "claude-official"
    ProviderCodexCLI      AccountProvider = "codex-cli"
    Provider[Platform]    AccountProvider = "[platform-name]"  // 新增
)
```

执行数据库迁移（如需）。

### 步骤 5: 编写测试

创建测试文件 `pkg/oauth/providers/[platform]_test.go`：

```go
package providers

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestGenerate[Platform]AuthURL(t *testing.T) {
    // TODO: 测试授权 URL 生成
}

func TestExchange[Platform]Code(t *testing.T) {
    // TODO: 测试授权码交换（使用 Mock HTTP 服务器）
}

func TestRefresh[Platform]Token(t *testing.T) {
    // TODO: 测试 Token 刷新
}
```

确保测试覆盖率 > 80%。

## PKCE Flow vs Device Flow

### PKCE Flow（Claude, Codex）

1. 生成 code_verifier 和 code_challenge
2. 用户访问授权 URL 完成授权
3. 后端使用 authorization_code + code_verifier 交换 Token

### Device Flow（Gemini, Droid - 预留）

1. 后端调用 `/device/code` 获取 device_code 和 user_code
2. 用户访问 verification_uri 输入 user_code
3. 后端轮询 `/device/token` 获取 Token

## 测试要求

- 使用 `httptest.NewServer` 创建 Mock OAuth 服务器
- 测试成功场景和错误场景（4xx、5xx、网络超时）
- 测试代理配置（HTTP/SOCKS5）
- 测试 Token 验证和解析逻辑
- 单元测试覆盖率 > 80%

## 参考实现

- `claude.go` - 标准 PKCE Flow 实现
- `codex.go` - ID Token 解析和 Refresh Token 回退逻辑
- `../util/pkce.go` - PKCE 工具函数
- `../util/proxy.go` - 代理配置工具
