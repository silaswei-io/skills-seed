# 更新日志

[简体中文](CHANGELOG.md) | [English](CHANGELOG.en.md)

## [v0.9.3]

### 修复

- 修复直接保存 Pattern 时分类大小写或空白可写入非规范 bucket 的问题，保存和按分类查询都会先归一化分类。
- 修复历史非规范分类 bucket 中同 ID Pattern 在重新保存后可能残留旧副本、导致统计重复的问题；删除 Pattern 时也会清理所有历史分类副本。
- 修复相似 Pattern 查找未归一化分类，导致兼容别名或大小写不同的分类无法命中已有规范分类模式的问题。
- 修复 `patterns compact --category` 对大小写、空白或兼容别名分类无法命中规范分类的问题。
- 修复 `learn current` 项目初始化 prompt 的 JSON 示例使用说明文字作为 `category` 值的问题，避免模型照抄非法分类字符串。

## [v0.9.2]

### 变更

- 模式分类契约集中到 domain 层，prompt、curator 校验和保存路径共用同一份合法分类列表。
- `learn history`、`learn current`、`patterns add` 和 `pattern-curate` 的中英文 prompt 统一展示合法分类，减少 AI 输出未支持分类。

### 修复

- 修复 AI 输出 `security` 分类时导致模式策展校验失败并回退的问题；该兼容别名现在归一为 `utils`。
- 修复 curator 校验失败日志误显示为“模式策展结果解析失败”的问题，现在区分解析失败和校验失败。

## [v0.9.1]

### 功能

- 新增 `learn current` 的 AI 相关文件筛选，可在候选文件较多时先根据文件树和变更元数据收敛分析范围。
- 新增 `skills-seed patterns delete`，支持按 pattern ID 删除模式，并在 workspace 根目录同步处理已关联子项目模式。
- 新增 skills dirty state 和 `generate skills --force`，生成阶段可跳过未变化目标，只重新生成受学习、pattern 或 workspace 关系影响的 skills。
- 新增更稳健的 AI JSON 修复流程，覆盖重复对象起始、非法转义、字符串内未转义引号和缺失闭合容器等常见模型输出问题。

### 变更

- 配置结构调整为 `learning.current` 和 `learning.history`：结构化上下文从 `analysis.structural` 移到 `learning.current.structural`，历史学习默认值移到 `learning.history`。
- `learn current --profile auto` 仅在缺少画像或本次实际写入/更新模式时刷新项目画像，减少无意义 Agent 调用。
- workspace 关系分析输入未变化且产物存在时会跳过重新分析，并只标记受影响的 workspace/子项目 skills 待生成。
- 生成的 skills/references 增加项目验证命令、模块与业务方法证据提示，减少硬编码项目指导。

### 修复

- 移除根命令中的 `completion` 命令，并删除命令文档中的 completion 章节。
- 修复中文 locale 下 `help`、`preview`、`review`、`patterns show/stats` 等命令露出英文描述、flag 或表头的问题。
- 修复英文 README 根示例仍使用旧 `skills-seed add .` 的问题，统一为 `skills-seed workspace add .`。

## [v0.9.0]

### 功能

- 新增模式策展服务 `curator`，作为候选模式写入本地模式库前的统一边界：检索相关历史模式、调用 AI 做去重/整合/丢弃、服务端校验输出，再写入数据库。
- 新增 `pattern-curate` prompt，要求 AI 在入库前验证候选覆盖、重复规则整合、代码证据来源、统计一致性和低质量候选丢弃。
- 新增显式维护命令 `skills-seed patterns compact`，用于人工触发已有模式库整理；支持 `--category` 和 `--dry-run`。

### 变更

- `learn current`、`learn history`、`learn staged/commit` 和 `patterns add` 现在只产出候选模式，所有新增、更新、合并或丢弃都由 curator 入库边界负责。
- `generate skills` 改为只读模式库，不再合并或修正 patterns；生成阶段只负责读取已沉淀的项目画像、workspace 画像/spec 和 patterns 并生成 skills。
- `sync` 流程简化为 `learn current -> generate skills` 或 `patterns add -> generate skills`，模式策展发生在学习/添加入库阶段。
- 项目结构摘要、样例文件收集和结构化预扫描统一走配置化文件选择策略，除内置安全边界和 `exclude` 外，不再在 analyzer 中维护额外目录关键字机制。
- Skills 模板和生成引用进一步收紧为语言无关、证据驱动表达，避免生成器合成硬编码项目指导。

