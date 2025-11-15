# QuotaLane API 设计规范

## 设计理念

QuotaLane 采用**扁平化路径**的 API 设计风格，每个操作对应一个独立的顶层路径。

### 核心原则

1. **统一 HTTP 方法**：所有 API 操作都使用 `POST` 方法
2. **扁平化路径**：每个操作有独立的顶层路径（如 `/CreateAccount`、`/ListAccounts`）
3. **路径即操作名**：路径直接对应 RPC 方法名，语义清晰
4. **类型安全**：每个操作有独立的 Request/Response 消息定义

### 设计优势

- ✅ **语义清晰**：路径名称直接表明操作意图，无需额外文档
- ✅ **gRPC-Gateway 自动路由**：利用 gRPC-Gateway 的自动转码功能
- ✅ **易于扩展**：添加新操作只需新增 RPC 方法和 HTTP 路由配置
- ✅ **类型安全**：每个操作有独立的 Protobuf 消息定义，编译时检查
- ✅ **统一调用方式**：所有操作统一使用 POST，简化客户端实现

---

## API 设计示例

### Account 服务

所有账户相关操作都有独立的顶层路径：

#### 1. 创建账户

**路径**：`POST /CreateAccount`

```bash
curl -X POST http://localhost:8000/CreateAccount \
  -H "Content-Type: application/json" \
  -d '{
    "Name": "My Claude Account",
    "Provider": "CLAUDE_CONSOLE",
    "OAuthData": "{...}",
    "RpmLimit": 10,
    "TpmLimit": 100000
  }'
```

**Response**:
```json
{
  "Account": {
    "Id": "1",
    "Name": "My Claude Account",
    "Provider": "CLAUDE_CONSOLE",
    "RpmLimit": 10,
    "TpmLimit": 100000,
    "HealthScore": 100,
    "Status": "ACCOUNT_ACTIVE",
    "CreatedAt": "2025-01-16T10:00:00Z",
    "UpdatedAt": "2025-01-16T10:00:00Z"
  }
}
```

#### 2. 查询账户列表

**路径**：`POST /ListAccounts`

```bash
curl -X POST http://localhost:8000/ListAccounts \
  -H "Content-Type: application/json" \
  -d '{
    "Page": 1,
    "PageSize": 10,
    "Provider": "CLAUDE_CONSOLE",
    "Status": "ACCOUNT_ACTIVE"
  }'
```

**Response**:
```json
{
  "Accounts": [...],
  "Total": 5,
  "Page": 1,
  "PageSize": 10
}
```

#### 3. 获取账户详情

**路径**：`POST /GetAccount`

```bash
curl -X POST http://localhost:8000/GetAccount \
  -H "Content-Type: application/json" \
  -d '{
    "Id": 1
  }'
```

#### 4. 更新账户

**路径**：`POST /UpdateAccount`

```bash
curl -X POST http://localhost:8000/UpdateAccount \
  -H "Content-Type: application/json" \
  -d '{
    "Id": 1,
    "Name": "Updated Account Name",
    "RpmLimit": 20,
    "Status": "ACCOUNT_ACTIVE"
  }'
```

#### 5. 删除账户

**路径**：`POST /DeleteAccount`

```bash
curl -X POST http://localhost:8000/DeleteAccount \
  -H "Content-Type: application/json" \
  -d '{
    "Id": 1
  }'
```

#### 6. 刷新 Token

**路径**：`POST /RefreshToken`

```bash
curl -X POST http://localhost:8000/RefreshToken \
  -H "Content-Type: application/json" \
  -d '{
    "Id": 1
  }'
```

#### 7. 测试账户连通性

**路径**：`POST /TestAccount`

```bash
curl -X POST http://localhost:8000/TestAccount \
  -H "Content-Type: application/json" \
  -d '{
    "Id": 1
  }'
```

#### 8. 生成 OAuth 授权 URL

**路径**：`POST /GenerateOAuthURL`

```bash
curl -X POST http://localhost:8000/GenerateOAuthURL \
  -H "Content-Type: application/json" \
  -d '{
    "Provider": "CLAUDE_OFFICIAL"
  }'
```

**Response**:
```json
{
  "AuthUrl": "https://claude.ai/oauth/authorize?...",
  "SessionId": "sess_abc123",
  "State": "random_state_string"
}
```

