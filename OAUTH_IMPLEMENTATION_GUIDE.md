# Codex CLI OAuth 实现指南

## 已完成的工作

### ✅ 1. 数据库迁移（Task 1）
- 创建 `migrations/000014_add_oauth_support_for_openai.up.sql`
- 添加字段：`access_token_encrypted`, `refresh_token_encrypted`, `token_expires_at`, `id_token_encrypted`, `organizations`
- 扩展 `provider` 枚举，添加 `'codex-cli'` 类型
- 添加索引：`idx_token_expires_at`, `idx_provider_status`

### ✅ 2. pkg/openai OAuth 服务（Task 2）
- 创建 `pkg/openai/oauth.go`
  - `GeneratePKCE()` - PKCE 参数生成（RFC 7636）
  - `GenerateAuthURL()` - 生成授权 URL
  - `ExchangeCode()` - 交换授权码获取 token
  - `RefreshToken()` - 刷新 access token（带3次重试）
  - `ValidateAccessToken()` - 使用 access token 验证账户
- 更新 `pkg/openai/client.go` 接口定义，添加 OAuth 方法
- 重构 `createHTTPClient()` 支持自定义超时

## 剩余实现步骤

### 📋 3. 扩展 Data 层（Task 3）

**文件**: `internal/data/account.go`

需要添加的字段到 `Account` 结构体：

```go
type Account struct {
	// ... 现有字段

	// OAuth 相关字段
	AccessTokenEncrypted  string     `gorm:"column:access_token_encrypted;type:varchar(1024)"`
	RefreshTokenEncrypted string     `gorm:"column:refresh_token_encrypted;type:varchar(1024)"`
	TokenExpiresAt        *time.Time `gorm:"column:token_expires_at"`
	IDTokenEncrypted      string     `gorm:"column:id_token_encrypted;type:varchar(2048)"`
	Organizations         string     `gorm:"column:organizations;type:text"` // JSON array
}
```

需要添加的方法：

```go
// ListCodexCLIAccountsNeedingRefresh 查询需要刷新 token 的 Codex CLI 账户
// 查询条件：provider='codex-cli' AND status='ACTIVE' AND token_expires_at < now() + 5分钟
func (r *accountRepo) ListCodexCLIAccountsNeedingRefresh(ctx context.Context) ([]*Account, error) {
	var accounts []*Account

	// Token 即将在 5 分钟内过期
	threshold := time.Now().Add(5 * time.Minute)

	err := r.data.db.WithContext(ctx).
		Where("provider = ? AND status = ? AND token_expires_at < ?",
			ProviderCodexCLI, StatusActive, threshold).
		Find(&accounts).Error

	if err != nil {
		return nil, err
	}

	return accounts, nil
}
```

需要添加的常量：

```go
const (
	// ... 现有常量

	// ProviderCodexCLI Codex CLI OAuth 账户
	ProviderCodexCLI AccountProvider = "codex-cli"
)
```

### 📋 4. 实现 Biz 层 OAuth 逻辑（Task 4）

**新建文件**: `internal/biz/account_openai_oauth.go`

