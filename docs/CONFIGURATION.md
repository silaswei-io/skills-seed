# Skills Seed 配置说明

[简体中文](CONFIGURATION.md) | [English](CONFIGURATION.EN.md)

配置文件位于 `.skills-seed/config.yaml`。`skills-seed init` 会按当前项目生成默认配置；大多数路径都相对项目根目录或 `.skills-seed` 目录，具体以字段说明为准。

## 0.8.x 配置结构

0.8.x 继续沿用 0.7.x 配置结构，不保留旧字段兼容：

- 顶层 `project` 改名为 `profile`，表示当前配置文件所属项目或工作区本身，不表示 `project` 运行模式。
- `workspace` 下只保留 `projects`，不再提供 `shared`、`contracts`、`infra` 给用户手填。
- workspace 公共库、契约和基础设施影响会在 `learn current` 阶段根据仓库证据、子项目画像和一次性用户说明分析并沉淀到 workspace profile/spec，不从配置文件读取；生成阶段只消费已沉淀结果。
- workspace 根配置的 `profile.language` 默认留空，因为一个工作区可以包含多种语言子项目。
- `analysis.codegraph` 已移除，结构化预扫描改为 `learning.current.structural`，基于内嵌 tree-sitter，不需要外部 CodeGraph 命令或索引。

## 配置示例

### 默认结构

```yaml
profile:
  name: "your-project"
  mode: "project"
  language: ""
  locale: "zh-CN"
  git_remote: ""
  root_path: ""
  initialized_at: ""

workspace:
  projects: []

agent:
  engine: "claude"
  commands:
    claude: "claude"
    codex: "codex"
  timeout: 1800
  allow_user_plugins: false
  parallelism: 0
  retry:
    max_retries: 3
    initial_interval: 15
    max_interval: 120

learning:
  current:
    mode: "normal"
    scope: "flow"
    parallelism: 1
    select_relevant_files: true
    select_relevant_files_min_candidates: 200
    structural:
      enabled: true
      max_symbols: 30
      max_file_size: 512
  history:
    max_commits: 50
    batch_size: 5

autofix:
  strategy: "patch"
  backup_path: "backups"

skills:
  target: "claude"
  locale: "en-US"
  paths:
    claude: ".claude/skills/skills-seed-skills"
    codex: ".agents/skills/skills-seed-skills"

logging:
  level: "DEBUG"
  logs_path: "runtime/logs"
  max_log_files: 30

exclude:
  gitignore: true
  paths:
    - ".*"
    - "vendor/**"
    - "node_modules/**"
    - "dist/**"
    - "build/**"
    - "out/**"
    - "target/**"
    - "coverage/**"
    - ".cache/**"
    - "tmp/**"
    - "temp/**"
    - "*.log"
    - "*.tmp"
    - "*.bak"
    - "*.swp"
    - "*.zip"
    - "*.tar"
    - "*.tar.gz"
    - "*.tgz"
    - "*.rar"
    - "*.7z"
    - "*.png"
    - "*.jpg"
    - "*.jpeg"
    - "*.gif"
    - "*.webp"
    - "*.ico"
    - "*.pdf"
    - "*.mp4"
    - "*.mov"
```

## 配置项

### `profile`

`profile` 描述当前配置文件所属的项目或工作区本身，不表示 `project` 运行模式

#### 字段

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `name` | 当前目录名 | 项目名称，init 时自动填充 |
| `mode` | `project` | 初始化模式：`project` 单项目，`workspace` 多子项目工作区 |
| `language` | 自动识别或空 | 项目主要语言；init 识别不到时留空，可按项目设置 |
| `locale` | `zh-CN` | 工具输出与配置模板语言 |
| `git_remote` | 自动填充或空 | Git 远程仓库地址 |
| `root_path` | 当前项目绝对路径 | init 时写入，供运行时定位项目根目录 |
| `initialized_at` | init 时间 | 初始化时间 |