#### 9. 交换 OAuth 授权码

**路径**：`POST /ExchangeOAuthCode`

```bash
curl -X POST http://localhost:8000/ExchangeOAuthCode \
  -H "Content-Type: application/json" \
  -d '{
    "SessionId": "sess_abc123",
    "Code": "oauth_code_from_provider",
    "Name": "My OAuth Account",
    "RpmLimit": 10,
    "TpmLimit": 100000
  }'
```

**Response**:
```json
{
  "AccountId": "2",
  "AccountName": "My OAuth Account",
  "Status": "ACCOUNT_ACTIVE",
  "Message": "Account created successfully",
  "TokenExpiresAt": "2025-02-16T12:00:00Z"
}
```

#### 10. 轮询 OAuth 授权状态（Device Flow）

**路径**：`POST /PollOAuthStatus`

```bash
curl -X POST http://localhost:8000/PollOAuthStatus \
  -H "Content-Type: application/json" \
  -d '{
    "SessionId": "sess_abc123"
  }'
```

---

### Admin 服务

所有管理后台操作也使用扁平化路径：

#### 1. 获取仪表板数据

**路径**：`POST /GetDashboard`

```bash
curl -X POST http://localhost:8000/GetDashboard \
  -H "Content-Type: application/json" \
  -d '{}'
```

**Response**:
```json
{
  "Data": {
    "TotalUsers": 100,
    "ActiveUsers": 50,
    "TotalRequestsToday": 5000,
    "TotalRequestsMonth": 150000,
    "ErrorRate": 0.5,
    "ActiveAccounts": 10,
    "TotalAccounts": 15
  }
}
```

#### 2. 获取系统指标

**路径**：`POST /GetMetrics`

```bash
curl -X POST http://localhost:8000/GetMetrics \
  -H "Content-Type: application/json" \
  -d '{
    "MetricName": "request_count",
    "StartTime": "2025-01-01T00:00:00Z",
    "EndTime": "2025-01-16T23:59:59Z",
    "IntervalSeconds": 300
  }'
```

#### 3. 获取使用统计

**路径**：`POST /GetUsageStats`

```bash
curl -X POST http://localhost:8000/GetUsageStats \
  -H "Content-Type: application/json" \
  -d '{
    "UserId": 123,
    "StartTime": "2025-01-01T00:00:00Z",
    "EndTime": "2025-01-16T23:59:59Z",
    "GroupBy": "model"
  }'
```

#### 4. 获取健康状态

**路径**：`POST /GetHealth`

```bash
curl -X POST http://localhost:8000/GetHealth \
  -H "Content-Type: application/json" \
  -d '{}'
```

**Response**:
```json
{
  "Health": {
    "OverallStatus": "HEALTHY",
    "Components": [
      {
        "Name": "mysql",
        "Status": "HEALTHY",
        "Message": "Connection OK",
        "ResponseTimeMs": 5
      },
      {
        "Name": "redis",
        "Status": "HEALTHY",
        "Message": "Connection OK",
        "ResponseTimeMs": 2
      }
    ],
    "UptimeSeconds": 86400,
    "ErrorRate": 0.1,
    "CheckedAt": "2025-01-16T10:30:00Z"
  }
}
```

#### 5. 查询系统日志

**路径**：`POST /GetLogs`

```bash
curl -X POST http://localhost:8000/GetLogs \
  -H "Content-Type: application/json" \
  -d '{
    "Level": "ERROR",
    "Service": "account",
    "Keyword": "timeout",
    "StartTime": "2025-01-16T00:00:00Z",
    "EndTime": "2025-01-16T23:59:59Z",
    "Page": 1,
    "PageSize": 50
  }'
```

---

## Proto 定义规范

### 消息命名规范

每个操作都有独立的 Request/Response 消息定义：

- **Request 消息**：`{OperationName}Request`（如 `CreateAccountRequest`、`ListAccountsRequest`）
- **Response 消息**：`{OperationName}Response`（如 `CreateAccountResponse`、`ListAccountsResponse`）

### HTTP 路由配置

所有操作统一使用 `POST` 方法和扁平化路径：