### 破坏性变化

- 移除 `skills-seed generate skills --merge`。
- 移除旧命令 `skills-seed patterns merge`，请改用 `skills-seed patterns compact`。
- 移除旧的 `internal/service/merger`、`pattern-merge` prompt 和 `MergePatterns*` Agent API。

## [v0.8.1]

### 功能

- 业务模式 reference 改为索引 + 子域详情结构：`business.md` 只保留读取指引和子域链接，详细规则与代码证据写入 `references/patterns/business/*.md`，避免单文件上下文过大。
- 业务模式子域按代码位置、scope 和稳定目录名自动聚类；无法稳定归属的规则归入 `other`，避免在通用生成器中写死具体项目业务词。
- 生成的主 skill 和 project spec 会根据实际生成的 reference 条件化链接，稀疏项目或跳过 references 时不再产生坏链接。

### 变更

- 项目初始化、增量学习和模式合并 prompt 强制 `good_example` 只能来自已读取源码的完整语义片段，禁止合成或改写“正确示例”。
- Skills 模板中的示例标题从 “Good Example/正确示例” 调整为 “Code Evidence/代码证据”，降低模型把示例当成可自由创作代码的概率。
- 项目规范不再限制业务规则数量，保留所有可执行业务规则，由 reference 拆分控制上下文体积。

### 修复

- 修复 `GenerateSkillsWithOptions` 丢弃传入选项的问题，`SkipReferences` 现在会真正跳过 reference 文件生成。
- 修复生成输入未变化时只检查 `business.md` 而忽略业务详情文件的问题，避免业务详情缺失时误跳过重新生成。
- 修复旧模板中残留的 `skills-seed generate-skills` 命令引用，统一为 `skills-seed generate skills`。

## [v0.8.0]

### 功能

- Agent 调用输出会单独归档到 `.skills-seed/memory/runtime/agent-outputs/`，包含最终内容、原始 CLI 输出、stderr 和 manifest，便于排查模型返回而不污染运行日志。
- 业务方法代码位置全面改为 `code_location` 结构化元数据，保留当前位置、历史位置、状态和语言无关符号快照；生成的 business methods reference 会展示位置状态。

### 变更

- 运行日志不再输出 Agent 回复预览、stdout/stderr 明文或 JSON 片段，只保留长度和 runtime 归档路径。
- 初始项目学习和模式合并等 prompt 示例统一使用 `code_location.current_location`，并把示例 JSON 包在说明用代码块内，同时继续强制实际回复不得使用 markdown。
- 生成的项目 skill 和 references 更紧凑：入口文档按任务读取最小必要参考，项目规范聚焦可执行规则，项目概览减少重复结构正文。
- Profile 保存前会清理不可用业务方法；业务方法必须有名称和可展示位置才进入最终画像。

### 修复

- 修复 `learn current` 在 Agent 输出末尾缺少 JSON 闭合容器时直接失败的问题；现在会对缺失的 `}` / `]` 做保守恢复，但不会修复半截字符串或真正非法 JSON。

## [v0.7.4]

### 修复

- 优化项目数据库被占用时的错误提示。当 BoltDB 无法在超时时间内获取 `.skills-seed/memory/project.db` 锁时，CLI 会提示数据库可能正在被其他 `skills-seed` 命令使用，并给出等待或检查残留进程的处理建议。

## [v0.7.3]

### 功能

- 新增 `skills-seed patterns show`，可查看 DB 中 pattern 的时间字段、来源、代码位置状态和语言无关符号快照；支持单条详情和 JSON 输出。

### 变更

- Pattern、文件分析指纹、pattern 命中、评审评论和已分析提交记录会维护 `created_at/updated_at`。
- 业务方法代码位置新增结构化 DB 元数据，保留历史位置、当前位置、状态、变化类型和语言无关符号快照；生成文档优先展示当前位置，并保留历史位置和状态。