#### 说明

1. `mode` 在开始学习或生成 skills 后会被锁定，不能直接在 `project` 和 `workspace` 模式之间切换。
2. 需要重新选择模式时，使用 `skills-seed reset --mode project` 或 `skills-seed reset --mode workspace`。
3. `locale` 支持 `zh-CN` 和 `en-US`。

### `workspace`

#### 字段

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `projects` | `[]` | 子项目列表；workspace init 会尝试发现第一层目录中的项目 |

#### `projects` 项目字段

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `id` | 目录名规范化 | 子项目唯一标识 |
| `path` | 发现到的相对路径 | 子项目路径，相对 workspace 根目录 |
| `type` | 自动识别 | 子项目类型，例如应用、库、共享组件、基础设施或契约项目 |
| `language` | 自动识别 | 子项目主要语言 |

#### 行为

1. `skills-seed init --workspace` 会初始化根仓，并同步初始化当时检测到的子项目。
2. 后续新增或拷入 workspace 的子项目使用 `skills-seed workspace add .` 自动检测添加，或使用 `skills-seed workspace add <子项目>` 指定添加。
3. 子项目已有 `.skills-seed/config.yaml` 时不覆盖；如果子项目 agent 和根仓不同，只提示并保留子项目配置。
4. 子项目已有 `.skills-seed` 目录但缺少 `config.yaml` 时会报错，避免覆盖半初始化状态。
5. 只有 workspace 根目录第一层且拥有独立 `.git` 的目录会被识别为子项目。
6. `go.mod`、`package.json`、安装脚本、Helm/Terraform 等标记只用于识别 `type` 和 `language`，不再决定目录是否是项目。

### `learning.current`

`learning.current` 控制 `learn current` 的文件范围和结构化上下文。

#### 字段

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `mode` | `normal` | 分析深度：`fast` 更快并合并更多相近能力，`normal` 平衡质量和速度，`deep` 更深入并保留更多业务边界 |
| `scope` | `flow` | 单元切分范围：`domain` 按业务域合并，`flow` 按业务流程/资源动作拆分，`module` 允许按模块/插件/接口更细拆分 |
| `parallelism` | `1` | 项目内分析单元并发数；普通项目和 workspace 子项目生效，`1` 表示串行 |
| `select_relevant_files` | `true` | 是否先基于候选文件树筛选最值得分析的相关文件，减少无意义文件进入 AI 分析 |
| `select_relevant_files_min_candidates` | `200` | 候选文件数达到该阈值时才调用 AI 文件筛选；小项目直接使用本地过滤结果，避免额外 AI 调用 |
| `structural.enabled` | `true` | 是否启用结构化上下文；即使开启，也只会在存在 focus、diff、sample 或入口文件时运行 |
| `structural.max_symbols` | `30` | 输出到结构化上下文的最大符号数 |
| `structural.max_file_size` | `512` | 单个源码文件大小上限，单位 KB；超过时跳过该文件 |

#### `structural`

基于内嵌 tree-sitter 的轻量结构化预扫描。它提供符号、导入、入口点和模块线索，不依赖外部命令，也不维护索引。

0.7.1 起，结构化预扫描、`learn current` 和 `preview` 共用同一套文件选择策略：默认只纳入源码、构建配置和依赖配置；文档、生成产物、全局 `exclude` 命中的路径以及已生成 Skills 输出目录会被跳过。

0.9.0 起，项目结构摘要、样例文件收集和结构化预扫描都统一使用同一套配置化文件选择策略。除 `.git`、`.skills-seed` 和已配置的 skills 输出目录等内置安全边界外，不再在 analyzer 内额外维护目录名关键字；需要排除依赖、构建产物或项目自定义目录时，应写入 `exclude`。

0.9.1 起，`select_relevant_files` 默认开启；当本地过滤后的候选文件数达到 `select_relevant_files_min_candidates` 时，`learn current` 会先让 AI 从候选文件树和变更元数据中筛出更相关的文件，再进入后续分析。

