# GitHub Actions CI/CD 工作流说明

本目录包含 QuotaLane 项目的自动化 CI/CD 工作流配置。

## 📋 工作流文件

### `ci.yml` - 主 CI/CD 流水线

完整的持续集成和持续部署流水线，包括以下 job：

#### 1. **Lint Job** - 代码质量检查
- **触发条件**: 所有 push 和 PR 事件
- **执行内容**:
  - 安装 Go 1.24 环境
  - 使用复合 action 安装 protoc 和插件（包含缓存优化）
  - 生成 Proto 和 Wire 代码
  - 运行 golangci-lint 检查（超时 5 分钟）
- **依赖**: 无
- **并行**: 与 test job 并行运行

#### 2. **Test Job** - 单元测试
- **触发条件**: 所有 push 和 PR 事件
- **执行内容**:
  - 安装 Go 1.24 环境和依赖
  - 生成 Proto 和 Wire 代码
  - 运行单元测试（`go test ./... -v -race -coverprofile=coverage.out`）
  - 上传覆盖率报告到 Codecov
- **依赖**: 无
- **并行**: 与 lint job 并行运行

#### 3. **Integration Test Job** - 集成测试（新增）
- **触发条件**: 所有 push 和 PR 事件
- **执行内容**:
  - 启动 MySQL 8.0 和 Redis 7 服务容器
  - 配置测试环境变量（.env 文件）
  - 等待服务就绪（健康检查）
  - 运行集成测试（`go test -tags=integration -v -race ./...`）
- **依赖**: lint 和 test job 必须成功
- **注意**: 当前项目尚无集成测试文件，job 会优雅处理此情况

#### 4. **Build Job** - 二进制构建
- **触发条件**: 所有 push 和 PR 事件
- **执行内容**:
  - 生成 Proto 和 Wire 代码
  - 编译 Go 二进制文件（`make build`）
  - 上传构建产物（保留 7 天）
- **依赖**: lint 和 test job 必须成功

#### 5. **Docker Job** - 镜像构建和推送
- **触发条件**: 仅在 `main` 分支 push 或 tag 创建时
- **执行内容**:
  - 设置 Docker Buildx（多平台构建）
  - 登录 GitHub Container Registry (ghcr.io)
  - 使用 metadata-action 生成镜像标签
  - 构建并推送 Docker 镜像（启用 GHA 缓存）
  - 测试拉取镜像并验证版本
- **依赖**: lint 和 test job 必须成功
- **权限**: 需要 `packages: write` 权限

## 🏷️ 镜像标签策略

使用 `docker/metadata-action@v5` 自动生成多个镜像标签：

| 触发事件 | 生成标签 | 示例 |
|---------|---------|------|
| 分支推送 | `type=ref,event=branch` | `ghcr.io/episkey-g/quotalane:main` |
| PR 推送 | `type=ref,event=pr` | `ghcr.io/episkey-g/quotalane:pr-123` |
| Tag 创建 (semver) | `type=semver,pattern={{version}}` | `ghcr.io/episkey-g/quotalane:1.2.3` |
| Tag 创建 (major.minor) | `type=semver,pattern={{major}}.{{minor}}` | `ghcr.io/episkey-g/quotalane:1.2` |
| 任意推送 | `type=sha,prefix={{branch}}-` | `ghcr.io/episkey-g/quotalane:main-abc1234` |
| main 分支推送 | `type=raw,value=latest` | `ghcr.io/episkey-g/quotalane:latest` |

## ⚡ 性能优化

### 1. Go 模块缓存
- 使用 `actions/setup-go@v5` 的内置缓存功能
- 缓存键：`${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}`
- 减少依赖下载时间：~2-3 分钟

### 2. Proto 插件缓存
- 复合 action `.github/actions/setup-proto` 缓存 `~/go/bin`
- 避免每次 job 重复安装 protoc-gen-* 插件
- 减少安装时间：~1-2 分钟

### 3. Docker 层缓存
- 使用 GitHub Actions Cache (`cache-from: type=gha`)
- 缓存 Docker 构建层，加速镜像构建
- 减少构建时间：~3-5 分钟

### 4. 并行执行
- lint、test job 无依赖关系，完全并行执行
- integration-test、build、docker job 依赖前置 job，顺序执行
- 总执行时间：< 10 分钟（符合 AC 5 要求）

## 🔧 复合 Action

### `.github/actions/setup-proto/action.yml`
封装 Protocol Buffers 工具安装逻辑，被 lint、test、build、integration-test job 复用。

**功能**:
- 安装系统 protoc 编译器
- 缓存 Go proto 插件（protoc-gen-go, protoc-gen-go-grpc 等）
- 安装 Wire 依赖注入工具

