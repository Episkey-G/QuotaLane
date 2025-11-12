# QuotaLane 架构设计文档

## 1. 系统概述

QuotaLane 是一个基于 Kratos 微服务框架构建的高性能 AI API 中转服务,采用 Go 语言开发,旨在提供企业级的稳定性和性能。

### 1.1 架构目标

- **高性能**: 10,000 req/s 并发处理能力
- **低延迟**: P95 响应延迟 <50ms
- **高可用**: 99.9% 可用性保障
- **可扩展**: 支持水平扩展和微服务拆分
- **可维护**: 清晰的分层架构和依赖注入

### 1.2 核心原则

- **分层架构**: Server → Service → Biz → Data 四层分离
- **依赖注入**: 使用 Wire 编译期依赖注入
- **接口契约**: 所有 API 通过 Proto 定义
- **配置分离**: 支持多环境配置管理

---

## 2. 技术选型

### 2.1 框架与库

| 组件 | 技术选型 | 版本 | 用途 |
|------|---------|------|------|
| 微服务框架 | Kratos | v2.8.0 | 服务治理、中间件、生命周期管理 |
| 通信协议 | gRPC | v1.65.0 | 内部服务间高性能通信 |
| HTTP Gateway | grpc-gateway | - | 对外提供 REST API |
| 依赖注入 | Wire | v0.6.0 | 编译期依赖注入 |
| ORM | GORM | v1.31.1 | 数据库操作 |
| 缓存 | go-redis | v9.16.0 | Redis 客户端 |
| 日志 | Zap | v1.27.0 | 结构化日志 |
| 配置 | Viper | v1.21.0 | 配置管理 |
| 数据验证 | validator | v10.28.0 | 请求参数验证 |

### 2.2 数据存储

- **MySQL 8.0+**: 关系型数据存储 (用户、账户、订单、账单等)
- **Redis 6.0+**: 缓存、会话、速率限制、并发控制

---

## 3. 分层架构

### 3.1 四层架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                     Server 层                                │
│  - HTTP/gRPC 服务器启动和配置                                │
│  - 路由注册                                                  │
│  - 中间件 (认证、限流、日志、错误处理)                       │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│                    Service 层                                │
│  - 实现 Proto 定义的接口                                     │
│  - 参数验证和转换                                            │
│  - 调用 Biz 层业务逻辑                                       │
│  - 组装响应数据                                              │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│                      Biz 层                                  │
│  - 核心业务逻辑                                              │
│  - 领域模型定义                                              │
│  - 业务规则验证                                              │
│  - 事务管理                                                  │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│                     Data 层                                  │
│  - 数据访问实现                                              │
│  - MySQL 操作 (GORM)                                         │
│  - Redis 操作                                                │
│  - 缓存策略                                                  │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 层级职责

#### Server 层
- **职责**: 服务器启动、路由注册、中间件配置
- **文件位置**: `internal/server/`
- **关键组件**:
  - `http.go`: HTTP 服务器配置
  - `grpc.go`: gRPC 服务器配置
  - `middleware.go`: 中间件注册

#### Service 层
- **职责**: 实现 Proto 接口,参数验证,调用业务逻辑
- **文件位置**: `internal/service/`
- **关键组件**:
  - `account.go`: 账户服务实现
  - `gateway.go`: API Gateway 服务
  - `auth.go`: 认证服务
  - `user.go`: 用户服务

#### Biz 层
- **职责**: 核心业务逻辑,领域模型,业务规则
- **文件位置**: `internal/biz/`
- **关键组件**:
  - `account.go`: 账户业务逻辑
  - `scheduler.go`: 智能调度算法
  - `auth.go`: 认证业务逻辑
  - `billing.go`: 计费业务逻辑

#### Data 层
- **职责**: 数据持久化,缓存管理
- **文件位置**: `internal/data/`
- **关键组件**:
  - `mysql.go`: MySQL 连接池
  - `redis.go`: Redis 连接池
  - `account.go`: 账户数据操作
  - `cache.go`: 缓存策略