0.9.11 起，文件选择策略默认还会叠加 Git ignore 规则；0.9.12 起，Git ignore 开关收敛到 `exclude.gitignore`。如需分析被 `.gitignore` 忽略的文件，可将 `exclude.gitignore` 设为 `false`。0.9.13 起，快照仍保存完整当前状态，但发送给 AI 的 diff 会按 `exclude.paths` 和 `exclude.gitignore` 过滤，避免被忽略文件作为删除 diff 进入分析。

#### 建议

1. 大多数项目保持默认值即可；没有边界输入时不会运行结构化上下文。
2. 明确不需要相关文件筛选时，把 `select_relevant_files` 设为 `false`。
3. 小项目可提高 `select_relevant_files_min_candidates`，直接跳过 AI 文件筛选；大型项目可降低该值以更早收敛范围。
4. 明确不需要结构化上下文时，把 `structural.enabled` 设为 `false`。
5. 大型仓库可降低 `structural.max_file_size`，避免解析生成文件、bundle 或异常大文件。
6. 结构化上下文只消费已有边界输入，不在没有 seed 时全仓扫描。

### Prompt 运行时调试

Prompt 片段仍从 `.skills-seed/prompts/` 读取，但 0.7.1 起渲染时会过滤默认元数据、空脚手架和未填写占位内容，只保留用户实际写入的约束。

渲染后的 prompt 默认保存在 `.skills-seed/runtime/rendered-prompts/`，并生成同名 `.manifest.json`。manifest 会记录内置模板、项目画像、项目补充、workspace 补充、用户指令和输出契约等片段是否参与合并、原始长度和最终长度，方便排查 Agent 实际收到的上下文。0.9.13 起，最终输出契约由独立的 append 模板追加，并对 JSON 型 prompt 强制要求最终响应只能是单个可解析 JSON 对象。

0.10.5 起，`learn current` 单元分析不会再把已有模式库写入每个单元 prompt；如果需要查看已有模式，请读取本地模式库或使用 `patterns show` / `patterns stats`。模型输出解析会在最终契约之外继续做程序化 JSON 修复，覆盖字符串内原始换行/控制字符、裸对象键和数组项缺失对象起始符等异常。0.10.7 起，修复范围继续扩展到尾随逗号、注释、单引号字符串、Python 风格字面量以及对象字段/数组元素漏逗号。

0.11.0 起，`learning.current.mode` 可设置为 `fast`、`normal` 或 `deep`，用于在学习速度和模式覆盖质量之间选择策略；该配置会进入续跑状态指纹。生成 skills 时会输出相关参考路由、重要性分层、验证矩阵和分组入口索引，并在渲染前校验证据路径是否存在。

0.11.1 起，`learning.current.scope` 可设置为 `domain`、`flow` 或 `module`，用于引导分析单元按业务域、业务流程或模块/插件粒度切分，并与 `mode` 一起进入续跑状态指纹。模型输出解析会额外修复证据行号范围表达式，将 `"line": 29-43` 这类非法 JSON 归一为单个行号。

初始化交互中的“Agent 总并发数”会自动落到具体配置：单项目模式写入 `learning.current.parallelism`；workspace 模式会根据发现到的子项目数拆分为根配置的 `agent.parallelism`（子项目并发）和 `learning.current.parallelism`（每个子项目内的分析单元并发），并确保两者乘积不超过总并发。

0.8.0 起，Agent 输出默认单独保存在 `.skills-seed/runtime/agent-outputs/`，包含最终内容、原始 CLI 输出、stderr 和 manifest。运行日志只记录长度和归档路径，不再输出模型回复预览或 stdout/stderr 明文。0.10.3 起，最终内容如果是合法 JSON，会在 `.md` 归档中格式化为可读的 `json` 代码块。