**优势**:
- 消除 54 行重复代码（3 job × 18 行）
- 统一管理 proto 工具版本
- 通过缓存减少 2-3 分钟安装时间

## 🔐 GitHub Secrets 配置

工作流需要以下 Secret（在仓库 Settings > Secrets and variables > Actions 配置）：

| Secret 名称 | 必需性 | 用途 | 获取方式 |
|------------|-------|------|---------|
| `CODECOV_TOKEN` | 可选（公开仓库） | 上传测试覆盖率到 Codecov | 参见 [docs/CODECOV_SETUP.md](../../docs/CODECOV_SETUP.md) 完整配置指南 |
| `GITHUB_TOKEN` | 自动提供 | 推送 Docker 镜像到 ghcr.io | GitHub 自动注入，无需配置 |

**注意**:
- `GITHUB_TOKEN` 需要在 Settings > Actions > General 中启用 "Read and write permissions"
- 私有仓库必须配置 `CODECOV_TOKEN`（详见 [Codecov 配置指南](../../docs/CODECOV_SETUP.md)）

## 🚀 触发场景

| 场景 | 触发的 Job | 镜像推送 | 部署 |
|-----|-----------|---------|------|
| 功能分支 Push | lint, test, integration-test, build | ❌ | ❌ |
| Pull Request | lint, test, integration-test, build | ❌ | ❌ |
| main 分支 Push | 全部 5 个 job | ✅ (latest + sha) | ❌ (可选) |
| Tag 创建 (v1.2.3) | 全部 5 个 job | ✅ (semver + latest) | ❌ (可选) |

## 📊 监控和调试

### 查看工作流执行
1. 访问仓库 Actions 页面: https://github.com/Episkey-G/QuotaLane/actions
2. 点击具体的工作流运行查看详细日志
3. 失败的 job 会在 GitHub 中显示红色 ❌，成功显示绿色 ✅

### 常见问题排查

#### 1. golangci-lint 超时
- **现象**: lint job 超时退出
- **原因**: 代码库过大或 linter 配置过严
- **解决**: 调整 `ci.yml` 中的 `--timeout=5m` 参数

#### 2. Docker 镜像推送失败
- **现象**: docker job 认证失败
- **原因**: GITHUB_TOKEN 权限不足
- **解决**: Settings > Actions > General > Workflow permissions 设置为 "Read and write permissions"

#### 3. 集成测试服务未就绪
- **现象**: 集成测试报错连接数据库失败
- **原因**: MySQL/Redis 服务容器健康检查未通过
- **解决**: 检查 `services` 配置的健康检查命令和超时时间

#### 4. Proto 代码生成失败
- **现象**: `make proto` 报错找不到 protoc-gen-* 插件
- **原因**: setup-proto action 缓存失效或安装失败
- **解决**: 清除缓存或检查 `.github/actions/setup-proto/action.yml` 配置

## 🔄 本地测试

### 运行 lint 检查
```bash
cd QuotaLane
make proto && make wire
golangci-lint run --timeout=5m
```

### 运行单元测试
```bash
make test
```

### 运行集成测试
```bash
# 启动测试环境
docker-compose up -d mysql redis

# 配置环境变量
cp .env.example .env

# 运行集成测试
go test -tags=integration -v -race ./...

# 清理环境
docker-compose down -v
```

### 构建 Docker 镜像
```bash
docker build -t quotalane:local .
docker run --rm quotalane:local --version
```

## 📚 参考文档

- [GitHub Actions 官方文档](https://docs.github.com/en/actions)
- [Docker Build Push Action](https://github.com/docker/build-push-action)
- [golangci-lint Action](https://github.com/golangci/golangci-lint-action)
- [Codecov Action](https://github.com/codecov/codecov-action)
- [QuotaLane 架构文档](../../docs/architecture-go.md)
- [Epic 1 Story 1.8 需求](../../docs/epics.md#Story-1.8)

## 📝 更新日志

### 2025-11-14 - Epic 1 Story 1.8
- ✅ 添加集成测试 job（使用 GitHub Actions services）
- ✅ 创建 setup-proto 复合 action（消除代码重复）
- ✅ 升级 Docker 镜像推送功能（metadata-action + build-push-action）
- ✅ 实现镜像标签策略（latest, semver, sha）
- ✅ 启用 Docker 层缓存（GHA cache）
- ✅ 性能优化：总执行时间 < 10 分钟
- ✅ 添加 CI/CD 徽章到 README.md

### 下一步计划（Epic 7）
- 添加测试环境部署步骤（SSH 或 Kubernetes）
- 添加生产环境部署步骤（Tag 创建时，手动批准）
- 集成 Slack/Email 通知
- 添加安全扫描 job（Trivy/Snyk）
