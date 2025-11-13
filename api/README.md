# QuotaLane API Interfaces

本目录包含 QuotaLane 项目的所有 gRPC 服务接口定义（Proto3 格式）。

## 目录结构

```
api/
├── v1/                          # API版本1
│   ├── account.proto            # 账号池管理服务
│   ├── gateway.proto            # API网关服务
│   ├── auth.proto               # 认证服务
│   ├── user.proto               # 用户管理服务
│   ├── plan.proto               # 套餐管理服务
│   ├── billing.proto            # 账单管理服务
│   ├── admin.proto              # 管理后台服务
│   ├── *.pb.go                  # 生成的Go代码（protobuf消息）
│   ├── *_grpc.pb.go             # 生成的gRPC服务代码
│   └── *_http.pb.go             # 生成的HTTP Gateway代码
└── README.md                    # 本文档
```

## 7 个核心服务接口

### 1. AccountService (account.proto)

**功能**: 账号池管理服务
**说明**: 提供AI账号的CRUD操作、Token刷新和健康检测功能

**RPC方法**:
- `CreateAccount` - 创建新账号
- `ListAccounts` - 查询账号列表
- `GetAccount` - 获取账号详情
- `UpdateAccount` - 更新账号信息
- `DeleteAccount` - 删除账号
- `RefreshToken` - 刷新OAuth Token
- `TestAccount` - 测试账号连通性和健康度

**核心枚举**:
- `AccountProvider` - 支持的AI服务提供商（Claude, OpenAI, Gemini, Bedrock等）
- `AccountStatus` - 账户状态（Active, Inactive, Error）

**HTTP映射**:
- `POST /v1/accounts` - 创建账号
- `GET /v1/accounts` - 查询列表
- `GET /v1/accounts/{id}` - 获取详情
- `PUT /v1/accounts/{id}` - 更新账号
- `DELETE /v1/accounts/{id}` - 删除账号
- `POST /v1/accounts/{id}/refresh` - 刷新Token
- `POST /v1/accounts/{id}/test` - 测试账号

---

### 2. GatewayService (gateway.proto)

**功能**: API网关服务
**说明**: 提供统一的AI API调用接口，完全兼容Anthropic API格式

**RPC方法**:
- `CreateMessage` - 创建消息（**支持流式响应**）
- `CountTokens` - 统计消息的Token数量（Beta API）
- `ListModels` - 获取可用模型列表

**特性**:
- ✅ 流式响应支持（SSE格式）
- ✅ 完全兼容Anthropic Messages API
- ✅ Token使用统计（input/output/cache tokens）

**HTTP映射**:
- `POST /v1/messages` - 创建消息（流式）
- `POST /v1/messages/count_tokens` - Token计数
- `GET /v1/models` - 模型列表

---

### 3. AuthService (auth.proto)

**功能**: 认证服务
**说明**: 提供API Key管理和验证功能，实现虚拟Token机制

**RPC方法**:
- `ValidateAPIKey` - 验证API Key有效性
- `CreateAPIKey` - 创建API Key
- `ListAPIKeys` - 查询API Key列表
- `GetAPIKey` - 获取API Key详情
- `UpdateAPIKey` - 更新API Key信息
- `DeleteAPIKey` - 删除（吊销）API Key

**核心枚举**:
- `APIKeyStatus` - API Key状态（Active, Expired, Revoked）

**HTTP映射**:
- `POST /v1/auth/validate` - 验证API Key
- `POST /v1/keys` - 创建API Key
- `GET /v1/keys` - 查询列表
- `GET /v1/keys/{id}` - 获取详情
- `PUT /v1/keys/{id}` - 更新API Key
- `DELETE /v1/keys/{id}` - 删除API Key

---

### 4. UserService (user.proto)

**功能**: 用户管理服务
**说明**: 提供用户注册、登录、资料管理功能

**RPC方法**:
- `Register` - 用户注册
- `Login` - 用户登录
- `GetProfile` - 获取用户资料
- `UpdateProfile` - 更新用户资料
- `CreateAPIKey` - 用户创建自己的API Key

**核心枚举**:
- `UserRole` - 用户角色（User, Admin, SuperAdmin）
- `UserStatus` - 用户状态（Active, Inactive, Banned）