---

## 4. 核心模块设计

### 4.1 账号池管理

**功能**: 管理多平台 AI API 账户

**支持的账户类型**:
- Claude Official (OAuth)
- Claude Console
- AWS Bedrock
- Google Gemini
- OpenAI Responses (Codex)
- Azure OpenAI
- Droid (Factory.ai)
- CCR

**关键特性**:
- OAuth 2.0 PKCE 认证
- 自动 Token 刷新
- AES 加密敏感数据
- 独立代理配置
- 健康检查

### 4.2 智能调度引擎

**算法**: Headroom 算法

**调度策略**:
1. 检查粘性会话 (session hash)
2. 筛选可用账户 (健康检查、并发限制)
3. 计算 Headroom 分数
4. 选择最优账户
5. 更新并发计数

**并发控制**:
- Redis Sorted Set 实现
- 自动过期清理
- 原子操作保证

### 4.3 用户管理

**认证方式**:
- JWT Token
- API Key (cr_ 前缀)
- LDAP (企业集成)

**权限控制**:
- API Key 级别权限 (all/claude/gemini/openai)
- 客户端限制 (User-Agent)
- 模型黑名单

### 4.4 计费系统

**定价模型**:
- Token 级别计费
- 模型差异化定价
- 缓存 Token 优惠

**计费流程**:
1. 捕获 usage 数据
2. 计算 Token 成本
3. 更新使用统计
4. 生成账单

---

## 5. 数据模型

### 5.1 核心实体

#### User (用户)
```go
type User struct {
    ID        int64
    Username  string
    Email     string
    Password  string // bcrypt hash
    Status    string // active/disabled
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### APIKey (API 密钥)
```go
type APIKey struct {
    ID          int64
    UserID      int64
    KeyHash     string   // SHA-256 hash
    Permissions []string // all/claude/gemini/openai
    RateLimit   int
    QuotaLimit  int64
    UsedQuota   int64
    ExpiresAt   time.Time
}
```

#### Account (账户)
```go
type Account struct {
    ID            int64
    Name          string
    Type          string // claude-official/gemini/...
    Credentials   string // AES encrypted
    ProxyConfig   *ProxyConfig
    Status        string // active/disabled/overload
    HealthScore   int
    CreatedAt     time.Time
}
```

#### Subscription (订阅)
```go
type Subscription struct {
    ID        int64
    UserID    int64
    PlanID    int64
    Status    string // active/expired/cancelled
    StartAt   time.Time
    ExpireAt  time.Time
    AutoRenew bool
}
```

### 5.2 Redis 数据结构

| Key Pattern | 类型 | 用途 |
|------------|------|------|
| `api_key:{hash}` | Hash | API Key 详情 |
| `session:{hash}` | String | 粘性会话绑定 |
| `concurrency:{accountId}` | ZSet | 并发计数 |
| `rate_limit:{keyId}:{window}` | String | 速率限制 |
| `cache:{model}:{hash}` | String | 响应缓存 |

---

## 6. 通信协议

### 6.1 gRPC 接口定义

**Proto 文件位置**: `api/v1/`

**核心服务**:
```protobuf
service AccountService {
  rpc CreateAccount(CreateAccountRequest) returns (Account);
  rpc GetAccount(GetAccountRequest) returns (Account);
  rpc ListAccounts(ListAccountsRequest) returns (ListAccountsResponse);
  rpc UpdateAccount(UpdateAccountRequest) returns (Account);
  rpc DeleteAccount(DeleteAccountRequest) returns (Empty);
}

