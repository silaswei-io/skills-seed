# 更新日志

[简体中文](CHANGELOG.md) | [English](CHANGELOG.en.md)

## [v0.0.1]

Skills Seed 的首个公开版本

### 功能

- 支持从当前工作区或 Git 历史中学习项目专属编码模式
- 支持根据已学习的模式生成 Claude Code、Codex 和通用技能文档
- 支持检查暂存代码，并输出可执行的问题说明
- 支持交互式和自动化的 patch 修复流程
- 在 `.skills-seed` 下本地保存模式、项目画像、内存数据和日志，避免上传项目隐私数据
- 支持中文和英文 prompts、技能模板、配置模板和命令行文案
- 支持生成项目画像、模块参考、通用工具参考和业务方法参考
- 支持为 Claude 和 Codex 分别配置技能文档输出路径
- 支持统计 AI Agent 调用中的 token 用量
- 支持安装 Git pre-commit hook，在提交前自动检查代码

### CLI 命令

- `skills-seed init`
- `skills-seed learn current`
- `skills-seed learn history`
- `skills-seed check`
- `skills-seed generate-skills`
- `skills-seed patterns merge`
- `skills-seed profile refresh`
- `skills-seed hook install pre-commit`
- `skills-seed view`

### 发布

- 添加 GitHub Actions CI，自动执行格式检查、依赖一致性检查、`go vet` 和单元测试
- 添加基于 GitHub Actions 原生命令的 Release 打包流程
- 发布 Linux、macOS 和 Windows 的 x86_64 / arm64 包（Windows 当前发布 x86_64）
- 在 GitHub Releases 中附带校验和与版本说明