**HTTP映射**:
- `POST /v1/users/register` - 用户注册
- `POST /v1/users/login` - 用户登录
- `GET /v1/users/profile` - 获取资料
- `PUT /v1/users/profile` - 更新资料
- `POST /v1/users/api-keys` - 创建API Key

---

### 5. PlanService (plan.proto)

**功能**: 套餐管理服务
**说明**: 提供套餐查询、订阅、取消订阅功能

**RPC方法**:
- `ListPlans` - 查询套餐列表
- `GetPlan` - 获取套餐详情
- `Subscribe` - 订阅套餐
- `GetCurrentSubscription` - 获取当前订阅
- `CancelSubscription` - 取消订阅

**核心枚举**:
- `PlanStatus` - 套餐状态（Active, Inactive）
- `SubscriptionStatus` - 订阅状态（Active, Expired, Cancelled）

**HTTP映射**:
- `GET /v1/plans` - 套餐列表
- `GET /v1/plans/{id}` - 套餐详情
- `POST /v1/subscriptions` - 订阅套餐
- `GET /v1/subscriptions/current` - 当前订阅
- `DELETE /v1/subscriptions/current` - 取消订阅

---

### 6. BillingService (billing.proto)

**功能**: 账单管理服务
**说明**: 提供订单查询、取消、折扣码验证、邀请码生成功能

**RPC方法**:
- `ListOrders` - 查询订单列表
- `GetOrder` - 获取订单详情
- `CancelOrder` - 取消订单
- `ValidateDiscountCode` - 验证折扣码
- `GenerateInviteCode` - 生成邀请码

**核心枚举**:
- `OrderStatus` - 订单状态（Pending, Paid, Cancelled, Refunded）
- `DiscountType` - 折扣类型（Percentage, Fixed）
- `DiscountCodeStatus` - 折扣码状态
- `InviteCodeStatus` - 邀请码状态

**HTTP映射**:
- `GET /v1/orders` - 订单列表
- `GET /v1/orders/{id}` - 订单详情
- `POST /v1/orders/{id}/cancel` - 取消订单
- `POST /v1/discount-codes/validate` - 验证折扣码
- `POST /v1/invite-codes` - 生成邀请码

---

### 7. AdminService (admin.proto)

**功能**: 管理后台服务
**说明**: 提供系统监控、统计、健康检查、日志查询功能

**RPC方法**:
- `GetDashboard` - 获取仪表板数据
- `GetMetrics` - 获取系统指标
- `GetUsageStats` - 获取使用统计
- `GetHealth` - 获取健康状态
- `GetLogs` - 查询系统日志

**核心枚举**:
- `LogLevel` - 日志级别（Debug, Info, Warn, Error, Fatal）
- `HealthStatus` - 健康状态（Healthy, Degraded, Unhealthy）

**HTTP映射**:
- `GET /v1/admin/dashboard` - 仪表板
- `GET /v1/admin/metrics` - 系统指标
- `GET /v1/admin/usage-stats` - 使用统计
- `GET /v1/admin/health` - 健康状态
- `GET /v1/admin/logs` - 系统日志

---

## Proto 编译命令

### 编译所有 Proto 文件

```bash
make proto
```

这个命令会：
1. 编译 `api/` 目录下的所有 Proto 文件
2. 生成 Go 代码（.pb.go）
3. 生成 gRPC 服务代码（_grpc.pb.go）
4. 生成 HTTP Gateway 代码（_http.pb.go）
5. 生成 OpenAPI 文档（openapi.yaml）
6. 编译 `internal/` 目录下的配置 Proto 文件

### 清理生成的文件

```bash
make proto-clean
```

### 重新编译

```bash
make proto-clean && make proto
```

---

## 使用示例

### gRPC 客户端示例

```go
package main

import (
    "context"
    "log"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"

    v1 "QuotaLane/api/v1"
)

func main() {
    // 连接 gRPC 服务器
    conn, err := grpc.Dial("localhost:9000", grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    defer conn.Close()

    // 创建 AccountService 客户端
    client := v1.NewAccountServiceClient(conn)

    // 创建账号
    resp, err := client.CreateAccount(context.Background(), &v1.CreateAccountRequest{
        Name:     "Claude Official Account",
        Provider: v1.AccountProvider_CLAUDE_OFFICIAL,
        ApiKey:   "sk-xxx",
        RpmLimit: 100,
        TpmLimit: 100000,
    })
    if err != nil {
        log.Fatalf("CreateAccount failed: %v", err)
    }

    log.Printf("Account created: %+v", resp.Account)
}
```