### 修复

- 修复 `learn current` 在 pattern 保存失败时仍提交文件分析指纹的问题，避免未成功学习的文件在后续增量学习中被误判为已学习。

## [v0.7.2]

### 修复

- 修复 `AnalyzeProject` 返回的 JSON 在对象数组中偶发出现重复对象起始片段时无法解析的问题，覆盖类似 `{"{"name": ...` 的模型输出畸形。
- 修复项目画像分析解析失败后仍保存 `unknown/解析失败` fallback 画像的问题；解析失败现在会返回错误，避免覆盖已有有效画像。
- 修复 `learn current` 控制台显示“已保存项目画像”但实际保存的是解析失败占位画像的误导性结果。

### 文档

- 更新 README 和更新日志，说明 0.7.2 的项目画像 JSON 恢复和解析失败保护。

## [v0.7.1]

### 功能

- Prompt 渲染会清理默认脚手架和生成元数据，只把用户实际填写的项目约束、工作区约束和指令片段合并进 Agent 输入。
- 渲染后的 prompt 会写入运行时目录，并附带 manifest 记录各片段是否参与合并、原始长度和最终长度，便于排查提示词上下文。
- `learn current` 的文件选择、排除、增量指纹和提交记录逻辑抽到 `fileanalysis` 服务，分析、预览和学习流程共用同一套策略。

### 变更

- 项目 prompt 模板默认正文改为空注释提示，避免初始化后把“通用说明”误当成用户自定义约束反复追加。
- 结构化分析和样本文件选择默认只纳入源码、构建配置和依赖配置，继续跳过文档、生成产物和已生成 Skills 输出目录。
- Skills 模板中的 skills-seed 生成说明改为受配置控制，默认不写入最终生成文件。
- 默认配置模板、源码注释和常量说明改为中文为主、必要处中英混合，保留 Agent、Skills、CLI、tree-sitter 等技术名。

### 文档

- 更新 README、配置说明和更新日志，补充 0.7.1 的 prompt 合并清理、运行时调试 manifest、统一文件选择策略和注释说明策略。

## [v0.7.0]

### 破坏性变更

- 移除 CodeGraph 集成和 `analysis.codegraph` 配置项，不保留旧字段兼容。
- 结构化分析配置改为 `analysis.structural`，仅保留 `enabled`、`max_symbols` 和 `max_file_size`。
- `max_nodes` 重命名为 `max_symbols`，含义明确为输出到结构化上下文的最大符号数。

### 功能

- 新增基于内嵌 tree-sitter 的轻量结构化预扫描，提取符号、导入、入口点和模块线索，不再依赖外部命令或本地索引。
- 结构化预扫描只在存在 focus、diff、sample 或入口文件等边界输入时运行，避免无边界全仓扫描。
- 当前代码学习支持新增、修改、删除三类文件状态；分析完成后会按作用范围覆盖快照，使下一次学习可以基于干净快照计算增量 diff。

### 文档

- 更新 README、命令参考和配置参考，说明 0.7.0 的内嵌结构化预扫描、`analysis.structural` 配置和 CodeGraph 移除。

## [v0.6.4]

### 功能

- 新增 `generate skills --no-references` 标志，跳过参考文档（`references/` 目录）生成；SKILL.md 和 Agent 元数据始终生成。

### 变更

- Generator 重构为纯编排层，非职责代码归还各层：
  - 提取 `SkillWriter`（`writer.go`）封装所有模板渲染与文件写入逻辑。
  - 纯函数移入 domain 层：`CleanProjectProfile`、`RankPatternsForGeneration`、`NewProjectSpecFromProfile` 等。
  - Workspace 生成流水线拆为独立子包 `internal/service/workspace/`，与单项目生成彻底解耦。
- `GeneratorService` 依赖从 10 个降至 5 个（`patternRepo`、`profileRepo`、`agent`、`configRepo`、`writer`）。

## [v0.6.3]

### 功能

- 新增 `--skills-locale` 参数，将工具输出/配置模板语言与生成 Skills、提示词语言分离。

### 变更