0.9.6 起，`.skills-seed/runtime` 下的调试记录使用 `YYYYMMDD-HHMMSS.NNNNNNNNN-<kind>-<name>` 文件名前缀。`rendered-prompts/` 与对应的 `agent-outputs/` 共享同一个日期时间 ID 和语义名，Agent 输出文件只额外包含 Agent 名称，方便把同一次调用中的 prompt 和输出一一对应。0.10.3 起，合法 JSON 输出会在 `.md` 归档中格式化为可读的 `json` 代码块。

0.9.0 起，模式库入库前会渲染 `pattern-curate` prompt，让 AI 对候选模式和相关历史模式做去重、整合、丢弃和输出前自检。0.10.4 起，默认入库策展使用本地确定性合并，并按 pattern ID 保持内部 accepted 集合唯一；候选模式复用已有 ID 或历史模式库已有重复 ID 时，会先收敛为单条更高质量的模式再写入。`generate skills` 不再执行模式合并，因此生成阶段的 prompt 只负责摘要和产物生成。

当前版本不再维护 skills dirty state。`sync` 完成学习后仅在本轮有学习变化时生成 skills；使用 `sync --pattern` 添加用户模式后会直接生成 skills。显式执行 `skills-seed generate skills` 也会删除旧的 skills-seed 生成目录并完整重建。

### 生成标记

Skills 模板中的 skills-seed 生成说明现在受内部默认值控制，默认不写入最终文件，减少生成内容对后续学习的干扰。需要确认产物来源时，可通过文件头部的 `generated-by` 元数据或运行时日志排查。

### `agent`

#### 字段

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `engine` | `claude` | 执行分析、学习和生成摘要的 Agent 引擎，对应 `commands` 的 key |
| `commands` | `claude: claude`、`codex: codex` | engine 到 CLI 命令的映射 |
| `timeout` | `1800` | 单次 AI 请求超时时间，单位秒 |
| `allow_user_plugins` | `false` | 是否允许 Agent 加载用户插件；默认关闭，避免批处理被用户插件影响 |
| `parallelism` | `0` | workspace 根配置中的子项目并发数；普通项目下不控制单元并发，`0` 表示自动 |
| `retry.max_retries` | `3` | 可重试错误的最大重试次数；配置为 `0` 时使用默认值 `3` |
| `retry.initial_interval` | `15` | 首次重试等待秒数；配置为 `0` 时使用默认值 `15` |
| `retry.max_interval` | `120` | 指数退避最大等待秒数；配置为 `0` 时使用默认值 `120` |

#### `parallelism` 说明

1. `project` 模式下，`agent.parallelism` 不控制项目内单元并发；请使用 `learning.current.parallelism`。
2. `workspace` 模式下，自动值为子项目数，上限 `6`。
3. 设置为大于 `0` 的数字时，使用该数字作为并发上限。
4. 实现上是真并发：子项目任务会通过 goroutine worker 池并行执行。

#### `retry` 说明

1. 当前会对 429 / 529 / overloaded 等可重试 Agent CLI 错误进行指数退避重试。
2. 等待时间从 `initial_interval` 开始，每次翻倍，并受 `max_interval` 限制。
3. `learn current` 等长耗时步骤会在进度行实时显示 Agent 错误、本次调用耗时和退避等待；等待结束并进入下一次调用时会切换为“第 N 次尝试”。

#### 切换 Agent

```yaml
agent:
  engine: "claude"
  commands:
    claude: "claude"
    codex: "codex"

skills:
  target: "codex"
  locale: "en-US"
  paths:
    claude: ".claude/skills/skills-seed-skills"
    codex: ".agents/skills/skills-seed-skills"
```

也可以在初始化时直接指定：

```bash
skills-seed init --mode project --agent codex
skills-seed init --workspace --agent codex
```

### 工作流资源