```protobuf
service AccountService {
  rpc CreateAccount(CreateAccountRequest) returns (CreateAccountResponse) {
    option (google.api.http) = {
      post: "/CreateAccount"  // 扁平化路径，操作名即路径
      body: "*"                // 接收整个请求体
    };
  }

  rpc ListAccounts(ListAccountsRequest) returns (ListAccountsResponse) {
    option (google.api.http) = {
      post: "/ListAccounts"
      body: "*"
    };
  }
}
```

### Request 消息结构示例

```protobuf
message CreateAccountRequest {
  string Name = 1 [(validate.rules).string = {min_len: 1, max_len: 100}];
  AccountProvider Provider = 2 [(validate.rules).enum = {defined_only: true, not_in: [0]}];
  string ApiKey = 3;
  string OAuthData = 4;
  int32 RpmLimit = 5 [(validate.rules).int32 = {gte: 0}];
  int32 TpmLimit = 6 [(validate.rules).int32 = {gte: 0}];
  string Metadata = 7;
}
```

---

## API 路由映射

### Account 服务路由

| 操作 | HTTP 路径 | RPC 方法 |
|------|----------|---------|
| 创建账户 | `POST /CreateAccount` | `CreateAccount` |
| 查询列表 | `POST /ListAccounts` | `ListAccounts` |
| 获取详情 | `POST /GetAccount` | `GetAccount` |
| 更新账户 | `POST /UpdateAccount` | `UpdateAccount` |
| 删除账户 | `POST /DeleteAccount` | `DeleteAccount` |
| 刷新Token | `POST /RefreshToken` | `RefreshToken` |
| 测试连通性 | `POST /TestAccount` | `TestAccount` |
| 生成OAuth URL | `POST /GenerateOAuthURL` | `GenerateOAuthURL` |
| 交换授权码 | `POST /ExchangeOAuthCode` | `ExchangeOAuthCode` |
| 轮询授权状态 | `POST /PollOAuthStatus` | `PollOAuthStatus` |

### Admin 服务路由

| 操作 | HTTP 路径 | RPC 方法 |
|------|----------|---------|
| 获取仪表板 | `POST /GetDashboard` | `GetDashboard` |
| 获取系统指标 | `POST /GetMetrics` | `GetMetrics` |
| 获取使用统计 | `POST /GetUsageStats` | `GetUsageStats` |
| 获取健康状态 | `POST /GetHealth` | `GetHealth` |
| 查询系统日志 | `POST /GetLogs` | `GetLogs` |

---

## 客户端示例

### Go 客户端

```go
package main

import (
    "context"
    "google.golang.org/grpc"
    v1 "QuotaLane/api/v1"
)

func main() {
    conn, _ := grpc.Dial("localhost:9000", grpc.WithInsecure())
    defer conn.Close()

    client := v1.NewAccountServiceClient(conn)

    // 创建账户
    createResp, _ := client.CreateAccount(context.Background(), &v1.CreateAccountRequest{
        Name:     "My Account",
        Provider: v1.AccountProvider_CLAUDE_CONSOLE,
        RpmLimit: 10,
        TpmLimit: 100000,
    })

    // 查询列表
    listResp, _ := client.ListAccounts(context.Background(), &v1.ListAccountsRequest{
        Page:     1,
        PageSize: 10,
    })
}
```

### cURL 客户端

```bash
#!/bin/bash

BASE_URL="http://localhost:8000"

# 创建账户
curl -X POST $BASE_URL/CreateAccount \
  -H "Content-Type: application/json" \
  -d '{
    "Name": "My Account",
    "Provider": "CLAUDE_CONSOLE",
    "RpmLimit": 10,
    "TpmLimit": 100000
  }'

# 查询列表
curl -X POST $BASE_URL/ListAccounts \
  -H "Content-Type: application/json" \
  -d '{
    "Page": 1,
    "PageSize": 10
  }'

# 生成 OAuth URL
curl -X POST $BASE_URL/GenerateOAuthURL \
  -H "Content-Type: application/json" \
  -d '{
    "Provider": "CLAUDE_OFFICIAL"
  }'
```

---

## 错误处理

所有 API 遵循统一的错误响应格式：

```json
{
  "code": 400,
  "reason": "INVALID_ARGUMENT",
  "message": "Invalid provider: ACCOUNT_PROVIDER_UNSPECIFIED",
  "metadata": {
    "field": "Provider"
  }
}
```