- 配置新增 `skills.locale`，默认生成英文 Skills；`profile.locale` 继续控制 CLI 输出和配置模板语言。
- Agent prompt、项目 prompt、Skills 模板和 workspace 生成流程统一读取 Skills 语言配置，减少工具界面语言对沉淀内容语言的影响。

### 文档

- 更新命令参考和配置参考，说明 `--locale` 与 `--skills-locale` 的职责差异。

## [v0.6.2]

### 修复

- 修复 workspace 根关系分析和 skills 生成在输入未变化时仍重复调用 Agent / 重写输出的问题；现在会记录输入 md5，输入不变且产物完整时直接跳过。
- 修复实际 CLI help 与命令参考不同步的问题，移除 `generate skills` 示例中已不存在的 `--context` 用法，并修正 `sync --context`、`patterns add --files` 等 flag 说明。

### 变更

- workspace 子项目快速跳过/完成步骤统一使用全局 `200ms` 短暂停顿，替代原先分散的固定等待，减少无变化场景下的终端空等。

### 文档

- 更新命令参考，说明 workspace 根关系分析和 `generate skills` 的输入 md5 跳过行为。
- 同步命令参考与实际 CLI help，修正 `init` / `reset` 默认值、`learn history --batch-size` 默认来源、`patterns add --files` 重复传参和 `sync --context` 作用范围说明。

## [v0.6.1]

### 修复

- 修复 workspace 学习阶段只把 `workspace.projects` 配置骨架写入 `workspace-profile.json`，导致根 workspace skill 无法继承子项目画像和用户学习说明的问题。
- 修复 workspace 模式下进入子项目学习/生成时仍可能在根工作区路径执行 Agent 的问题；Agent 调用会按当前子项目 `.skills-seed` 解析工作目录。
- 修复生成阶段可传入一次性用户说明并进入 skill 摘要的边界问题；`generate skills` 不再接收 `--context` / `--context-file`，只消费学习阶段已经沉淀的 profile/spec/patterns。
- 修复 workspace 子项目学习完成后，根仓 workspace profile/spec 分析阶段缺少终端进度输出、看起来像卡住的问题。
- 收紧 skills 输出目录校验，禁止 workspace 根或子项目把生成结果写出对应项目根目录，避免跨项目污染。

### 变更

- `learn current --context` / `--context-file` 仍作为学习阶段一次性输入；workspace 学习会把该说明传给 workspace profile/spec 分析，但提示词明确禁止把说明原文或长段转述写入持久化画像/规范。
- workspace 根学习现在会读取子项目已沉淀的 `project-profile.json` 摘要、框架和关键模块，生成并保存更完整的 `workspace-profile.json` 与 `workspace-spec.json`。
- workspace 画像/spec 合并逻辑提取到 `internal/workspace`，学习阶段和生成阶段共用同一套保底路由与合并规则。

### 文档

- 更新 README、命令参考和配置参考，说明 0.6.1 的一次性用户说明边界、workspace 学习沉淀流程，以及 `generate skills` 不再接收 context 参数。

## [v0.6.0]

### 破坏性变更

- 配置顶层 `project` 重命名为 `profile`，用于描述当前配置文件所属项目或工作区本身，避免和 `profile.mode: "project"` 混淆。
- 移除用户配置中的 `workspace.shared`、`workspace.contracts`、`workspace.infra`。workspace 公共路径、契约路径和基础设施路径改由学习/生成阶段根据仓库证据和用户上下文分析进入 workspace profile/spec，不再要求用户手填。
- workspace 子项目发现规则收紧为“第一层目录中拥有独立 `.git` 的目录才是子项目”。`go.mod`、`package.json`、安装脚本、Helm/Terraform 等文件只用于识别项目类型和语言，不再决定项目是否存在。

### 功能

- workspace 初始化会把根仓 `profile.language` 留空，适配一个工作区包含多种语言子项目的场景。
- `init` 自动写入 `profile.git_remote`，从当前仓库 `origin` 远程地址读取。
- Shell 安装/底座类仓库可被识别为 `type: "infra"`、`language: "shell"`，例如包含 `install.sh`、`_install.sh`、`install.ini` 的独立 Git 子仓。

### 体验