```go
package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"QuotaLane/internal/data"
	"QuotaLane/pkg/openai"

	"github.com/google/uuid"
)

const (
	// OAuthSessionKeyPrefix Redis OAuth 会话前缀
	OAuthSessionKeyPrefix = "oauth_session:"

	// OAuthSessionTTL OAuth 会话过期时间（10分钟）
	OAuthSessionTTL = 10 * time.Minute
)

// OAuthSession OAuth 会话数据（存储在 Redis）
type OAuthSession struct {
	CodeVerifier  string    `json:"code_verifier"`
	CodeChallenge string    `json:"code_challenge"`
	State         string    `json:"state"`
	ProxyURL      string    `json:"proxy_url,omitempty"`
	Platform      string    `json:"platform"`
	CreatedAt     time.Time `json:"created_at"`
	ExpiresAt     time.Time `json:"expires_at"`
}

// GenerateOpenAIAuthURL 生成 OAuth 授权 URL
func (uc *AccountUsecase) GenerateOpenAIAuthURL(ctx context.Context, proxyURL string) (string, string, error) {
	// 1. 生成 PKCE 参数
	pkce, err := openai.GeneratePKCE()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate PKCE: %w", err)
	}

	// 2. 生成授权 URL
	authURL := uc.openaiService.GenerateAuthURL(pkce)

	// 3. 生成会话 ID
	sessionID := uuid.New().String()

	// 4. 保存会话数据到 Redis
	session := OAuthSession{
		CodeVerifier:  pkce.CodeVerifier,
		CodeChallenge: pkce.CodeChallenge,
		State:         pkce.State,
		ProxyURL:      proxyURL,
		Platform:      "openai",
		CreatedAt:     time.Now(),
		ExpiresAt:     time.Now().Add(OAuthSessionTTL),
	}

	sessionJSON, err := json.Marshal(session)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal session: %w", err)
	}

	key := fmt.Sprintf("%s%s", OAuthSessionKeyPrefix, sessionID)
	err = uc.rdb.Set(ctx, key, sessionJSON, OAuthSessionTTL).Err()
	if err != nil {
		uc.logger.Errorw("failed to save OAuth session to Redis",
			"session_id", sessionID,
			"error", err)
		return "", "", fmt.Errorf("failed to save session: %w", err)
	}

	uc.logger.Infow("generated OpenAI OAuth authorization URL",
		"session_id", sessionID,
		"auth_url", authURL)

	return authURL, sessionID, nil
}

// ExchangeOpenAICode 交换授权码创建账户
func (uc *AccountUsecase) ExchangeOpenAICode(ctx context.Context, sessionID, code, name, description string) (*data.Account, error) {
	// 1. 从 Redis 获取会话数据
	key := fmt.Sprintf("%s%s", OAuthSessionKeyPrefix, sessionID)
	sessionJSON, err := uc.rdb.Get(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("session not found or expired: %w", err)
	}

	var session OAuthSession
	if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
		return nil, fmt.Errorf("failed to parse session: %w", err)
	}

	// 2. 交换授权码获取 token
	tokens, err := uc.openaiService.ExchangeCode(ctx, code, session.CodeVerifier, session.ProxyURL)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	// 3. 加密存储 token
	accessTokenEncrypted, err := uc.crypto.Encrypt(tokens.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt access token: %w", err)
	}

	refreshTokenEncrypted, err := uc.crypto.Encrypt(tokens.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt refresh token: %w", err)
	}

	var idTokenEncrypted string
	if tokens.IDToken != "" {
		idTokenEncrypted, err = uc.crypto.Encrypt(tokens.IDToken)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt ID token: %w", err)
		}
	}

	// 4. 计算 token 过期时间
	expiresAt := time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)

	// 5. 序列化 organizations
	orgsJSON := ""
	if len(tokens.Organizations) > 0 {
		orgsBytes, _ := json.Marshal(tokens.Organizations)
		orgsJSON = string(orgsBytes)
	}

	// 6. 创建账户
	account := &data.Account{
		Name:                  name,
		Description:           description,
		Provider:              data.ProviderCodexCLI,
		Status:                data.StatusCreated, // 先设为 CREATED，验证通过后改为 ACTIVE
		HealthScore:           100,
		BaseAPI:               "https://api.openai.com", // Codex CLI 默认 endpoint
		AccessTokenEncrypted:  accessTokenEncrypted,
		RefreshTokenEncrypted: refreshTokenEncrypted,
		TokenExpiresAt:        &expiresAt,
		IDTokenEncrypted:      idTokenEncrypted,
		Organizations:         orgsJSON,
		Metadata:              fmt.Sprintf(`{"proxy_url":"%s"}`, session.ProxyURL),
	}

	// 7. 保存到数据库
	if err := uc.repo.CreateAccount(ctx, account); err != nil {
		return nil, fmt.Errorf("failed to create account: %w", err)
	}

	// 8. 验证 access token
	err = uc.ValidateCodexCLIAccount(ctx, account.ID)
	if err != nil {
		uc.logger.Warnw("Codex CLI account validation failed after creation",
			"account_id", account.ID,
			"error", err)
	}

	// 9. 删除 Redis 会话
	uc.rdb.Del(ctx, key)

	uc.logger.Infow("created Codex CLI account via OAuth",
		"account_id", account.ID,
		"account_name", name)

	return account, nil
}

// ValidateCodexCLIAccount 验证 Codex CLI 账户（使用 access token）
func (uc *AccountUsecase) ValidateCodexCLIAccount(ctx context.Context, accountID int64) error {
	// 1. 获取账户
	account, err := uc.repo.GetAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}

	// 验证 Provider 类型
	if account.Provider != data.ProviderCodexCLI {
		return fmt.Errorf("account is not Codex CLI type: provider=%s", account.Provider)
	}

	// 2. 解密 access token
	accessToken, err := uc.crypto.Decrypt(account.AccessTokenEncrypted)
	if err != nil {
		return fmt.Errorf("failed to decrypt access token: %w", err)
	}

	// 3. 提取代理配置
	var proxyURL string
	if account.Metadata != "" {
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(account.Metadata), &metadata); err == nil {
			if proxy, ok := metadata["proxy_url"].(string); ok {
				proxyURL = proxy
			}
		}
	}

	// 4. 调用 OpenAI 服务验证 access token
	err = uc.openaiService.ValidateAccessToken(ctx, account.BaseAPI, accessToken, proxyURL)

	if err != nil {
		// 验证失败：可能是 token 过期，尝试刷新
		if err := uc.RefreshCodexCLIToken(ctx, accountID); err != nil {
			// 刷新也失败，更新状态
			return uc.handleValidationFailure(ctx, account, err)
		}
		// 刷新成功，重新验证
		return nil
	}

	// 5. 验证成功
	return uc.handleValidationSuccess(ctx, account)
}

// RefreshCodexCLIToken 刷新 Codex CLI access token
func (uc *AccountUsecase) RefreshCodexCLIToken(ctx context.Context, accountID int64) error {
	// 1. 获取账户
	account, err := uc.repo.GetAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}

	// 2. 解密 refresh token
	refreshToken, err := uc.crypto.Decrypt(account.RefreshTokenEncrypted)
	if err != nil {
		return fmt.Errorf("failed to decrypt refresh token: %w", err)
	}

	// 3. 提取代理配置
	var proxyURL string
	if account.Metadata != "" {
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(account.Metadata), &metadata); err == nil {
			if proxy, ok := metadata["proxy_url"].(string); ok {
				proxyURL = proxy
			}
		}
	}

	// 4. 调用 OAuth 服务刷新 token
	tokens, err := uc.openaiService.RefreshToken(ctx, refreshToken, proxyURL)
	if err != nil {
		uc.logger.Errorw("failed to refresh Codex CLI token",
			"account_id", accountID,
			"error", err)
		return fmt.Errorf("failed to refresh token: %w", err)
	}

	// 5. 加密新 token
	accessTokenEncrypted, err := uc.crypto.Encrypt(tokens.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt access token: %w", err)
	}

	// 如果返回了新的 refresh token，也要加密
	var refreshTokenEncrypted string
	if tokens.RefreshToken != refreshToken {
		refreshTokenEncrypted, err = uc.crypto.Encrypt(tokens.RefreshToken)
		if err != nil {
			return fmt.Errorf("failed to encrypt refresh token: %w", err)
		}
	} else {
		refreshTokenEncrypted = account.RefreshTokenEncrypted
	}

	// 6. 更新数据库
	expiresAt := time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)
	account.AccessTokenEncrypted = accessTokenEncrypted
	account.RefreshTokenEncrypted = refreshTokenEncrypted
	account.TokenExpiresAt = &expiresAt

	if err := uc.repo.UpdateAccount(ctx, account); err != nil {
		return fmt.Errorf("failed to update account: %w", err)
	}

	uc.logger.Infow("refreshed Codex CLI access token",
		"account_id", accountID,
		"expires_at", expiresAt)

	return nil
}

// RefreshCodexCLITokens 批量刷新即将过期的 Codex CLI token（定时任务调用）
func (uc *AccountUsecase) RefreshCodexCLITokens(ctx context.Context) error {
	startTime := time.Now()

	// 查询需要刷新的账户
	accounts, err := uc.repo.ListCodexCLIAccountsNeedingRefresh(ctx)
	if err != nil {
		return fmt.Errorf("failed to list accounts: %w", err)
	}

	totalCount := len(accounts)
	if totalCount == 0 {
		uc.logger.Infow("no Codex CLI accounts need token refresh")
		return nil
	}

	uc.logger.Infow("starting Codex CLI token refresh",
		"total_accounts", totalCount)

	// 批量刷新
	successCount := 0
	failureCount := 0
	for _, account := range accounts {
		if err := uc.RefreshCodexCLIToken(ctx, account.ID); err != nil {
			uc.logger.Errorw("failed to refresh token for account",
				"account_id", account.ID,
				"account_name", account.Name,
				"error", err)
			failureCount++
		} else {
			successCount++
		}
	}

	duration := time.Since(startTime)
	uc.logger.Infow("Codex CLI token refresh completed",
		"total_accounts", totalCount,
		"success_count", successCount,
		"failure_count", failureCount,
		"duration_ms", duration.Milliseconds())

	return nil
}
```

