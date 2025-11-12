# QuotaLane 开发指南

本文档为 QuotaLane 项目开发者提供详细的开发指南和最佳实践。

---

## 📋 目录

- [开发环境设置](#开发环境设置)
- [项目结构说明](#项目结构说明)
- [开发工作流](#开发工作流)
- [代码规范](#代码规范)
- [测试指南](#测试指南)
- [调试技巧](#调试技巧)
- [常见问题](#常见问题)

---

## 🛠 开发环境设置

### 前置要求

```bash
# Go 版本
go version  # 应该 >= 1.22 (推荐 1.24+)

# 数据库
mysql --version  # >= 8.0
redis-server --version  # >= 6.0

# 开发工具
make --version
git --version
docker --version  # (可选)
```

### 安装开发工具

```bash
# 1. 安装 Kratos CLI
go install github.com/go-kratos/kratos/cmd/kratos/v2@latest

# 2. 安装 Wire (依赖注入)
go install github.com/google/wire/cmd/wire@latest

# 3. 安装 Protoc 工具
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest
go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest

# 4. 安装 golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# 5. 验证安装
kratos -v
wire --version
golangci-lint --version
```

### 配置 IDE

#### VS Code

推荐安装的扩展:
- **Go** (golang.go)
- **Protobuf** (zxh404.vscode-proto3)
- **YAML** (redhat.vscode-yaml)
- **Docker** (ms-azuretools.vscode-docker)

推荐设置 (`.vscode/settings.json`):
```json
{
  "go.useLanguageServer": true,
  "go.lintTool": "golangci-lint",
  "go.lintOnSave": "workspace",
  "go.formatTool": "goimports",
  "[go]": {
    "editor.formatOnSave": true,
    "editor.codeActionsOnSave": {
      "source.organizeImports": true
    }
  }
}
```

#### GoLand

1. 启用 Go Modules: `Settings → Go → Go Modules → Enable Go modules integration`
2. 配置 gofmt: `Settings → Tools → File Watchers → + → gofmt`
3. 配置 Wire: `Settings → Tools → File Watchers → + → Custom` (监听 wire.go 变化)

---

## 📁 项目结构说明

### 核心目录

```
QuotaLane/
├── api/                    # Proto 接口定义 (IDL)
│   └── v1/                 # API v1 版本
│       ├── *.proto         # Proto 文件
│       └── *.pb.go         # 生成的 Go 代码
├── cmd/                    # 应用程序入口
│   └── QuotaLane/
│       ├── main.go         # 主函数
│       ├── wire.go         # Wire 依赖注入配置
│       └── wire_gen.go     # Wire 生成的代码 (git ignore)
├── internal/               # 私有代码 (不对外暴露)
│   ├── biz/                # 业务逻辑层
│   │   ├── biz.go          # Provider 定义
│   │   └── *.go            # 业务逻辑实现
│   ├── data/               # 数据访问层
│   │   ├── data.go         # Provider 定义
│   │   ├── mysql.go        # MySQL 连接池
│   │   ├── redis.go        # Redis 连接池
│   │   └── *.go            # 数据操作实现
│   ├── service/            # 服务层 (实现 Proto 接口)
│   │   ├── service.go      # Provider 定义
│   │   └── *.go            # 服务实现
│   ├── server/             # 服务器层
│   │   ├── server.go       # Provider 定义
│   │   ├── http.go         # HTTP 服务器
│   │   ├── grpc.go         # gRPC 服务器
│   │   └── middleware.go   # 中间件
│   └── conf/               # 配置结构体定义
│       ├── conf.proto      # 配置 Proto 定义
│       └── conf.pb.go      # 生成的配置代码
├── pkg/                    # 公共库 (可复用)
│   ├── crypto/             # 加密工具
│   ├── oauth/              # OAuth 工具
│   ├── scheduler/          # 调度算法
│   └── limiter/            # 限流器
├── configs/                # 配置文件
│   ├── config.yaml         # 开发环境配置
│   └── config.prod.yaml    # 生产环境配置
├── migrations/             # 数据库迁移
│   ├── 001_init.up.sql
│   └── 001_init.down.sql
├── test/                   # 测试文件
│   ├── integration/        # 集成测试
│   └── e2e/                # 端到端测试
├── third_party/            # 第三方 Proto 依赖
├── docs/                   # 文档
├── scripts/                # 脚本工具
├── Makefile                # 构建脚本
├── go.mod                  # Go 依赖管理
└── README.md               # 项目说明
```

### 文件命名规范

- **Go 文件**: 小写蛇形命名 (`account_service.go`)
- **Proto 文件**: 小写蛇形命名 (`account.proto`)
- **测试文件**: `*_test.go`
- **Mock 文件**: `mock_*.go`
- **配置文件**: `config.yaml`, `config.{env}.yaml`

---

## 🔄 开发工作流

### 1. 新增功能开发流程

#### Step 1: 定义 Proto 接口

```bash
# 创建 Proto 文件
vim api/v1/account.proto
```

```protobuf
syntax = "proto3";

package api.v1;

option go_package = "QuotaLane/api/v1;v1";

service AccountService {
  rpc CreateAccount(CreateAccountRequest) returns (Account);
  rpc GetAccount(GetAccountRequest) returns (Account);
}

message CreateAccountRequest {
  string name = 1;
  string type = 2;
}

message Account {
  int64 id = 1;
  string name = 2;
  string type = 3;
}
```

#### Step 2: 生成代码

```bash
# 生成 Proto 代码
make proto

# 查看生成的文件
ls api/v1/
# account.pb.go         # Proto 消息定义
# account_grpc.pb.go    # gRPC 服务定义
# account_http.pb.go    # HTTP 路由定义
```

#### Step 3: 实现 Data 层

```bash
vim internal/data/account.go
```

```go
package data

import (
    "context"
    "QuotaLane/internal/biz"
    "gorm.io/gorm"
)

type accountRepo struct {
    data *Data
}

func NewAccountRepo(data *Data) biz.AccountRepo {
    return &accountRepo{data: data}
}

func (r *accountRepo) CreateAccount(ctx context.Context, account *biz.Account) error {
    // 实现数据库操作
    return r.data.db.Create(account).Error
}
```

#### Step 4: 实现 Biz 层

```bash
vim internal/biz/account.go
```

```go
package biz

import "context"

type Account struct {
    ID   int64
    Name string
    Type string
}

type AccountRepo interface {
    CreateAccount(ctx context.Context, account *Account) error
    GetAccount(ctx context.Context, id int64) (*Account, error)
}

type AccountUsecase struct {
    repo AccountRepo
}

func NewAccountUsecase(repo AccountRepo) *AccountUsecase {
    return &AccountUsecase{repo: repo}
}

func (uc *AccountUsecase) CreateAccount(ctx context.Context, name, accountType string) (*Account, error) {
    account := &Account{
        Name: name,
        Type: accountType,
    }
    if err := uc.repo.CreateAccount(ctx, account); err != nil {
        return nil, err
    }
    return account, nil
}
```

#### Step 5: 实现 Service 层

```bash
vim internal/service/account.go
```

```go
package service

import (
    "context"
    pb "QuotaLane/api/v1"
    "QuotaLane/internal/biz"
)

type AccountService struct {
    pb.UnimplementedAccountServiceServer
    uc *biz.AccountUsecase
}

func NewAccountService(uc *biz.AccountUsecase) *AccountService {
    return &AccountService{uc: uc}
}

func (s *AccountService) CreateAccount(ctx context.Context, req *pb.CreateAccountRequest) (*pb.Account, error) {
    account, err := s.uc.CreateAccount(ctx, req.Name, req.Type)
    if err != nil {
        return nil, err
    }
    return &pb.Account{
        Id:   account.ID,
        Name: account.Name,
        Type: account.Type,
    }, nil
}
```

#### Step 6: 配置 Wire 依赖注入

```bash
vim cmd/QuotaLane/wire.go
```

```go
//go:build wireinject
// +build wireinject

package main

import (
    "QuotaLane/internal/biz"
    "QuotaLane/internal/data"
    "QuotaLane/internal/service"
    "QuotaLane/internal/server"
    "github.com/google/wire"
)

func wireApp() (*kratos.App, func(), error) {
    panic(wire.Build(
        data.ProviderSet,
        biz.ProviderSet,
        service.ProviderSet,
        server.ProviderSet,
        newApp,
    ))
}
```

#### Step 7: 生成 Wire 代码并运行

```bash
# 生成 Wire 依赖注入代码
make wire

# 编译运行
make build
./bin/QuotaLane -conf ./configs

# 或直接运行
go run ./cmd/QuotaLane -conf ./configs
```

#### Step 8: 测试功能

```bash
# gRPC 测试 (使用 grpcurl)
grpcurl -plaintext -d '{"name":"test","type":"claude"}' \
    localhost:9000 api.v1.AccountService/CreateAccount

# HTTP 测试
curl -X POST http://localhost:8000/api/v1/accounts \
    -H "Content-Type: application/json" \
    -d '{"name":"test","type":"claude"}'
```

### 2. 修改现有功能

```bash
# 1. 修改 Proto 定义
vim api/v1/account.proto

# 2. 重新生成代码
make proto

# 3. 修改实现
vim internal/service/account.go

# 4. 重新编译
make build

# 5. 运行测试
make test
```

### 3. 添加依赖

```bash
# 添加依赖
go get github.com/some/package@latest

# 整理依赖
go mod tidy

# 验证依赖
go mod verify
```

---

## 📝 代码规范

### Go 代码规范

#### 1. 命名规范

```go
// ✅ 好的命名
type UserService struct {}
func NewUserService() *UserService {}
var ErrUserNotFound = errors.New("user not found")

// ❌ 不好的命名
type userservice struct {}
func new_user_service() *userservice {}
var err_user_not_found = errors.New("user not found")
```

#### 2. 错误处理

```go
// ✅ 使用 Kratos Errors
import "github.com/go-kratos/kratos/v2/errors"

func GetUser(id int64) (*User, error) {
    user, err := repo.FindByID(id)
    if err != nil {
        return nil, errors.NotFound("USER_NOT_FOUND", "user not found")
    }
    return user, nil
}

// ❌ 不要使用 panic
func GetUser(id int64) *User {
    user, err := repo.FindByID(id)
    if err != nil {
        panic(err) // 不要这样做
    }
    return user
}
```

#### 3. 日志记录

```go
import "github.com/go-kratos/kratos/v2/log"

// ✅ 结构化日志
log.Info("user created",
    log.Field("user_id", user.ID),
    log.Field("username", user.Username))

// ❌ 不要使用 fmt.Println
fmt.Println("user created:", user.ID)
```

#### 4. Context 使用

```go
// ✅ 始终传递 context
func GetUser(ctx context.Context, id int64) (*User, error) {
    user, err := repo.FindByID(ctx, id)
    return user, err
}

// ❌ 不要忽略 context
func GetUser(id int64) (*User, error) {
    user, err := repo.FindByID(id)
    return user, err
}
```

### Proto 代码规范

```protobuf
// ✅ 好的 Proto 定义
syntax = "proto3";

package api.v1;

option go_package = "QuotaLane/api/v1;v1";

import "google/api/annotations.proto";

service UserService {
  rpc GetUser(GetUserRequest) returns (User) {
    option (google.api.http) = {
      get: "/api/v1/users/{id}"
    };
  }
}

message GetUserRequest {
  int64 id = 1;
}

message User {
  int64 id = 1;
  string username = 2;
  string email = 3;
}
```

---

## 🧪 测试指南

### 单元测试

```go
// internal/biz/account_test.go
package biz

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestAccountUsecase_CreateAccount(t *testing.T) {
    repo := &mockAccountRepo{}
    uc := NewAccountUsecase(repo)

    account, err := uc.CreateAccount(context.Background(), "test", "claude")

    assert.NoError(t, err)
    assert.NotNil(t, account)
    assert.Equal(t, "test", account.Name)
}
```

### 集成测试

```go
// test/integration/account_test.go
package integration

import (
    "testing"
    "github.com/testcontainers/testcontainers-go"
)

func TestAccountIntegration(t *testing.T) {
    // 启动测试容器
    mysqlC, _ := testcontainers.GenericContainer(...)
    redisC, _ := testcontainers.GenericContainer(...)

    // 运行测试
    // ...

    // 清理
    defer mysqlC.Terminate(context.Background())
    defer redisC.Terminate(context.Background())
}
```

### 运行测试

```bash
# 运行所有测试
make test

# 运行特定包的测试
go test ./internal/biz/...

# 运行带覆盖率的测试
go test -cover ./...

# 生成覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

---

## 🐛 调试技巧

### 使用 Delve 调试

```bash
# 安装 Delve
go install github.com/go-delve/delve/cmd/dlv@latest

# 调试运行
dlv debug ./cmd/QuotaLane -- -conf ./configs

# 设置断点
(dlv) break internal/service/account.go:42
(dlv) continue
```

### 日志调试

```go
import "github.com/go-kratos/kratos/v2/log"

log.Debug("debugging info",
    log.Field("variable", value))
```

### 性能分析

```bash
# CPU profiling
go test -cpuprofile=cpu.prof ./...
go tool pprof cpu.prof

# 内存 profiling
go test -memprofile=mem.prof ./...
go tool pprof mem.prof
```

---

## ❓ 常见问题

### Q1: Wire 生成失败

```bash
# 确保 wire.go 有正确的 build tag
//go:build wireinject
// +build wireinject

# 重新生成
make wire
```

### Q2: Proto 生成失败

```bash
# 确保安装了所有工具
make init

# 检查 Proto 语法
protoc --lint api/v1/*.proto
```

### Q3: 依赖冲突

```bash
# 清理依赖
go clean -modcache
go mod tidy

# 重新下载
go mod download
```

### Q4: 测试失败

```bash
# 清理测试缓存
go clean -testcache

# 重新运行
go test -v ./...
```

---

## 📚 参考资料

- [Kratos 官方文档](https://go-kratos.dev)
- [Wire 用户指南](https://github.com/google/wire/blob/main/docs/guide.md)
- [gRPC-Go 教程](https://grpc.io/docs/languages/go/quickstart/)
- [GORM 指南](https://gorm.io/docs/)
- [Go Testing Best Practices](https://dave.cheney.net/2019/05/07/prefer-table-driven-tests)