- 默认 `config.yaml` 改为大块模块注释和字段前置说明，模块之间保留空行，注释行不再使用句号结尾。
- `workspace.projects` 成为 workspace 配置中唯一需要用户关注的字段，减少 project/profile/workspace/shared/infra 概念混杂。
- 保存旧配置时会按新结构重写配置文件并移除已废弃的 workspace 路径字段。

### 文档

- 更新 README、命令参考和配置参考，说明 0.6.0 配置结构、workspace 子项目边界规则和已移除的路径配置项。

## [v0.5.0]

### 破坏性变更

- `skills-seed add` 迁移为 `skills-seed workspace add`
- `skills-seed generate-skills` 拆分为 `skills-seed generate skills`
- 移除旧的 `internal/command/add` 包，逻辑统一到 `internal/command/workspace`

### 功能

- 新增 `skills-seed patterns add <描述>`，支持用自然语言定义编码模式，AI 结合代码生成结构化 pattern
- 新增 `skills-seed sync` 一键同步命令：
  - `sync` = learn current → patterns merge → generate skills
  - `sync --add <描述>` = patterns add → patterns merge → generate skills
- 新增 `skills-seed generate` 父命令和 `generate skills` 子命令，为后续更多生成类型预留扩展
- 新增 `skills-seed workspace` 父命令和 `workspace add` 子命令，使命令结构更清晰
- AI Agent 新增指数退避重试机制（429 / 529 / overloaded），重试次数和间隔可在 `config.yaml` 的 `agent.retry` 中配置；当前进度行会区分正常、等待重试和重试中状态，并显示 Agent 错误、本次调用耗时和退避等待
- 新增 `UserPatternDefiner` Agent 接口，支持用户自定义模式生成
- 新增用户定义模式 prompt 模板（`user-define-pattern`），支持中英文
- 用户定义的模式自动标记 `source: user_defined`

### 变更

- 命令路由表更新：`generate/skills`、`sync`、`workspace/add`、`patterns/add` 需要项目运行时
- `commandNeedsProjectRuntime` 移除不可达代码

### 文档

- 更新 README、命令参考和配置参考，覆盖 0.5.0 命令结构、`patterns add`、`sync` 和 `agent.retry`

## [v0.4.4]

### 优化

- 优化运行时 prompt 的 JSON 输出约束，移除示例中的 markdown 代码块，降低 Agent 返回 fenced JSON 导致解析失败的概率。
- 收紧 `learn current`、`learn history`、`generate-skills` 和 `check` 修复生成相关 prompt 的读取范围：优先读取目标文件、变更文件、CodeGraph 结构化上下文和直接相关调用关系，避免提示 Agent 无差别扫描整个仓库。
- 优化项目初始化分析 prompt：不再列举固定框架/ORM/日志库清单，只提取项目实际使用的技术栈，减少模板示例诱导误判。

### 变更

- `fix-generate` 的 `summary` 和 `warnings` 字段现在会被解析并在 `check` 生成修复时输出；无法安全完整重写的文件可通过 `warnings` 提示人工审查。
- `skill-project-summary` 的 `key_insights` 和 `improvement_suggestions` 现在会进入生成的项目 `SKILL.md`，让 Agent 能看到摘要阶段提炼出的关键洞察和改进建议。
- `pattern-merge` 合并结果会保留正例、反例和业务方法信息，合并后的 pattern 不再丢失这些后续生成 skills 可用的字段。

### 修复

- 修复中文 `skill-project-summary` prompt 中 `concurrency` 分类拼写错误。

## [v0.4.3]

### 修复

- 修复未指定 `--locale` 时 Windows 默认生成英文配置的问题；未显式指定时稳定使用中文。
- 修复根项目 `init` 未自动识别前端/Node 项目语言、导致配置默认写成 `go` 的问题。
- 优化 Windows 路径兼容：支持 `~\path` 展开，并避免调用 Windows 不兼容的 Unix `tree` 参数。

### 发布

- 新增 Windows arm64 release 包。

## [v0.4.2]

### 修复