### 📋 5. 扩展 Proto 定义（Task 5）

**文件**: `api/v1/account.proto`

```protobuf
// 新增消息定义
message GenerateOpenAIAuthURLRequest {
  // 代理配置（可选）
  optional string proxy_url = 1;
}

message GenerateOpenAIAuthURLResponse {
  string auth_url = 1;
  string session_id = 2;
}

message ExchangeOpenAICodeRequest {
  string session_id = 1;
  string code = 2;
  string name = 3;
  string description = 4;
}

message ExchangeOpenAICodeResponse {
  int64 account_id = 1;
  string account_name = 2;
  string status = 3;
}

// 扩展 AccountService
service AccountService {
  // ... 现有方法

  // OAuth 授权流程
  rpc GenerateOpenAIAuthURL(GenerateOpenAIAuthURLRequest) returns (GenerateOpenAIAuthURLResponse) {
    option (google.api.http) = {
      post: "/v1/accounts/openai/generate-auth-url"
      body: "*"
    };
  }

  rpc ExchangeOpenAICode(ExchangeOpenAICodeRequest) returns (ExchangeOpenAICodeResponse) {
    option (google.api.http) = {
      post: "/v1/accounts/openai/exchange-code"
      body: "*"
    };
  }
}
```

### 📋 6. 实现 Service 层（Task 6）

