# 更新日志

[简体中文](CHANGELOG.md) | [English](CHANGELOG.en.md)

## [v0.2.0]

### 变更

- 模板国际化约定翻转：中文模板文件名不再带 `.zh-CN` 后缀（如 `learn-analyze.txt.tmpl`），英文模板显式标注 `.en-US`（如 `learn-analyze.en-US.txt.tmpl`）；`zh-CN` 成为所有模板加载的默认 locale
- 所有 prompt 和 skills 模板统一使用 `域名-功能` kebab-case 命名，替换原先的 snake_case / 混合命名：
  `analyze` → `learn-analyze`、`batch-learn` → `learn-batch`、`generate_fixes` → `fix-generate`、`generate_skills_summary` → `skill-project-summary`、`merge-patterns` → `pattern-merge`、`project-analysis` → `project-analyze`、`init-skills` → `skill-project-init`、`workspace-profile` → `skill-workspace-profile`、`workspace-spec` → `skill-workspace-spec`、`skill` → `project-skill`、`workspace/SKILL` → `workspace-skill`
- Skills 模板引入中央目录（catalog）注册机制，通过 `TemplateEntry` 声明式定义模板 ID、路径和 provider 白名单，取代原有 `fs.WalkDir` 动态扫描

### 功能

- 新增 `DefaultExcludePatterns()` 提取为独立函数，初始化时写入完整的静态排除规则到配置文件
- 默认排除规则从 7 条扩展到 31 条，覆盖常见构建产物（`dist`、`build`、`out`、`target`）、临时文件（`*.tmp`、`*.bak`、`*.swp`）、压缩包（`*.zip`、`*.tar.gz`）、图片和视频资源等
- 文件过滤器新增基名 glob 匹配：不含 `/` 的模式（如 `*.log`）会同时对文件基名和完整路径进行匹配

### 文档

- 更新配置参考文档中 `exclude` 默认值表格，反映扩展后的排除规则列表

## [v0.1.0]

### 修复

- 修复 `skills-seed init --workspace --children` 在子项目初始化失败后仍保留根目录 `.skills-seed` 的问题，避免下次重试时误报“已初始化”
- 优化终端输出顺序：运行中的步骤先完整显示进度标题，普通日志和 Token 明细延迟到步骤完成后输出；workspace 子项目生成的 Token 明细会保留子项目归属

## [v0.0.9]

### 功能

- 新增 pattern 质量指标，保存和合并模式时自动计算项目特有性、证据数量、泛化惩罚和综合分
- `check` 会记录带 `PatternID` 的问题命中，沉淀每条模式是否在后续检查中真正被使用
- 新增 `skills-seed patterns stats`，展示模式分类、特有性、置信度、综合分、命中次数和最近命中时间
- 新增 `skills-seed review import --from-file` 和 `skills-seed review stats`，可导入本地评审评论并按文件与行号窗口统计已有模式命中的防漏效果

### 体验

- 已知 patterns 快照增加质量指标，后续学习可参考已有规则质量，降低泛化规则继续放大的概率
- 模式统计按命中次数和综合分排序，便于识别高价值规则和长期未命中的规则
- `generate-skills` 会按 `EffectiveScore*0.6 + normalized(HitCount)*0.3 + Confidence*0.1` 对 patterns 排序，并把质量指标与命中统计传给 Agent，优先沉淀项目特有且被实际命中的规则
- 评审评论统计默认使用 `±3` 行匹配窗口，并展示总评论数、已预防评论数、遗漏评论数和命中模式计数

### 文档

- 更新 README 和命令参考，说明 pattern 质量指标、`patterns stats`、review 评论导入统计，以及 `generate-skills` 的质量排序策略

## [v0.0.8]

### 文档

- 精简 README，把完整命令参考迁移到 `docs/COMMANDS.md` / `docs/COMMANDS.EN.md`
- 新增配置参考文档 `docs/CONFIGURATION.md` / `docs/CONFIGURATION.EN.md`，集中说明配置字段、默认值、路径语义和联动行为
- 命令参考按顶层命令组织，使用“命令概述 / 命令形式 / 参数 / 子命令参数 / 注意事项”等标准结构
- 补齐所有命令、子命令和参数说明，包括 `help`、`completion`、`--context`、`--profile`、workspace、hook、patterns 和 profile 等用法
- README 改为介绍页，重点说明核心能力、工作方式、Agent 支持和快速入门；文档入口指向独立命令参考和配置参考
- README 顶部改为居中展示区，集中展示项目定位、语言切换、支持的 Agent 和核心文档入口

### 体验

- 补齐所有业务子命令的 `Long`、`Example` 和参数 help，`skills-seed <command> --help` 可直接查看完整用法
- 精简根命令 help 顶部介绍，减少 `skills-seed help` 的冗余说明
- 增加双语 help 覆盖测试，防止新增命令缺失 help 或泄漏未翻译 i18n key
- `init` 成功输出改为同一行显示相对 `.skills-seed` 路径，并输出当前版本 tag 对应的 README 文档地址
- 移除 `init` 和 `learn current` 末尾的可选后续步骤提示，避免命令输出过长
- `init` 新增 `--agent` 参数，单项目和 workspace 根仓初始化时可直接指定 `claude`、`codex` 等 provider
- 新增 `skills-seed init --workspace --children`，根仓初始化后可同步初始化 `workspace.projects` 中缺失 `.skills-seed` 的子项目
- 新增 `workspace.init_children` 配置，默认 `false`；开启后 `learn current` 会在学习前补初始化缺失 `.skills-seed` 的子项目
- 新初始化的 workspace 子项目会继承根仓 `agent.provider`、`agent.commands` 和 `output.skills_paths`；已有不同 agent 的子仓只提示并跳过