- 修复 Windows 未初始化目录中执行 `skills-seed help` 时路径上溯无法在盘符根目录退出，导致命令卡死的问题。
- 修复 `help`、`--version`、`completion`、`init`、`hook` 等不依赖项目运行时的命令在项目学习占用数据库时无法使用的问题；`reset` 仍需要项目运行时保护，避免学习过程中重置 `.skills-seed`。
- 修复 `skills-seed reset help` 会被当作 `reset` 执行的问题；不接收位置参数的命令现在会拒绝多余参数，避免误触发业务逻辑。

## [v0.4.1]

### 变更

- 优化 `.skills-seed/prompts/` 语义：用户文件现在作为项目上下文、workspace 约束和补充指令与内置 prompt 合并，不再表达为替换内置 prompt 的覆盖模板。
- 将用户补充指令目录调整为 `.skills-seed/prompts/instructions/<prompt-id>.md`，并将项目级 prompt 片段调整为 `.skills-seed/prompts/project/<prompt-id>.md`。
- workspace prompt 初始化文件名改为 canonical runtime prompt ID：`skill-workspace-profile.md` 和 `skill-workspace-spec.md`。
- `project-profile.md` 默认内容改为事实记录式“未记录”，避免把“请补充/请分析”类任务指令混入运行时 prompt。
- 新增内置 `output-contract-guard` prompt 模板，在用户补充指令后追加最终输出契约，保护 JSON / Markdown 输出格式。

### 文档

- README / README.en 新增 prompt 合并和一次性 `--context` / `--context-file` 说明。
- 更新命令参考与配置参考，说明 `.skills-seed/prompts/` 的目录用途、合并顺序、最终输出契约，以及一次性说明参数和持久补充指令的区别。

## [v0.4.0]

### 修复

- 优化 `generate-skills` 工作区进度输出：第一行显示子项目总完成进度，子项目行显示各自 5 步详细进度，避免旧的 `1/1 写入技能文件` 和根/子项目进度重叠。
- 修复快步骤进度缺少可见动画的问题，短耗时步骤也能看到稳定的 spinner 和耗时反馈。
- 修复 Agent 返回 JSON 中非法转义导致解析失败的问题，并统一 JSON 文件读写逻辑。

### 体验

- 优化 `.skills-seed/config.yaml` 注释排版，改为更清晰的块状注释，减少行尾注释噪音。
- `learn` 与 `generate-skills` 复用工作区子项目进度命名逻辑，保持输出风格一致。

## [v0.3.0]

### 破坏性变更

- 配置命名从 `agent.provider` / `output.skills_paths` 调整为 `agent.engine` / `skills.target` / `skills.paths`，明确区分“执行分析、学习和摘要的 Agent CLI”和“生成的 skills 目标格式”
- 移除 `workspace.init_children` 和 `init --children` / `init children` 语义；workspace 初始化时会直接初始化当时检测到的子项目

### 功能

- 新增 `skills-seed add .`，可在 workspace 根仓自动检测并添加所有当前子项目，同时初始化缺失 `.skills-seed` 的子仓
- 新增 `skills-seed add <child...>`，支持按子仓 ID 或路径添加指定子项目；`./frontend`、`frontend/`、`frontend\` 会归一化为同一目标
- `add` 会先初始化子仓，再同步更新根仓 `workspace.projects`；如果子仓初始化失败，不会污染根仓配置
- workspace 初始化现在默认同步初始化检测到的子项目，新建子项目继承根仓 Agent 和 Skills 配置，已有子项目配置保持不覆盖
- `generate-skills` 默认输出路径改为根据 `skills.target` 查询 `skills.paths`，支持用 `claude` 执行生成摘要并输出 `codex` skills

### 文档

- 重写 README / README.en，把项目定位、工作流、workspace 行为、`add` 命令、Agent engine 与 skills target 的关系整理为正式入口文档
- 更新命令参考、配置参考、CLI help 和 prompt 文案，移除旧 `provider` / `output.skills_paths` / `init children` 说明

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
- `skills-seed patterns show`

### 发布

- 添加 GitHub Actions CI，自动执行格式检查、依赖一致性检查、`go vet` 和单元测试
- 添加基于 GitHub Actions 原生命令的 Release 打包流程
- 发布 Linux、macOS 和 Windows 的 x86_64 / arm64 包（Windows 当前发布 x86_64）
- 在 GitHub Releases 中附带校验和与版本说明