**文件**: `internal/service/account.go`

```go
// GenerateOpenAIAuthURL 生成 OAuth 授权 URL
func (s *AccountService) GenerateOpenAIAuthURL(ctx context.Context, req *v1.GenerateOpenAIAuthURLRequest) (*v1.GenerateOpenAIAuthURLResponse, error) {
	authURL, sessionID, err := s.uc.GenerateOpenAIAuthURL(ctx, req.ProxyUrl)
	if err != nil {
		return nil, err
	}

	return &v1.GenerateOpenAIAuthURLResponse{
		AuthUrl:   authURL,
		SessionId: sessionID,
	}, nil
}

// ExchangeOpenAICode 交换授权码创建账户
func (s *AccountService) ExchangeOpenAICode(ctx context.Context, req *v1.ExchangeOpenAICodeRequest) (*v1.ExchangeOpenAICodeResponse, error) {
	account, err := s.uc.ExchangeOpenAICode(ctx, req.SessionId, req.Code, req.Name, req.Description)
	if err != nil {
		return nil, err
	}

	return &v1.ExchangeOpenAICodeResponse{
		AccountId:   account.ID,
		AccountName: account.Name,
		Status:      string(account.Status),
	}, nil
}
```

### 📋 7. 配置定时任务（Task 7）

**文件**: `cmd/QuotaLane/main.go`

```go
// Codex CLI Token 刷新任务（每5分钟执行）
_, err = c.AddFunc("*/5 * * * *", func() {
	defer func() {
		if r := recover(); r != nil {
			helper.Errorf("panic in Codex CLI token refresh cron job: %v", r)
		}
	}()

	ctx := context.Background()
	helper.Info("Starting Codex CLI token refresh cron job")

	if err := accountUC.RefreshCodexCLITokens(ctx); err != nil {
		helper.Errorf("Codex CLI token refresh cron job failed: %v", err)
	} else {
		helper.Info("Codex CLI token refresh cron job completed successfully")
	}
})
if err != nil {
	helper.Fatalf("Failed to schedule Codex CLI token refresh cron job: %v", err)
}
```

## 下一步操作

1. **执行数据库迁移**：
   ```bash
   cd QuotaLane
   make migrate
   ```

2. **运行代码生成**：
   ```bash
   make proto  # 生成 Proto 代码
   make wire   # 生成 Wire 代码
   ```

3. **构建和测试**：
   ```bash
   make build
   make test
   ```

4. **测试 OAuth 流程**：
   - 调用 `GenerateOpenAIAuthURL` 生成授权链接
   - 在浏览器中授权
   - 调用 `ExchangeOpenAICode` 创建账户
   - 观察定时任务日志验证 token 刷新

## 参考 claude-relay-service 实现

- 前端 OAuth UI: `web/admin-spa/src/components/accounts/OAuthFlow.vue` (line 294-465)
- 后端授权 URL 生成: `src/routes/admin.js` (line 7103-7166)
- 后端代码交换: `src/routes/admin.js` (line 7169-7287)
- PKCE 生成逻辑: 参考 `generateOpenAIPKCE()` 函数

## 注意事项

1. **安全性**：
   - 所有 token 必须加密存储
   - OAuth 会话 TTL 10分钟
   - 使用 HTTPS（生产环境）

2. **错误处理**：
   - Token 刷新失败：更新账户状态为 ERROR
   - 授权码过期：提示用户重新授权
   - 代理连接失败：记录详细日志

3. **性能优化**：
   - Redis 缓存账户数据（TTL 5分钟）
   - 批量刷新时限制并发数

4. **兼容性**：
   - 保持与 openai-responses (API Key) 类型的兼容
   - 健康检查逻辑复用 `handleValidationSuccess/Failure`