## [v0.0.7]

### 变更

- `generate-skills` 固定调用当前 Agent 做摘要合并，移除 `generation.mode` 配置项
- CodeGraph 默认开启 `auto_init`，目标项目缺少索引时会自动初始化
- 精简默认 `config.yaml` 分区标题，降低配置文件阅读成本
- 修复 workspace 子项目并发任务取消语义，并复用 learn/generate 的子项目容器校验逻辑
- 收敛 learn current 排除规则：代码内置只保护 `.git/**`、`.skills-seed/**`、`.claude/**`、`.agents/**`，其他可选工具/项目产物放入默认 `exclude`
- 默认 `exclude` 使用 glob 风格的 `.*` 屏蔽任意层级点号开头文件/目录，覆盖 `.github`、`.cursor`、`.codegraph`、`.env` 等本地或工具产物
- 将源码和中文配置模板中的英文说明性注释改为中文，保留必要的代码标识、命令名和英文版模板内容

## [v0.0.6]

### 功能

- `learn current` 和 `generate-skills` 新增 `--context` / `--context-file`，支持为单次学习或生成传入用户补充说明
- workspace 根 skill 生成在 `generation.mode = ai` 或传入上下文时，会额外分析工作区事实画像和开发规范，再合并到根 skill references
- workspace AI 分析新增结构化项目职责、框架/运行时、子项目依赖、影响路由、工作区特定规则、改动顺序和并发边界
- Claude 和 Codex Agent 新增 workspace profile / spec 分析能力，并解析为 `WorkspaceProfile` / `WorkspaceSpec`

### 模板

- 精简 workspace 根 `SKILL.md`，将入口 skill 聚焦为路由、子项目 skill 选择和跨项目规则判断
- 扩展 `workspace-overview.md`，写入用户补充说明、AI 分析出的工作区事实、依赖关系、影响路由、职责和框架信息
- 扩展 `cross-project-rules.md`，写入工作区特定规则、路由、改动顺序、必须同时读取多个 skills 的场景和并发 Agent 约束
- 更新学习、画像和生成提示词，改为通过文件路径读取大型输入和一次性用户上下文

### 体验

- Agent 调用的大型输入改为写入 `.skills-seed/memory/runtime` 下的临时文件，减少提示词正文体积
- workspace 生成对子项目执行时会屏蔽根级一次性上下文，避免根 workspace 说明误注入子项目 skill
- 当用户上下文存在时，即使默认模板生成模式也会要求可用 Agent，用于把上下文并入生成结果

### 文档

- 清理过期的项目架构、生成链路和增量学习设计/计划文档

## [v0.0.5]

### 功能

- `learn current` 新增文件 md5 增量学习，成功学习后记录普通项目文件指纹
- 未检测到可学习文件变化时，同时跳过 patterns 学习和项目画像刷新
- workspace 根仓 `learn current` 会进入各独立 Git 子仓，用子仓自己的 `.skills-seed` 执行增量学习
- workspace 根仓只刷新工作区画像和跨项目关系，不保存子仓 patterns 或文件指纹
- 删除文件只触发基于已有画像的增量画像刷新，不再无意义提取 patterns
- `generate-skills` 新增 `generation.mode` 配置，默认 `template` 不额外调用 AI，`ai` 模式保留生成前摘要合并
- workspace 根仓 `generate-skills` 会先进入各独立 Git 子仓，用子仓自己的 `.skills-seed` 生成子仓 skill，最后再生成根 workspace skill

### 体验

- 默认排除配置的 skills 输出目录以及 `.claude/skills/**`、`.agents/skills/**`，避免生成内容回流到下一轮学习
- 当前代码学习会把已有 patterns 摘要传给 Agent，降低同一规则换名重复输出的概率
- 学习日志补充增量文件统计和 generated skills 排除提示
- 已有手写 `SKILL.md` 没有 `generated-by: skills-seed` 标记时默认不覆盖；workspace 生成会跳过该子仓 skill 并继续生成根 skill

### 文档

- 更新 README、生成链路文档和配置模板，说明 md5 增量学习、workspace/子仓解耦、生成模式配置和 generated skills 默认排除

## [v0.0.4]

### 功能

- workspace 初始化只扫描第一层目录，并扩展常见项目标记识别范围
- workspace 模式下按当前 `agent.provider` 生成根入口 skill，子项目 skill 由子仓自己生成
- workspace 根 skill 路由引用子项目独立 skill 路径，避免根仓写入子仓输出目录
- workspace 根 skill 也生成 provider 元数据，Codex 输出时包含标准 `agents/openai.yaml`
- workspace 子项目存在 `.skills-seed/config.yaml` 时视为独立初始化，外层 workspace 不生成或覆盖该子项目 skill

### 模板

- 强化 workspace 根 skill 内容，补充工作区地图、影响范围判断、跨项目执行顺序、默认特殊路径识别和并发写入边界
- 强化 `workspace-overview.md` 和 `cross-project-rules.md`，未配置 contracts/shared/infra 时也会给出默认识别规则
- workspace 根 skill 和概览会标记独立初始化子项目，并提示按子项目自己的 `.skills-seed/config.yaml` 查找 provider 与 skill 路径

### 体验

- workspace 配置保存保持模板注释与双引号风格，避免回退到全文件 YAML marshal
- workspace 根仓只补齐/刷新根 workspace skill，避免覆盖子仓已有 agent 配置
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
