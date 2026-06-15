# 贡献指南与 Release 流程

## 📖 目录

- [代码提交规范](#代码提交规范)
- [Release 流程](#release-流程)
- [版本号规则](#版本号规则)
- [标签创建步骤](#标签创建步骤)

---

## 📝 代码提交规范

### 提交格式

```
<类型>(<范围>): <描述>

[可选的详细描述]
```

### 类型说明

| 类型 | 说明 | 示例 |
|------|------|------|
| `feat` | 新功能 | `feat(api): 添加百度翻译 API 支持` |
| `fix` | Bug 修复 | `fix(csv): 修复中文 slug 解码问题` |
| `docs` | 文档更新 | `docs(readme): 更新目录结构说明` |
| `refactor` | 代码重构 | `refactor(utils): 优化翻译缓存逻辑` |
| `perf` | 性能优化 | `perf(cache): 减少 API 调用次数` |
| `test` | 测试相关 | `test(merge): 添加合并功能测试` |
| `chore` | 构建/工具 | `chore(docker): 更新 Dockerfile` |
| `style` | 代码风格 | `style(go): 格式化 Go 代码` |

### 提交示例

```bash
# 正确示例
git commit -m "feat(nginx): 支持自动生成 301 跳转规则"
git commit -m "fix(csv): 处理超长 slug 截断问题"
git commit -m "docs(readme): 添加 Step 4 英文标签修复说明"
```

---

## 🚀 Release 流程

### 标准流程

```
1. 完成功能开发 → 2. 编写测试 → 3. 更新文档 → 4. 创建标签 → 5. 推送标签 → 6. 自动生成 Release
```

### 详细步骤

1. **代码审查**
   - 确保所有代码已通过代码审查
   - 确保所有测试用例通过

2. **更新版本号**
   - 根据变更内容确定版本号（见下文版本号规则）

3. **更新 CHANGELOG（可选）**
   - 如果项目有 CHANGELOG.md，更新版本变更记录

4. **创建 Git 标签**
   ```bash
   # 创建带注释的标签
   git tag -a v1.0.0 -m "Release v1.0.0"
   
   # 查看标签
   git tag
   ```

5. **推送标签**
   ```bash
   # 推送单个标签
   git push origin v1.0.0
   
   # 或推送所有标签
   git push origin --tags
   ```

6. **自动生成 Release**
   - 推送标签后，GitHub Actions 会自动执行 `.github/workflows/release.yml`
   - 自动生成 Release 描述，包含按类型分类的变更日志

---

## 🔢 版本号规则

采用语义化版本控制（Semantic Versioning）：

```
v<主版本号>.<次版本号>.<修订号>
```

| 版本号 | 变更场景 | 示例 |
|--------|----------|------|
| **主版本号** | 不兼容的 API 变更 | `v2.0.0` |
| **次版本号** | 向后兼容的新功能 | `v1.1.0` |
| **修订号** | 向后兼容的 Bug 修复 | `v1.0.1` |

### 预发布版本

- **Beta 版本**：`v1.0.0-beta.1`
- **RC 版本**：`v1.0.0-rc.1`

### 版本升级判断

| 变更类型 | 版本升级 |
|----------|----------|
| 新增功能 | 次版本号 +1 |
| Bug 修复 | 修订号 +1 |
| 破坏性变更 | 主版本号 +1 |

---

## 📌 标签创建步骤

### 方法一：使用命令行

```bash
# 查看当前分支
git branch

# 确保在 main 分支
git checkout main

# 拉取最新代码
git pull origin main

# 创建标签
git tag -a v1.0.0 -m "Release v1.0.0"

# 推送标签
git push origin v1.0.0
```

### 方法二：使用 GitHub 网页

1. 访问仓库的 **Releases** 页面
2. 点击 **Draft a new release**
3. 在 **Choose a tag** 输入框中输入新标签名（如 `v1.0.0`）
4. 点击 **Create new tag**
5. 填写版本标题和描述（或等待自动生成）
6. 点击 **Publish release**

---

## 📊 自动生成的 Release 描述示例

推送标签后，GitHub Actions 会自动生成如下格式的 Release 描述：

```markdown
## 📋 版本说明

📈 [查看变更对比](https://github.com/xxx/xxx/compare/v0.1.0...v1.0.0)

✨ 新功能

- 添加百度翻译 API 支持
- 支持自动生成 Nginx 301 跳转规则

🐛 Bug 修复

- 修复中文 slug 解码问题
- 处理超长 slug 截断问题

📝 文档更新

- 更新目录结构说明
- 添加 Step 4 英文标签修复说明

---

🚀 发布于 2026/06/15
```

---

## 🛠️ 常见问题

### Q1: Release 描述没有自动生成？

**检查事项：**
- 标签名称是否以 `v` 开头（如 `v1.0.0`）
- `.github/workflows/release.yml` 文件是否存在
- GitHub Actions 运行状态是否正常

### Q2: 如何取消已发布的 Release？

```bash
# 删除本地标签
git tag -d v1.0.0

# 删除远程标签
git push origin :v1.0.0
```

然后在 GitHub 网页上删除对应的 Release。

---

## 📜 参考链接

- [语义化版本控制](https://semver.org/lang/zh-CN/)
- [Conventional Commits](https://www.conventionalcommits.org/zh-hans/)
- [GitHub Actions 文档](https://docs.github.com/zh/actions)