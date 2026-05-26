# 更新日志

[简体中文](CHANGELOG.md) | [English](CHANGELOG.en.md)

## [v0.0.4]

### 功能

- workspace 初始化只扫描第一层目录，并扩展常见项目标记识别范围
- workspace 模式下按当前 `agent.provider` 生成根入口 skill，并为子项目生成各自 skill
- workspace 根 skill 路由只引用当前 provider 的子项目 skill，避免 Claude 生成时同时落出 Codex 目录
- workspace 根 skill 也生成 provider 元数据，Codex 输出时包含标准 `agents/openai.yaml`
- 新增 `workspace.child_skill_policy`，支持 `skip_existing`、`overwrite`、`root_only` 控制子项目 skill 生成
- `generate-skills` 新增 `--overwrite` 和 `--root-only`，可在本次生成中覆盖 workspace 子项目 skill 策略
- workspace 子项目存在 `.skills-seed/config.yaml` 时视为独立初始化，外层 workspace 不生成或覆盖该子项目 skill

### 模板

- 强化 workspace 根 skill 内容，补充工作区地图、影响范围判断、跨项目执行顺序、默认特殊路径识别和并发写入边界
- 强化 `workspace-overview.md` 和 `cross-project-rules.md`，未配置 contracts/shared/infra 时也会给出默认识别规则
- workspace 根 skill 和概览会标记独立初始化子项目，并提示按子项目自己的 `.skills-seed/config.yaml` 查找 provider 与 skill 路径

### 体验

- workspace 配置保存保持模板注释与双引号风格，避免回退到全文件 YAML marshal
- workspace 默认跳过当前 provider 已存在的子项目 skill，只补齐/刷新根 workspace skill，避免覆盖子仓已有 agent 配置
- CLI flag 帮助和 workspace 生成日志补充 i18n 文案
- workspace 子项目学习日志对齐单项目模式，补充子项目开始、分析结果、保存模式、保存画像和跳过原因输出
- workspace 子项目学习的 Token 消耗延迟到子项目日志末尾输出，并标明对应子项目
- `learn current` 单项目模式下 Token 消耗固定作为学习输出最后一条日志，workspace 模式下按子项目完成顺序输出，避免并发日志错位

### 文档

- 重写 README 结构，补充单项目和 workspace 快速开始、初始化锁定、配置和常用命令
- 更新 `docs/` 架构与生成链路文档，并补充对应英文文档

## [v0.0.3]

### 功能

- 支持 `skills-seed init --mode workspace` / `--workspace` 初始化多子项目工作区
- 新增 `skills-seed reset --mode ...`，切换初始化模式时默认备份旧 `.skills-seed`
- 配置新增 `project.mode`、`workspace.projects` 和 `agent.parallelism`
- workspace 模式下支持按子项目并发学习，并为 patterns 写入 `project_id`、`scope_path`、`workspace_role`
- workspace 模式下生成根 `.claude/.agents` 入口 skills，并为子项目生成各自 `.claude/.agents` skills
- 生成项目级 `project-spec.json` 和 `references/project-spec.md`，workspace 子项目也拥有独立项目规范

### 模板

- 新增 `embedfs/templates/prompts/common/workspace-*` 工作区通用提示词
- 新增 `embedfs/templates/prompts/workspace/*` 工作区初始化提示词模板
- 新增 `embedfs/templates/skills/common/workspace/*` 工作区根 skills 与 references 模板
- 工作区通用提示词补充严格 JSON 输出、路由规则、影响半径、跨项目改动顺序和并发 Agent 约束
- 统一配置模板顶层模块注释风格，所有模块标题使用 `# ========================================` 包裹
- 子项目继续复用 `embedfs/templates/prompts/project/` 与现有 project skills 模板，并在生成内容中引用 `references/project-spec.md`

### 兼容性

- 开始学习或生成后会锁定初始化模式，避免在 project/workspace 之间直接切换导致数据结构混用

### 体验

- 调整 Agent Token 消耗的控制台输出顺序，避免打断正在执行的进度步骤完成日志

## [v0.0.2]

### 功能

- 支持 `learn current --focus ... --profile refresh` 基于已有项目画像和聚焦路径做增量项目画像刷新
- 项目画像分析 prompt 支持保留旧画像中的未变更模块、工具方法、业务方法、依赖和架构信息
- `learn current` 日志增加增量画像相关诊断信息，便于确认是否走增量刷新

### 文档

- README 增加精准学习、局部学习和项目画像刷新命令示例
- 整理中英文 Markdown 文档与 Go 注释风格

### 体验

- 初始化完成后的后续步骤提示改为可选后续步骤

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
