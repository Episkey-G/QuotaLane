# Codecov 集成配置指南

本文档说明如何为 QuotaLane 项目配置 Codecov 测试覆盖率报告。

## 前置条件

- GitHub 仓库已启用 GitHub Actions
- 项目包含单元测试并生成覆盖率报告（`coverage.out`）

## 配置步骤

### 1. 创建 Codecov 账户

1. 访问 [Codecov 官网](https://about.codecov.io/)
2. 使用 GitHub 账户登录
3. 授权 Codecov 访问您的 GitHub 仓库

### 2. 添加仓库到 Codecov

1. 登录后，点击 "Add new repository"
2. 在仓库列表中找到 `QuotaLane` 并点击 "Setup repo"
3. Codecov 会自动检测仓库类型（Go）

### 3. 获取 Codecov Token（私有仓库必需）

**注意**: 公开仓库可以跳过此步骤，GitHub Actions 会自动上传覆盖率。

对于**私有仓库**：

1. 在 Codecov 仓库设置页面，找到 "Repository Upload Token"
2. 点击 "Copy" 复制 token（格式类似：`xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`）
3. **重要**: 此 token 是敏感信息，不要提交到代码库中

### 4. 配置 GitHub Secret

1. 打开 GitHub 仓库页面
2. 进入 **Settings** > **Secrets and variables** > **Actions**
3. 点击 **New repository secret**
4. 添加以下 Secret：
   - **Name**: `CODECOV_TOKEN`
   - **Value**: 粘贴在步骤 3 中复制的 token
5. 点击 **Add secret** 保存

### 5. 验证集成

1. 提交代码触发 GitHub Actions 工作流
2. 在 Actions 页面查看 `test` job 的 "Upload coverage to Codecov" 步骤
3. 如果配置正确，您会看到：
   ```
   Uploading coverage reports to Codecov
   [info] Codecov report uploaded successfully
   ```
4. 访问 Codecov 仓库页面查看覆盖率报告

## 覆盖率徽章（可选）

在 Codecov 仓库页面可以获取徽章代码：

1. 进入仓库 Settings > Badges & Graphs
2. 复制 Markdown 格式徽章代码
3. 添加到 `README.md`（项目已添加）

示例：
```markdown
[![codecov](https://codecov.io/gh/Episkey-G/QuotaLane/branch/main/graph/badge.svg)](https://codecov.io/gh/Episkey-G/QuotaLane)
```

## 工作流配置说明

项目已在 `.github/workflows/ci.yml` 中配置 Codecov 上传步骤：

```yaml
- name: Upload coverage to Codecov
  uses: codecov/codecov-action@v4
  with:
    token: ${{ secrets.CODECOV_TOKEN }}
    files: ./coverage.out
    flags: unittests
    name: codecov-umbrella
```

**配置说明**：
- `token`: 从 GitHub Secret 读取（私有仓库必需）
- `files`: 覆盖率文件路径（Go 测试生成）
- `flags`: 标记为单元测试覆盖率
- `name`: 上传标识符

## 常见问题

### Q: 公开仓库是否需要 CODECOV_TOKEN？

**A**: 不需要。公开仓库可以直接上传覆盖率，无需配置 token。但建议仍然配置以提高上传成功率。

### Q: 上传失败提示 "Missing repository token"

**A**: 这是私有仓库的常见问题。请确保：
1. 已在 Codecov 添加仓库
2. 已正确配置 `CODECOV_TOKEN` Secret
3. Secret 名称完全匹配（区分大小写）

### Q: 如何查看历史覆盖率趋势？

**A**: 在 Codecov 仓库页面的 "Graphs" 选项卡可以查看覆盖率趋势图和详细报告。

### Q: 可以设置覆盖率阈值吗？

**A**: 可以。在 Codecov 仓库设置中可以配置：
- 最低覆盖率要求
- PR 覆盖率下降阈值
- 自动评论 PR 的覆盖率报告

## 安全提醒

⚠️ **敏感信息保护**：
- ❌ 不要将 `CODECOV_TOKEN` 硬编码到代码中
- ❌ 不要将 token 提交到版本控制
- ✅ 始终使用 GitHub Secrets 存储敏感凭据
- ✅ 定期轮换 token（如有泄露风险）

## 参考链接

- [Codecov 官方文档](https://docs.codecov.com/)
- [GitHub Actions 集成指南](https://docs.codecov.com/docs/github-actions-integration)
- [codecov-action 仓库](https://github.com/codecov/codecov-action)
- [Go 语言覆盖率指南](https://go.dev/blog/cover)

## 故障排查

如果遇到问题，可以：
1. 查看 GitHub Actions 日志中的详细错误信息
2. 访问 [Codecov 状态页面](https://status.codecov.io/) 检查服务状态
3. 在 Codecov 仓库设置中查看上传历史和错误日志
4. 参考 [Codecov 故障排查文档](https://docs.codecov.com/docs/common-errors)