用户工作流不写入 `config.yaml`，也不属于 `profile.mode`。使用命令把用户显式传入的目标、约束、背景或路径交给当前 Agent 推导为标准工作流，推导后的正文保存到 `.skills-seed/workflows/<id>/WORKFLOW.md`，原始输入记录和元数据保存到同目录 `metadata.yaml`：

```bash
skills-seed workflow --context "发布前检查环境变量和构建产物，发布后执行 smoke test"
```

未提供 `--name` 时，Agent 会根据 `--context` 生成英文工作流标题，并用标题 slug 作为 `<id>`；标题重复时自动追加序号。`--context` 可以是目标、约束、背景、路径或零散说明，Agent 会从这些显式输入推导标准工作流。同名工作流默认会与已有内容合并去重；需要完全替换时使用 `--overwrite`。

生成 skills 时，工作流会写入输出目录的 `workflows/`，对应脚本目录会复制到 `scripts/workflows/<id>/`。

### `learning.history`

#### 字段

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `max_commits` | `50` | `learn history` 默认最多分析的 Git 提交数量 |
| `batch_size` | `5` | 批量学习历史提交时，单次 AI 调用包含的 commit 数量 |

#### 命令覆盖

```bash
skills-seed learn history --limit 100 --batch-size 10
```

命令参数只影响本次执行，不会改写配置文件。

### `.skills-seed` 目录结构

`.skills-seed/store/` 是持久化数据目录，不应删除；`.skills-seed/cache/` 是可重建缓存；`.skills-seed/runtime/` 只保存日志、渲染 prompt、Agent 输出和临时输入等运行时产物，可以在不需要排障时删除。

| 路径 | 作用 |
|---|---|
| `.skills-seed/store/project.db` | patterns、命中统计、文件指纹、评审等索引数据 |
| `.skills-seed/store/documents/` | 画像、规范、状态和变更记录等可读 JSON 文档 |
| `.skills-seed/cache/snapshots/` | 可重建的文件快照缓存 |
| `.skills-seed/cache/commands/<command>/state.json` | 未完成命令的可恢复状态，例如 `learn-current` 或 `sync`；可删除，删除后该命令会重新检测和规划 |
| `.skills-seed/runtime/logs/` | 运行日志 |
| `.skills-seed/runtime/rendered-prompts/` | 渲染后的 prompt 和 manifest |
| `.skills-seed/runtime/agent-outputs/` | Agent 输出归档 |

### `.skills-seed/prompts/`

`.skills-seed/prompts/` 不是 `config.yaml` 字段，但由 `skills-seed init` 创建，属于项目级可编辑运行时提示词片段。它用于长期生效的项目说明、workspace 约束和用户补充指令。

常见路径：

| 路径 | 作用 |
|---|---|
| `.skills-seed/prompts/project/project-profile.md` | 项目事实画像，会合并到相关 prompt |
| `.skills-seed/prompts/project/common.md` | 项目通用约束，会合并到相关 prompt |
| `.skills-seed/prompts/project/<prompt-id>.md` | 可选：某个 prompt 的项目级补充 |
| `.skills-seed/prompts/workspace/<prompt-id>.md` | workspace 级补充，例如 `skill-workspace-profile.md` |
| `.skills-seed/prompts/instructions/<prompt-id>.md` | 用户补充指令，追加到对应 prompt |

这些文件会与内置 prompt 合并，不会替换内置 prompt。合并后还会追加一个内置最终输出契约，保护 AI 返回的 JSON / Markdown 格式，避免用户补充指令破坏解析。

`--context` 和 `--context-file` 是学习阶段的一次性命令参数，只影响当前 `learn current` 运行，不会写入 `.skills-seed/prompts/`，也不会传给 `generate skills`。长期规则写入 `prompts/instructions/<prompt-id>.md`；临时说明使用 `learn current --context` 或 `learn current --context-file`。

### `autofix`