### HTTP Gateway 示例

```bash
# 创建账号（HTTP POST）
curl -X POST http://localhost:8000/v1/accounts \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Claude Official Account",
    "provider": "CLAUDE_OFFICIAL",
    "api_key": "sk-xxx",
    "rpm_limit": 100,
    "tpm_limit": 100000
  }'

# 查询账号列表（HTTP GET）
curl http://localhost:8000/v1/accounts?page=1&page_size=10

# 创建消息（流式响应）
curl -X POST http://localhost:8000/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "max_tokens": 1024,
    "messages": [
      {"role": "user", "content": "你好"}
    ],
    "stream": true
  }'
```

---

## 字段验证规则

所有 Proto 消息都包含字段验证注解（使用 `validate.proto`），确保数据有效性：

```protobuf
// 邮箱格式验证
string email = 1 [(validate.rules).string = {email: true}];

// 密码长度验证
string password = 2 [(validate.rules).string = {min_len: 8, max_len: 128}];

// 数字范围验证
int32 rpm_limit = 3 [(validate.rules).int32 = {gte: 0}];

// 枚举值验证
AccountProvider provider = 2 [(validate.rules).enum = {defined_only: true, not_in: [0]}];
```

---

## 数据库表对应关系

Proto 消息与 MySQL 表结构保持一致：

| Proto Message | MySQL Table | 说明 |
|--------------|-------------|------|
| Account | api_accounts | 账号池表 |
| APIKey | api_keys | 虚拟Token表 |
| User | users | 用户基本信息表 |
| Plan | plans | 套餐定义表 |
| Order | orders | 订单记录表 |
| DiscountCode | discount_codes | 折扣码表 |
| InviteCode | invite_codes | 邀请码表 |

---

## 扩展性设计

### 预留扩展空间

所有枚举都预留了扩展空间：

```protobuf
enum AccountProvider {
  ACCOUNT_PROVIDER_UNSPECIFIED = 0;
  CLAUDE_OFFICIAL = 1;       // MVP已支持
  CLAUDE_CONSOLE = 2;        // MVP已支持
  BEDROCK = 3;               // 预留，后续添加
  GEMINI = 6;                // 预留，后续添加
  // ... 更多提供商可以后续添加
}
```

### 元数据扩展

使用 JSON 格式的 `metadata` 字段支持动态扩展：

```protobuf
message Account {
  // ...
  string metadata = 11;  // 扩展元数据（JSON格式：代理配置、区域等）
}
```

---

## 参考文档

- [Protocol Buffers Language Guide (proto3)](https://protobuf.dev/programming-guides/proto3/)
- [gRPC Go Quick Start](https://grpc.io/docs/languages/go/quickstart/)
- [gRPC-Gateway](https://grpc-ecosystem.github.io/grpc-gateway/)
- [protoc-gen-validate](https://github.com/bufbuild/protoc-gen-validate)
- [Kratos Framework](https://go-kratos.dev/)

---

## 注意事项

1. **枚举值命名**: 所有枚举值使用带前缀的 UPPER_SNAKE_CASE 格式，避免包级别的命名冲突
2. **消息命名**: 使用 CamelCase（如 CreateAccountRequest）
3. **字段命名**: 使用 snake_case（如 created_at）
4. **RPC 方法命名**: 使用动词+名词格式（如 CreateAccount）
5. **时间字段**: 统一使用 `google.protobuf.Timestamp` 类型
6. **可选字段**: 使用 `optional` 关键字标记可选字段（Proto3语法）

---

## 生成的文件说明

每个 `.proto` 文件会生成 3 个 Go 文件：

1. **`*.pb.go`** - Protocol Buffers 消息定义
   - 包含所有消息类型的 Go 结构体
   - 实现 `proto.Message` 接口

2. **`*_grpc.pb.go`** - gRPC 服务定义
   - 包含服务接口定义
   - 包含客户端和服务端实现

3. **`*_http.pb.go`** - HTTP Gateway 定义
   - 包含 HTTP 路由映射
   - 支持 RESTful API 访问

---

**最后更新**: 2025-11-13
**Proto 版本**: v1
**编译器版本**: protoc 31.1
