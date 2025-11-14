# QuotaLane

[![Go Version](https://img.shields.io/badge/Go-1.24-blue.svg)](https://golang.org)
[![Kratos](https://img.shields.io/badge/Kratos-v2.8.0-green.svg)](https://go-kratos.dev)
[![Build Status](https://github.com/Episkey-G/QuotaLane/workflows/CI/badge.svg)](https://github.com/Episkey-G/QuotaLane/actions)
[![codecov](https://codecov.io/gh/Episkey-G/QuotaLane/branch/main/graph/badge.svg)](https://codecov.io/gh/Episkey-G/QuotaLane)
[![Docker Image](https://ghcr-badge.egpl.dev/episkey-g/quotalane/latest_tag?trim=major&label=docker)](https://github.com/Episkey-G/QuotaLane/pkgs/container/quotalane)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

> **QuotaLane** - 高性能 AI API 中转服务的 Go 重构版本

基于 **Kratos 微服务框架** 构建的企业级 AI API 中转平台，提供账号池管理、智能调度、用户管理、套餐订阅等完整商业化功能。

---

## 📋 目录

- [项目愿景](#项目愿景)
- [核心特性](#核心特性)
- [技术栈](#技术栈)
- [快速开始](#快速开始)
- [项目架构](#项目架构)
- [开发指南](#开发指南)
- [API 文档](#api-文档)
- [部署指南](#部署指南)
- [贡献指南](#贡献指南)
- [许可证](#许可证)

---

## 🎯 项目愿景

QuotaLane 是 Claude Relay Service 的 Go 重构版本，旨在:

- **性能提升 10 倍**: 从 Node.js 1,000 req/s 提升到 Go 10,000 req/s
- **延迟降低 4 倍**: P95 响应延迟从 ~200ms 降至 <50ms
- **内存优化 2.5 倍**: 从 ~500MB 降至 <200MB
- **商业化运营**: 完整的套餐订阅、计费、账单系统
- **企业级稳定性**: 99.9% 可用性保障

### 重构目标

从 **Node.js 单体架构** 迁移到 **Go + Kratos 微服务框架**，支持:

- ✅ 多平台 AI API 支持 (Claude、Gemini、OpenAI、Bedrock、Azure、Droid)
- ✅ 智能账户调度和负载均衡
- ✅ 用户管理和 RBAC 权限控制
- ✅ 套餐订阅和自动计费
- ✅ 实时监控和告警系统
- ✅ 完整的 Web 管理后台

---

## ✨ 核心特性

### 1️⃣ 账号池管理系统
- 多账户类型支持 (Claude Official/Console, Bedrock, Gemini, OpenAI, Azure, Droid, CCR)
- OAuth 2.0 PKCE 认证流程
- 自动 Token 刷新和健康检查
- AES 加密敏感数据存储
- 独立代理配置 (SOCKS5/HTTP)
- 账户组和优先级管理

### 2️⃣ 智能调度引擎
- 统一调度器 (Headroom 算法)
- 粘性会话支持 (会话级账户绑定)
- 并发控制和速率限制
- 自动故障转移
- 负载均衡策略

### 3️⃣ 用户管理与认证
- 用户注册/登录系统
- JWT 认证中间件
- API Key 管理 (cr_ 前缀格式)
- 细粒度权限控制 (all/claude/gemini/openai)
- 客户端识别和限制
- 模型黑名单支持

### 4️⃣ 套餐订阅与计费
- 灵活的套餐管理
- 自动订阅续费
- 折扣码系统
- 订单管理和退款
- 套餐升级/降级
- 配额控制

### 5️⃣ 使用统计与成本计算
- 实时 Token 使用统计
- 模型定价服务
- 成本趋势分析
- 缓存优化统计
- 多维度报表

### 6️⃣ 监控告警系统
- Prometheus 指标采集
- Grafana 可视化仪表盘
- Webhook 告警通知
- 日志聚合 (Zap)
- 健康检查和就绪探针

---

## 🛠 技术栈

### 核心框架
- **Go**: 1.24.0+ (利用泛型和新标准库特性)
- **Kratos**: v2.8.0 (Go 微服务框架)
- **Wire**: v0.6.0 (编译期依赖注入)

### 通信协议
- **gRPC**: v1.65.0 (内部高性能通信)
- **HTTP Gateway**: grpc-gateway (外部 REST API)
- **Protocol Buffers**: v1.34.1 (接口定义语言)

### 数据存储
- **MySQL**: 主存储 (GORM v1.31.1)
- **Redis**: v9.16.0 (缓存 + 会话 + 并发控制)

### 配置与日志
- **Viper**: v1.21.0 (配置管理，支持多环境)
- **Zap**: v1.27.0 (结构化日志)

### 代码质量
- **golangci-lint**: 2.3.0 (14 个 linters)
- **validator**: v10.28.0 (数据验证)

### 部署
- **Docker**: 容器化部署
- **Docker Compose**: 开发环境编排

---

## 🚀 快速开始

### 🐳 方式一: Docker Compose 一键启动 (推荐)

**前置要求**:
- Docker 20.10+
- Docker Compose 2.0+

**启动步骤**:

#### 1. 克隆仓库
```bash
git clone https://github.com/Episkey-G/QuotaLane.git
cd QuotaLane
```

#### 2. 配置环境变量
```bash
# 复制环境变量模板
cp .env.example .env

# 编辑 .env 文件（⚠️ 生产环境必须修改 JWT_SECRET 和 ENCRYPTION_KEY）
vim .env
```

**重要配置说明**:
- `JWT_SECRET`: JWT 签名密钥（至少 32 字符，生成: `openssl rand -base64 32`）
- `ENCRYPTION_KEY`: 数据加密密钥（精确 32 字符，生成: `openssl rand -hex 16`）
- `MYSQL_ROOT_PASSWORD`: MySQL root 密码
- `QUOTALANE_ENV`: 运行环境（development/production）

#### 3. 一键启动所有服务
```bash
docker-compose up -d
```

这将启动以下 5 个容器:
- ✅ **app** - QuotaLane 应用 (端口: 8000, 9000, 9090)
- ✅ **mysql** - MySQL 8.0 数据库 (端口: 3306)
- ✅ **redis** - Redis 7 缓存 (端口: 6379)
- ✅ **prometheus** - Prometheus 监控 (端口: 9091)
- ✅ **grafana** - Grafana 可视化 (端口: 3000)

#### 4. 查看日志
```bash
# 查看应用启动日志
docker-compose logs -f app

# 等待日志显示 "Database connected" 和 "Redis connected"
```

#### 5. 访问服务

| 服务 | 地址 | 说明 |
|------|------|------|
| **应用 HTTP** | http://localhost:8000 | HTTP API 端口 |
| **应用 gRPC** | localhost:9000 | gRPC 服务端口 |
| **Prometheus Metrics** | http://localhost:9090 | 应用指标端口 (Story 7.1) |
| **Prometheus UI** | http://localhost:9091 | Prometheus Web 界面 |
| **Grafana** | http://localhost:3000 | 监控仪表盘 (默认: admin/admin) |

#### 6. 验证服务
```bash
# 检查所有容器状态（应全部显示 "Up" 或 "healthy"）
docker-compose ps

# 验证应用端口可访问（临时方案，Story 7.3 后使用 /health 端点）
nc -zv localhost 8000

# 验证数据库迁移成功
docker exec -it quotalane-mysql mysql -uroot -proot -D quotalane -e "SHOW TABLES;"
```

#### 7. 停止和清理
```bash
# 停止服务（保留数据）
docker-compose down

# 停止并删除所有数据卷（⚠️ 会删除数据库数据）
docker-compose down -v

# 重新构建应用镜像
docker-compose build --no-cache app
```

---

### 🛠 方式二: 本地开发环境

**前置要求**:
- Go 1.24+ (项目要求)
- MySQL 8.0+
- Redis 6.0+
- protoc 3.x+
- golang-migrate/migrate v4

**安装步骤**:

#### 1. 克隆仓库
```bash
git clone https://github.com/Episkey-G/QuotaLane.git
cd QuotaLane
```

#### 2. 安装依赖
```bash
# 安装 Go 依赖
go mod download

# 安装开发工具
make init
```

#### 3. 配置文件
```bash
# 复制配置模板
cp configs/config.yaml configs/config.local.yaml

# 编辑配置文件 (数据库、Redis 连接等)
vim configs/config.local.yaml
```

#### 4. 启动数据库 (Docker Compose)
```bash
# 仅启动 MySQL 和 Redis
docker-compose up -d mysql redis
```

#### 5. 数据库迁移
```bash
# 执行数据库迁移
bash scripts/migrate.sh up

# 插入种子数据
bash scripts/seed.sh
```

#### 6. 生成代码
```bash
# 生成 Proto 代码
make proto

# 生成 Wire 依赖注入代码
make wire
```

#### 7. 编译运行
```bash
# 编译项目
make build

# 运行服务
./bin/QuotaLane -conf ./configs
```

服务将在以下端口启动:
- **HTTP**: http://localhost:8000
- **gRPC**: localhost:9000

#### 8. 验证服务
```bash
# 测试 HTTP 端点
curl http://localhost:8000/helloworld/QuotaLane

# 预期响应
{"message":"Hello QuotaLane"}
```

---

### 🔧 常见问题排查

#### 端口占用
```bash
# 检查端口占用
lsof -i :8000
lsof -i :3306
lsof -i :6379

# 停止占用端口的进程
kill -9 <PID>
```

#### 数据库连接失败
```bash
# 查看 app 容器日志
docker-compose logs -f app

# 检查 MySQL 容器状态
docker-compose ps mysql

# 确认 MySQL 健康检查通过
docker inspect quotalane-mysql | grep Health
```

#### Proto/Wire 代码生成失败
```bash
# 重新构建应用镜像（清除缓存）
docker-compose build --no-cache app

# 查看构建日志
docker-compose build app
```

#### 配置文件找不到
```bash
# 确认配置文件存在
ls -la configs/config.yaml

# 确认 docker-compose.yml 正确挂载到 /data/conf
docker exec -it quotalane-app ls -la /data/conf
```

#### 数据持久化测试
```bash
# 插入测试数据
docker exec -it quotalane-mysql mysql -uroot -proot -D quotalane -e "SELECT * FROM plans;"

# 停止服务（不删除数据卷）
docker-compose down

# 重启服务
docker-compose up -d

# 验证数据仍然存在
docker exec -it quotalane-mysql mysql -uroot -proot -D quotalane -e "SELECT COUNT(*) FROM plans;"
```

---

## 🏗 项目架构

### 目录结构

```
QuotaLane/
├── api/                          # Proto 文件 (IDL 接口定义)
│   └── v1/
│       ├── account.proto         # 账号池接口
│       ├── gateway.proto         # API Gateway 接口
│       ├── auth.proto            # 认证授权接口
│       ├── plan.proto            # 套餐管理接口
│       ├── user.proto            # 用户管理接口
│       └── billing.proto         # 账单接口
├── cmd/                          # 主程序入口
│   └── QuotaLane/
│       ├── main.go               # 启动文件
│       ├── wire.go               # Wire 依赖注入配置
│       └── wire_gen.go           # Wire 自动生成代码
├── internal/                     # 内部实现 (不对外暴露)
│   ├── biz/                      # 业务逻辑层 (领域模型)
│   ├── data/                     # 数据访问层 (MySQL + Redis)
│   ├── service/                  # 服务层 (实现 Proto 接口)
│   └── server/                   # 服务器配置 (gRPC + HTTP)
├── pkg/                          # 公共库 (可跨项目复用)
│   ├── crypto/                   # AES 加密工具
│   ├── oauth/                    # OAuth 2.0 PKCE 工具
│   ├── scheduler/                # 调度算法
│   └── limiter/                  # 限流器
├── configs/                      # 配置文件
│   └── config.yaml               # 默认配置
├── migrations/                   # 数据库迁移脚本
├── third_party/                  # Proto 依赖 (Google API, Validate)
├── Makefile                      # 构建脚本
├── Dockerfile                    # Docker 镜像构建
├── docker-compose.yml            # Docker Compose 编排
└── README.md                     # 项目文档
```

### 分层架构

Kratos 标准四层架构 (自上而下):

```
┌─────────────────────────────────────────┐
│  Server 层 (HTTP/gRPC 服务器)           │
│  - 路由配置                              │
│  - 中间件 (认证、限流、日志)            │
└─────────────────────────────────────────┘
              ↓
┌─────────────────────────────────────────┐
│  Service 层 (实现 Proto 接口)           │
│  - 参数验证                              │
│  - 调用业务逻辑                          │
└─────────────────────────────────────────┘
              ↓
┌─────────────────────────────────────────┐
│  Biz 层 (业务逻辑/领域模型)             │
│  - 核心业务规则                          │
│  - 领域对象                              │
└─────────────────────────────────────────┘
              ↓
┌─────────────────────────────────────────┐
│  Data 层 (数据访问)                     │
│  - MySQL 操作 (GORM)                    │
│  - Redis 操作                            │
│  - 缓存策略                              │
└─────────────────────────────────────────┘
```

**架构原则**:
- 严格分层依赖，禁止跨层调用
- Wire 编译期依赖注入，避免运行时反射
- Proto 定义所有接口，确保类型安全
- Viper 多环境配置管理

---

## 💻 开发指南

### Makefile 命令

```bash
# 代码生成
make proto          # 生成 Proto 代码 (pb.go, gRPC, HTTP)
make wire           # 生成 Wire 依赖注入代码

# 构建
make build          # 编译二进制文件到 bin/
make docker         # 构建 Docker 镜像

# 测试
make test           # 运行单元测试 (带竞态检测)
make lint           # 运行代码质量检查 (golangci-lint)

# 全部生成
make all            # proto + config + wire
```

### 开发工作流

1. **修改 Proto 文件**
   ```bash
   vim api/v1/account.proto
   make proto
   ```

2. **实现业务逻辑**
   ```bash
   # Biz 层 (internal/biz/)
   # Service 层 (internal/service/)
   # Data 层 (internal/data/)
   ```

3. **生成 Wire 代码**
   ```bash
   make wire
   ```

4. **运行测试**
   ```bash
   make test
   make lint
   ```

5. **编译运行**
   ```bash
   make build
   ./bin/QuotaLane -conf ./configs
   ```

### 代码规范

- **遵循 Google Go Style Guide**
- **使用 gofmt/goimports 自动格式化**
- **golangci-lint 强制检查** (14 个 linters)
- **Kratos Errors 统一错误码**
- **Zap 结构化日志**

---

## 📚 API 文档

### gRPC API

Proto 文件位于 `api/v1/` 目录:

- `account.proto` - 账号池管理
- `gateway.proto` - API Gateway (转发、调度)
- `auth.proto` - 认证授权 (JWT、API Key)
- `user.proto` - 用户管理
- `plan.proto` - 套餐管理
- `billing.proto` - 计费账单

### HTTP Gateway

所有 gRPC 接口自动映射为 RESTful API:

```
POST   /api/v1/accounts           # 创建账户
GET    /api/v1/accounts/{id}      # 获取账户详情
PUT    /api/v1/accounts/{id}      # 更新账户
DELETE /api/v1/accounts/{id}      # 删除账户
POST   /api/v1/messages           # AI 消息转发
```

### OpenAPI 规范

自动生成的 OpenAPI 文档: `openapi.yaml`

---

## 🐳 部署指南

### Docker 部署

#### 构建镜像

```bash
make docker
# 或
docker build -t quotalane:latest .
```

#### 运行容器

```bash
docker run -d \
  -p 8000:8000 \
  -p 9000:9000 \
  -v $(pwd)/configs:/data/conf \
  --name quotalane \
  quotalane:latest
```

### Docker Compose 部署

```bash
# 启动所有服务 (MySQL, Redis, QuotaLane)
docker-compose up -d

# 查看日志
docker-compose logs -f quotalane

# 停止服务
docker-compose down
```

### 生产环境建议

- **使用环境变量**: 覆盖敏感配置 (数据库密码、密钥等)
- **启用健康检查**: `/health` 端点
- **配置监控**: Prometheus + Grafana
- **日志聚合**: 集中式日志收集
- **负载均衡**: Nginx/Traefik 前置
- **数据备份**: 定期备份 MySQL 数据

---

## 🤝 贡献指南

欢迎贡献代码! 请遵循以下步骤:

### 1. Fork 仓库

点击右上角 **Fork** 按钮

### 2. 克隆到本地

```bash
git clone https://github.com/YOUR_USERNAME/QuotaLane.git
cd QuotaLane
```

### 3. 创建功能分支

```bash
git checkout -b feature/your-feature-name
```

### 4. 提交代码

```bash
git add .
git commit -m "feat: add your feature description"
```

遵循 **Conventional Commits** 规范:
- `feat:` 新功能
- `fix:` Bug 修复
- `docs:` 文档更新
- `refactor:` 代码重构
- `test:` 测试相关
- `chore:` 构建/工具相关

### 5. 推送分支

```bash
git push origin feature/your-feature-name
```

### 6. 创建 Pull Request

在 GitHub 上创建 PR，等待 Code Review

### 代码审查标准

- ✅ 所有测试通过 (`make test`)
- ✅ 代码质量检查通过 (`make lint`)
- ✅ 遵循 Kratos 架构规范
- ✅ 添加必要的单元测试
- ✅ 更新相关文档

---

## 📄 许可证

本项目采用 **MIT License** 开源协议。详见 [LICENSE](LICENSE) 文件。

---

## 🙏 致谢

- [Kratos](https://go-kratos.dev) - 优秀的 Go 微服务框架
- [Wire](https://github.com/google/wire) - 编译期依赖注入工具
- [GORM](https://gorm.io) - Go ORM 框架
- [Zap](https://github.com/uber-go/zap) - 高性能日志库

---

## 📞 联系方式

- **GitHub Issues**: [提交 Issue](https://github.com/Episkey-G/QuotaLane/issues)
- **项目主页**: https://github.com/Episkey-G/QuotaLane

---

<p align="center">
  <b>🤖 Built with Kratos Framework | Powered by Go</b>
</p>