**常见错误码**：
- `400` - 请求参数错误
- `401` - 未授权（API Key 无效）
- `403` - 禁止访问（权限不足）
- `404` - 资源不存在
- `500` - 服务器内部错误

---

## 最佳实践

### 1. 使用正确的字段名

字段名遵循 PascalCase（首字母大写）：

```bash
# ✅ 正确：使用 PascalCase
{
  "Name": "My Account",
  "Provider": "CLAUDE_CONSOLE"
}

# ❌ 错误：使用 camelCase
{
  "name": "My Account",
  "provider": "CLAUDE_CONSOLE"
}
```

### 2. 使用枚举值

Provider 和 Status 字段使用枚举常量：

```bash
# ✅ 正确：使用枚举常量
{
  "Provider": "CLAUDE_CONSOLE",
  "Status": "ACCOUNT_ACTIVE"
}

# ❌ 错误：使用数字
{
  "Provider": 2,
  "Status": 1
}
```

### 3. 处理可选字段

可选字段可以省略或设置为零值（0, "", false）：

```bash
# ✅ 正确：省略可选过滤条件
{
  "Page": 1,
  "PageSize": 10
}

# ✅ 也正确：提供可选过滤条件
{
  "Page": 1,
  "PageSize": 10,
  "Provider": "CLAUDE_CONSOLE",
  "Status": "ACCOUNT_ACTIVE"
}
```

### 4. 请求 ID 追踪

```bash
# ✅ 推荐：添加 X-Request-ID 用于日志追踪
curl -X POST http://localhost:8000/CreateAccount \
  -H "Content-Type: application/json" \
  -H "X-Request-ID: req-$(uuidgen)" \
  -d '{...}'
```

---

## 与其他设计风格的比较

### RESTful 风格（已废弃）

**旧设计**（不再使用）：
```
GET    /v1/accounts           # ListAccounts
GET    /v1/accounts/{Id}      # GetAccount
POST   /v1/accounts           # CreateAccount
PUT    /v1/accounts/{Id}      # UpdateAccount
DELETE /v1/accounts/{Id}      # DeleteAccount
```

**问题**：
- 不同 HTTP 方法混用，客户端需要记住哪个操作用哪个方法
- 路径参数和请求体参数混合，增加复杂度
- gRPC-Gateway 需要复杂的路由配置

### Action 字段风格（已废弃）

**旧设计**（不再使用）：
```
POST /v1/accounts  (Action: "CreateAccount")
POST /v1/accounts  (Action: "ListAccounts")
POST /v1/accounts  (Action: "UpdateAccount")
```

**问题**：
- gRPC-Gateway 无法基于请求体内容路由
- 需要自定义 HTTP 处理器
- 失去 gRPC-Gateway 的自动转码功能

### 扁平化路径风格（当前）

**新设计**：
```
POST /CreateAccount
POST /ListAccounts
POST /GetAccount
POST /UpdateAccount
POST /DeleteAccount
```

**优势**：
- ✅ 路径即操作名，语义最清晰
- ✅ 统一使用 POST 方法
- ✅ gRPC-Gateway 自动路由
- ✅ 无需自定义处理器
- ✅ OpenAPI/Swagger 自动生成

---

## 未来规划

### 计划中的服务

**Gateway 服务**：
- `POST /SendMessage` - 发送消息到 AI 模型
- `POST /StreamMessage` - 流式发送消息
- `POST /CancelRequest` - 取消正在进行的请求

**Billing 服务**：
- `POST /GetBill` - 获取账单
- `POST /GetInvoice` - 获取发票
- `POST /DownloadInvoice` - 下载发票

**User 服务**：
- `POST /CreateUser` - 创建用户
- `POST /UpdateUser` - 更新用户
- `POST /GetUserProfile` - 获取用户资料
- `POST /ListAPIKeys` - 查询用户的 API Keys

---

## 参考资料

- [gRPC-Gateway Documentation](https://grpc-ecosystem.github.io/grpc-gateway/)
- [Google Cloud API Design Guide](https://cloud.google.com/apis/design)
- [Protocol Buffers Style Guide](https://protobuf.dev/programming-guides/style/)
- [Kratos Framework](https://go-kratos.dev/)

---

**最后更新**: 2025-01-16
**版本**: v2.0.0 - 扁平化路径设计