#### 字段

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `strategy` | `patch` | 自动修复策略：`patch`、`backup`、`stash`、`branch` |
| `backup_path` | `backups` | 备份路径，相对 `.skills-seed` 目录 |

#### 策略

1. `patch`：生成 patch 文件，默认推荐。
2. `backup`：备份原文件后修改。
3. `stash`：应用修复后通过 Git stash 保存。
4. `branch`：创建新分支应用修复。

### `skills`

#### 字段

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `target` | `agent.engine` | 生成的 Skills 目标类型；可与 `agent.engine` 不同 |
| `locale` | `en-US` | 生成的 Skills、AI prompt 以及会沉淀到 Skills 的自然语言内容 |
| `paths.claude` | `.claude/skills/skills-seed-skills` | Claude Code skills 输出目录 |
| `paths.codex` | `.agents/skills/skills-seed-skills` | Codex skills 输出目录 |

#### 说明

1. `generate skills` 默认使用 `skills.target` 对应的 `skills.paths`。
2. 可通过 `skills-seed generate skills --output <path>` 临时指定输出目录。
3. `skills.locale` 支持 `zh-CN` 和 `en-US`，默认英文；`profile.locale` 不再决定 AI prompt 或 Skills 内容语言。
4. 新增自定义 engine 或 target 时，应分别添加 `agent.commands.<engine>` 和 `skills.paths.<target>`。

### `logging`

#### 字段

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `level` | `DEBUG` | 日志级别：`DEBUG`、`INFO`、`WARN`、`ERROR` |
| `logs_path` | `runtime/logs` | 日志目录，相对 `.skills-seed` |
| `max_log_files` | `30` | 最多保留的日志文件数量，超过后自动清理旧日志 |

### `exclude`

`exclude` 控制学习、预览、项目结构摘要、样例文件收集和结构化预扫描共享的全局文件边界。

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `gitignore` | `true` | 是否排除 Git ignore 命中的文件，包括 `.gitignore`、`.git/info/exclude` 和全局 Git ignore |
| `paths` | 见下表 | 需要排除的相对路径或 glob |

关闭 `gitignore` 后，文件选择仍会应用内置安全边界、已生成 Skills 输出目录和 `exclude.paths`，但不会再跳过被 Git ignore 规则忽略的源码文件。

#### 默认值

| Pattern | 说明 |
|---|---|
| `.*` | 点号开头的文件和目录，如 `.github`、`.cursor`、`.env` |
| `vendor/**` | 常见依赖目录 |
| `node_modules/**` | 常见依赖目录 |
| `dist/**` | 常见构建产物目录 |
| `build/**` | 常见构建产物目录 |
| `out/**` | 常见输出目录 |
| `target/**` | 常见构建产物目录 |
| `coverage/**` | 覆盖率报告目录 |
| `.cache/**` | 缓存目录 |
| `tmp/**` | 临时目录 |
| `temp/**` | 临时目录 |
| `*.log` | 日志文件 |
| `*.tmp` | 临时文件 |
| `*.bak` | 备份文件 |
| `*.swp` | 编辑器交换文件 |
| `*.zip` / `*.tar` / `*.tar.gz` / `*.tgz` / `*.rar` / `*.7z` | 压缩包 |
| `*.png` / `*.jpg` / `*.jpeg` / `*.gif` / `*.webp` / `*.ico` | 图片资源 |
| `*.pdf` | 文档产物 |
| `*.mp4` / `*.mov` | 视频资源 |

#### 说明

1. `exclude.paths` 使用 glob 风格匹配，不是正则。不含 `/` 的模式（如 `*.log`）会同时对文件基名和完整路径匹配。
2. 排除规则会影响学习、预览、项目结构摘要、样例文件收集和结构化预扫描；默认还会叠加 `exclude.gitignore`。
3. 生成的 skills 目录默认也会排除，包括配置中的 `skills.paths`、`.claude/skills/**` 和 `.agents/skills/**`。