service GatewayService {
  rpc SendMessage(MessageRequest) returns (MessageResponse);
  rpc StreamMessage(MessageRequest) returns (stream MessageChunk);
}
```

### 6.2 HTTP Gateway

通过 grpc-gateway 自动生成 REST API:

```
POST   /api/v1/accounts           → CreateAccount
GET    /api/v1/accounts/{id}      → GetAccount
PUT    /api/v1/accounts/{id}      → UpdateAccount
DELETE /api/v1/accounts/{id}      → DeleteAccount
POST   /api/v1/messages           → SendMessage
```

---

## 7. 部署架构

### 7.1 单体部署 (MVP)

```
┌─────────────────────────┐
│   Nginx/Traefik         │
│   (负载均衡)            │
└────────┬────────────────┘
         │
    ┌────▼────┐  ┌─────────┐
    │QuotaLane│──│  Redis  │
    │ (HTTP+  │  └─────────┘
    │  gRPC)  │
    └────┬────┘
         │
    ┌────▼────┐
    │  MySQL  │
    └─────────┘
```

### 7.2 微服务部署 (未来)

```
┌────────────────────────────────────────┐
│        API Gateway (Envoy/Kong)        │
└────┬───────────┬───────────┬───────────┘
     │           │           │
┌────▼───┐  ┌───▼────┐  ┌───▼─────┐
│Account │  │Gateway │  │Billing  │
│Service │  │Service │  │Service  │
└────┬───┘  └───┬────┘  └───┬─────┘
     │          │           │
┌────▼──────────▼───────────▼─────┐
│         Shared Data Layer        │
│      (MySQL + Redis Cluster)     │
└──────────────────────────────────┘
```

---

## 8. 监控与可观测性

### 8.1 指标采集

- **Prometheus**: 采集 HTTP/gRPC 指标
- **Grafana**: 可视化仪表盘
- **Custom Metrics**: 业务指标 (账户健康、调度成功率)

### 8.2 日志

- **Zap**: 结构化日志
- **日志级别**: Debug/Info/Warn/Error
- **日志格式**: JSON
- **日志聚合**: ELK Stack / Loki

### 8.3 链路追踪

- **OpenTelemetry**: 分布式追踪
- **Jaeger**: 追踪可视化

---

## 9. 安全设计

### 9.1 认证与授权

- **JWT**: 用户身份认证
- **API Key**: 应用级认证
- **RBAC**: 基于角色的权限控制

### 9.2 数据安全

- **敏感数据加密**: AES-256-GCM
- **密码哈希**: bcrypt
- **API Key 哈希**: SHA-256
- **TLS**: 传输加密

### 9.3 速率限制

- **Token Bucket**: 令牌桶算法
- **滑动窗口**: 防止突发流量
- **并发控制**: Redis ZSet

---

## 10. 性能优化

### 10.1 缓存策略

- **L1 缓存**: LRU 内存缓存 (解密数据)
- **L2 缓存**: Redis 缓存 (响应数据)
- **缓存失效**: TTL + 主动失效

### 10.2 数据库优化

- **索引优化**: 覆盖索引、复合索引
- **查询优化**: GORM 预加载、批量查询
- **连接池**: 合理配置连接数

### 10.3 并发优化

- **Goroutine Pool**: 限制并发数
- **Context**: 请求超时控制
- **原子操作**: 减少锁竞争

---

## 11. 扩展性考虑

### 11.1 水平扩展

- **无状态设计**: 服务实例可随意扩缩容
- **会话存储**: Redis 集中式会话
- **负载均衡**: 轮询/最少连接

### 11.2 微服务拆分

**拆分原则**:
- 按业务领域拆分 (账户管理、网关、计费)
- 服务自治 (独立数据库)
- 通过 gRPC 通信

**未来架构**:
- Account Service (账户管理)
- Gateway Service (API 转发)
- Scheduler Service (智能调度)
- Billing Service (计费账单)
- Auth Service (认证授权)

---

## 12. 参考资料

- [Kratos 官方文档](https://go-kratos.dev)
- [gRPC Best Practices](https://grpc.io/docs/guides/performance/)
- [Google Go Style Guide](https://google.github.io/styleguide/go/)
- [Twelve-Factor App](https://12factor.net/)
