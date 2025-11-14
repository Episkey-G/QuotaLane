# Codecov 配置指南

## 配置步骤

### 1. 添加 Codecov Token 到 GitHub Secrets

**操作路径**：
```
GitHub 仓库 → Settings → Secrets and variables → Actions → New repository secret
```

**详细步骤**：

1. 打开仓库页面：https://github.com/Episkey-G/QuotaLane
2. 点击顶部的 **Settings** 标签
3. 在左侧菜单中找到 **Secrets and variables**
4. 点击 **Actions**
5. 点击右上角的 **New repository secret** 按钮
6. 填写以下信息：
   - **Name**: `CODECOV_TOKEN`
   - **Secret**: `481120af-e3ed-4f83-ac0b-98b294a8300c`
7. 点击 **Add secret** 保存

### 2. 验证配置

**CI 配置已更新**：
- `.github/workflows/ci.yml` 已添加 `token: ${{ secrets.CODECOV_TOKEN }}`
- 使用 `codecov/codecov-action@v4` 上传覆盖率报告

**触发测试**：

1. 提交并推送代码到任意分支
2. 等待 GitHub Actions 运行完成
3. 检查 Actions 日志中的 "Upload coverage to Codecov" 步骤
4. 如果成功，会显示：
   ```
   [info] Uploading reports
   [info] View report at: https://codecov.io/gh/Episkey-G/QuotaLane
   ```

### 3. 查看覆盖率报告

**仓库主页**：
- 访问：https://app.codecov.io/gh/Episkey-G/QuotaLane
- 查看整体覆盖率趋势、文件覆盖率、提交历史等

**Pull Request 检查**：
- 每个 PR 会自动显示 Codecov 检查结果
- 包含覆盖率变化、未覆盖的代码行等信息

### 4. 添加 Codecov 徽章到 README（可选）

在 `README.md` 中添加以下徽章：

```markdown
[![codecov](https://codecov.io/gh/Episkey-G/QuotaLane/branch/main/graph/badge.svg?token=YOUR_UPLOAD_TOKEN)](https://codecov.io/gh/Episkey-G/QuotaLane)
```

**获取徽章链接**：
1. 访问 https://app.codecov.io/gh/Episkey-G/QuotaLane
2. 点击 Settings → Badges & Graphs
3. 复制 Markdown 格式的徽章代码

## 故障排查

### 问题 1: "Missing repository token"

**原因**: GitHub Secret 未配置或名称错误

**解决方案**:
- 确认 Secret 名称为 `CODECOV_TOKEN`（大写）
- 确认 Token 值正确：`481120af-e3ed-4f83-ac0b-98b294a8300c`
- 检查 Secret 是否在正确的仓库中配置

### 问题 2: "No coverage reports found"

**原因**: 测试未生成覆盖率文件

**解决方案**:
- 检查 `make test` 是否生成 `coverage.out` 文件
- 确认 Makefile 中的测试命令包含 `-coverprofile=coverage.out`

### 问题 3: "Token is invalid"

**原因**: Token 过期或错误

**解决方案**:
- 登录 Codecov 重新生成 Token
- 更新 GitHub Secret 中的值

## 相关链接

- **Codecov Dashboard**: https://app.codecov.io/gh/Episkey-G/QuotaLane
- **GitHub Actions**: https://github.com/Episkey-G/QuotaLane/actions
- **Codecov 文档**: https://docs.codecov.com/docs

## 覆盖率目标

| 类型 | 当前 | 目标 |
|------|------|------|
| 整体覆盖率 | - | > 80% |
| 核心业务逻辑 | - | > 90% |
| 数据访问层 | - | > 85% |
| API 服务层 | - | > 80% |

**说明**: 当前值需要在首次运行 CI 后查看 Codecov 报告获取。
