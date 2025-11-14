# QuotaLane 测试指南

## 🧪 本地集成测试

### 前置条件

- ✅ Docker 或 OrbStack 已安装并运行
- ✅ Go 1.24+ 已安装
- ✅ 项目依赖已安装 (`go mod download`)

### 快速开始

#### 方式 1: 使用自动化脚本 (推荐)

```bash
cd QuotaLane

# 运行集成测试 (自动启动 MySQL + Redis 服务)
./scripts/run-integration-tests.sh
```

脚本会自动：
1. 检查 Docker 运行状态
2. 启动 MySQL 和 Redis 服务 (`docker-compose up -d mysql redis`)
3. 等待服务健康检查通过
4. 运行集成测试
5. 显示服务状态信息

#### 方式 2: 手动运行

```bash
cd QuotaLane

# 1. 启动 MySQL 和 Redis 服务
docker-compose up -d mysql redis

# 2. 等待服务就绪 (约 10-15 秒)
# 检查健康状态:
docker ps | grep quotalane

# 3. 运行集成测试
export TEST_MYSQL_DSN="root:root@tcp(127.0.0.1:3306)/quotalane?parseTime=true&loc=UTC"
export TEST_REDIS_ADDR="localhost:6379"
go test -tags=integration ./internal/biz -v

# 4. 停止服务 (可选)
docker-compose stop mysql redis
```

### 服务配置

测试使用 `docker-compose.yml` 中定义的标准服务：

| 服务 | 端口 | 容器名称 |
|------|------|----------|
| MySQL | 3306 | quotalane-mysql |
| Redis | 6379 | quotalane-redis |

**注意**：测试使用 Redis DB 1（而不是默认的 DB 0），避免与应用数据冲突。

### 集成测试覆盖

Story 2-2 的集成测试包含 6 个测试用例：

1. ✅ **成功刷新流程** - 完整的 decrypt → OAuth → encrypt → DB update
2. ✅ **失败处理** - 健康分数减 20 分，Redis 计数器
3. ✅ **连续失败 3 次** - 标记账户为 ERROR 状态
4. ✅ **批量刷新 10 个账户** - 验证并发执行（5 个并发）
5. ✅ **部分成功/部分失败** - 混合场景
6. ✅ **查询过滤逻辑** - ListExpiringAccounts 验证

### 故障排除

#### MySQL 连接失败

```bash
# 检查 MySQL 容器状态
docker ps | grep quotalane-mysql

# 查看 MySQL 日志
docker logs quotalane-mysql

# 测试连接
docker exec -it quotalane-mysql mysql -uroot -proot -e "SELECT 1"
```

#### Redis 连接失败

```bash
# 检查 Redis 容器状态
docker ps | grep quotalane-redis

# 测试连接
docker exec -it quotalane-redis redis-cli ping
```

#### 服务管理

```bash
# 启动所有服务
docker-compose up -d

# 仅启动 MySQL 和 Redis
docker-compose up -d mysql redis

# 停止服务
docker-compose stop mysql redis

# 完全清理（包括数据卷）
docker-compose down -v
```

### 清理测试数据

集成测试使用事务和自动清理机制，但如果需要手动清理：

```bash
# 清理 MySQL 测试数据
docker exec -it quotalane-mysql mysql -uroot -proot -e "DELETE FROM quotalane.api_accounts WHERE name LIKE 'Test_%'"

# 清理 Redis DB 1 (测试数据库)
docker exec -it quotalane-redis redis-cli -n 1 FLUSHDB
```

## 🔧 单元测试

```bash
# 运行所有单元测试
go test ./... -v

# 运行特定包的测试
go test ./pkg/oauth -v
go test ./internal/biz -v

# 查看测试覆盖率
go test ./pkg/oauth -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## 📊 测试覆盖率目标

- **pkg/oauth**: > 70% ✅ (当前 67.6%)
- **internal/biz**: > 80%
- **internal/data**: > 75%

## 🚀 CI/CD 集成

GitHub Actions 配置示例：

```yaml
name: Integration Tests

on: [push, pull_request]

jobs:
  integration-test:
    runs-on: ubuntu-latest

    services:
      mysql:
        image: mysql:8.0
        env:
          MYSQL_ROOT_PASSWORD: root
          MYSQL_DATABASE: quotalane
        ports:
          - 3306:3306
        options: >-
          --health-cmd="mysqladmin ping"
          --health-interval=10s
          --health-timeout=5s
          --health-retries=5

      redis:
        image: redis:7-alpine
        ports:
          - 6379:6379
        options: >-
          --health-cmd="redis-cli ping"
          --health-interval=10s
          --health-timeout=5s
          --health-retries=5

    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.24'

      - name: Run Integration Tests
        run: |
          export TEST_MYSQL_DSN="root:root@tcp(127.0.0.1:3306)/quotalane?parseTime=true&loc=UTC"
          export TEST_REDIS_ADDR="localhost:6379"
          go test -tags=integration ./internal/biz -v
```

## 📝 注意事项

1. **数据隔离**: 测试使用 Redis DB 1（生产使用 DB 0）
2. **并发安全**: 多个测试用例会并发执行，确保数据独立性
3. **自动清理**: 每个测试用例后自动清理 MySQL 和 Redis 数据
4. **环境变量**: 优先使用环境变量配置，方便 CI/CD 集成

## 🔗 相关文档

- [集成测试详细说明](internal/biz/INTEGRATION_TEST_README.md)
- [Story 2-2 实现文档](.bmad-ephemeral/stories/2-2-claude-oauth-refresh.md)
