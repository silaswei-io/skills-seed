# Skills Seed 配置说明

[简体中文](CONFIGURATION.md) | [English](CONFIGURATION.EN.md)

配置文件位于 `.skills-seed/config.yaml`。`skills-seed init` 会按当前项目生成默认配置；大多数路径都相对项目根目录或 `.skills-seed` 目录，具体以字段说明为准。

## 0.8.x 配置结构

0.8.x 继续沿用 0.7.x 配置结构，不保留旧字段兼容：

- 顶层 `project` 改名为 `profile`，表示当前配置文件所属项目或工作区本身，不表示 `project` 运行模式。
- `workspace` 下只保留 `projects`，不再提供 `shared`、`contracts`、`infra` 给用户手填。
- workspace 公共库、契约和基础设施影响会在 `learn current` 阶段根据仓库证据、子项目画像和一次性用户说明分析并沉淀到 workspace profile/spec，不从配置文件读取；生成阶段只消费已沉淀结果。
- workspace 根配置的 `profile.language` 默认留空，因为一个工作区可以包含多种语言子项目。
- `analysis.codegraph` 已移除，结构化上下文与符号校验统一配置在 `learning.current.structural`；默认 `provider: auto` 使用 CodeGraph，只有显式配置时才使用内嵌 tree-sitter。

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
  model: ""
  retry:
    max_retries: 3
    initial_interval: 15
    max_interval: 120

learning:
  current:
    mode: "normal"
    scope: "flow"
    max_focuses_per_call: 1
    structural:
      enabled: true
      provider: "auto"
      max_symbols: 30
      max_file_size: 512
skills:
  target: "claude"
  locale: "en-US"
  paths:
    claude: ".claude/skills/<project-name>-dev"
    codex: ".agents/skills/<project-name>-dev"

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
| `locale` | `zh-CN` | 工具输出、配置模板与 seed context 模板语言 |
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
| `mode` | `normal` | 学习策略：`normal` 平衡质量和速度；`fast` 保留紧凑但直接有证据的模式；`deep` 更愿意保留有证据的局部业务/代码模式 |
| `scope` | `flow` | 焦点规划取向：`flow` 默认且最稳定，优先关注业务流程/资源动作；`domain` 更偏长期业务职责；`module` 更偏模块/插件/契约边界。它是学习视角，不是固定分类或数量边界 |
| `max_focuses_per_call` | `1` | 单次 AI 调用最多分析的焦点数；`1` 表示不合批，降低单次输出过大、解析失败和跨焦点结论串扰风险 |
| `select_relevant_files` | `true` | 大候选集是否在议程规划前启用 AI 候选收敛；关闭后使用本地保守候选准备 |
| `select_relevant_files_min_candidates` | `200` | 候选文件数达到该阈值时才调用 AI 候选收敛；小范围变更直接保留本地候选 |
| `structural.enabled` | `true` | 是否启用结构化上下文；即使开启，也只会在存在 focus、diff、sample 或入口文件时运行 |
| `structural.provider` | `auto` | 结构化上下文与符号校验来源：`auto`/`codegraph` 使用 CodeGraph 并自动修复；`treesitter` 显式选择内嵌 parser |
| `structural.max_symbols` | `30` | 输出到结构化上下文的最大符号数 |
| `structural.max_file_size` | `512` | 单个源码文件大小上限，单位 KB；超过时跳过该文件 |

#### `structural`

结构化 provider 同时用于给 Agent 提供符号、导入、入口点和模块线索，以及校验 AI 返回的源码符号。默认 `provider: auto` 使用 CodeGraph；如果目标项目未初始化会自动执行初始化，索引同步或状态检查异常时会尝试自动修复。`provider: codegraph` 语义相同但表达显式选择；`provider: treesitter` 才会使用内嵌 parser。CodeGraph 上下文失败会记录警告并跳过该上下文，符号校验失败会直接返回错误；两者都不会在同一次运行中静默切换解析引擎。

0.7.1 起，结构化预扫描、`learn current` 和 `preview` 共用同一套文件过滤策略：默认只纳入源码、构建配置和依赖配置；文档、生成产物、全局 `exclude` 命中的路径以及已生成 Skills 输出目录会被跳过。

0.9.0 起，项目结构摘要、样例文件收集和结构化预扫描都统一使用同一套配置化文件过滤策略。除 `.git`、`.skills-seed` 和已配置的 skills 输出目录等内置安全边界外，不再在 analyzer 内额外维护目录名关键字；需要排除依赖、构建产物或项目自定义目录时，应写入 `exclude`。

当前版本中，`learn current` 在本地文件过滤后会执行候选准备：小范围变更直接保留候选，不再用路径词表删减源码文件；路径信号只用于挑选结构化上下文入口。候选数达到 `select_relevant_files_min_candidates` 且 `select_relevant_files` 开启时，会先调用 `learning-candidate-select` 让 AI 基于候选清单、必选路径和结构化上下文收敛大候选集；AI 不可用时回退为使用全部候选。候选路径、diff、焦点文件和结构化上下文等大块输入会写入 runtime 输入文件，prompt 通过路径引用它们。

0.9.11 起，文件过滤策略默认还会叠加 Git ignore 规则；0.9.12 起，Git ignore 开关收敛到 `exclude.gitignore`。如需分析被 `.gitignore` 忽略的文件，可将 `exclude.gitignore` 设为 `false`。0.9.13 起，快照仍保存完整当前状态，但发送给 AI 的 diff 会按 `exclude.paths` 和 `exclude.gitignore` 过滤，避免被忽略文件作为删除 diff 进入分析。

#### 建议

1. 大多数项目保持默认值即可；没有边界输入时不会运行结构化上下文。
2. 大型仓库候选特别多但希望减少规划成本时，保持 `select_relevant_files: true` 并按项目规模调整 `select_relevant_files_min_candidates`。
3. 明确不需要结构化上下文时，把 `structural.enabled` 设为 `false`。
4. 大型仓库显式使用 tree-sitter 时可降低 `structural.max_file_size`，避免解析生成文件、bundle 或异常大文件。
5. 结构化上下文只消费已有边界输入，不在没有 seed 时全仓扫描。

### Prompt 运行时调试

项目上下文从 `.skills-seed/context/` 读取，渲染时会过滤默认元数据、空脚手架和未填写占位内容，只保留用户实际写入的上下文。

渲染后的 prompt 默认保存在 `.skills-seed/runtime/rendered-prompts/`，并生成同名 `.manifest.json`。manifest 会记录内置模板、context 片段和输出契约等片段是否参与合并、原始长度和最终长度，方便排查 Agent 实际收到的上下文。候选文件、焦点文件和结构化上下文等大块输入会优先保存到 `.skills-seed/runtime/` 下的 prompt input 目录，渲染后的 prompt 通过路径引用它们。最终输出契约由独立的 append 模板追加，并对 JSON 型 prompt 强制要求最终响应只能是单个可解析 JSON 对象，同时要求相同输入下保持语义稳定和确定性排序。

0.10.5 起，`learn current` 焦点分析不会再把已有模式库写入每个焦点 prompt；如果需要查看已有模式，请读取本地模式库或使用 `patterns show` / `patterns stats`。Claude 和 Codex 调用统一使用 DTO 生成的 Schema 约束原生结构化输出；解析层使用 `jsonrepair-go` 修复 JSON 语法，修复结果仍必须通过严格 DTO 解码，不会放行未知字段或错误结构。

0.11.0 起，`learning.current.mode` 可设置为 `fast`、`normal` 或 `deep`，用于在学习速度和模式覆盖质量之间选择策略；该配置会进入续跑状态指纹。生成 skills 时会输出相关参考路由、重要性分层和分组入口索引，并在渲染前校验证据路径是否存在。验证命令集中写入 `references/validation.md`；Go 项目根据真实 `go.mod` 和 `_test.go` 额外生成 `references/testing.md`，测试文件归属到最近的上级 module。

0.11.1 起，`learning.current.scope` 可设置为 `domain`、`flow` 或 `module`，用于引导学习焦点按业务域、业务流程或模块/插件粒度切分，并与 `mode` 一起进入续跑状态指纹。模型输出解析会额外修复证据行号范围表达式，将 `"line": 29-43` 这类非法 JSON 归一为单个行号。

0.11.2 起，`learning.current.max_focuses_per_call` 可控制一次 AI 调用最多分析的焦点数，默认 `1` 表示不合批；调高后会把多个焦点放入同一次调用并要求响应按顶层 `focuses` 返回。生成 skills 时，低频或局部证据不会进入强约束层，避免把偶发现象误写成必须遵守的项目标准。

候选准备决定进入议程规划的文件范围；AI 候选收敛、学习焦点规划和当前代码学习 prompt 都使用明确的稳定决策规则。当证据等价时，优先结构证据、可路由性和源码词汇，最后用路径、ID 或符号的字典序作为 tie-breaker。

初始化交互中的“Agent 总并发数”会写入 `agent.parallelism`。workspace 根配置用它控制子项目并发；普通 project 配置用它控制当前代码学习的证据焦点批次并发。`learn current` 仍使用分段短会话：候选准备后由 planning 会话生成证据包，每个证据焦点批次分析使用独立会话，模式入库前按候选归属执行分片 pattern curation，并由本地确定性逻辑完成跨分片合并。

0.8.0 起，Agent 输出默认单独保存在 `.skills-seed/runtime/agent-outputs/`，包含最终内容、原始 CLI 输出、stderr 和 manifest。运行日志只记录长度和归档路径，不再输出模型回复预览或 stdout/stderr 明文。0.10.3 起，最终内容如果是合法 JSON，会在 `.md` 归档中格式化为可读的 `json` 代码块。

0.9.6 起，`.skills-seed/runtime` 下的调试记录使用 `YYYYMMDD-HHMMSS[-NNN]-<kind>-<name>` 文件名前缀；同一秒内生成多个 runtime ID 时追加递增序号避免覆盖。`rendered-prompts/` 与对应的 `agent-outputs/` 共享同一个日期时间 ID 和语义名，Agent 输出文件只额外包含 Agent 名称，方便把同一次调用中的 prompt 和输出一一对应。0.10.3 起，合法 JSON 输出会在 `.md` 归档中格式化为可读的 `json` 代码块。

0.9.0 起，模式库入库前会执行候选策展。当前 `learn current` 使用分段会话式 `learning-pattern-curate` AI 契约，只让 Agent 决定规范文本、置信度和真实 `source_ids`；示例、源码证据、能力入口、频次、来源和统计由本地代码从输入恢复。未知来源会被清理，未分类候选只执行本地确定性恢复，不再完整重跑 AI；同一候选同时被保留和丢弃时，以可追溯的规范模式为准。`generate skills` 只读取已保存数据，不执行模式合并或 Agent 调用。

当前版本不再维护 skills dirty state。`sync` 完成学习后仅在本轮有学习变化时生成 skills。显式执行 `skills-seed generate skills` 会删除旧的 skills-seed 生成目录并完整重建；手动添加用户模式后应显式运行该命令刷新产物。

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
| `parallelism` | `0` | Agent 并发数；workspace 根配置控制子项目并发，普通 project 配置控制证据焦点批次并发，`0` 表示自动 |
| `model` | 空 | skills-seed 调用 Agent CLI 时使用的模型名；空值不传模型参数，继承本机 Agent CLI 默认配置 |
| `retry.max_retries` | `3` | 可重试错误的最大重试次数；配置为 `0` 时使用默认值 `3` |
| `retry.initial_interval` | `15` | 首次重试等待秒数；配置为 `0` 时使用默认值 `15` |
| `retry.max_interval` | `120` | 指数退避最大等待秒数；配置为 `0` 时使用默认值 `120` |

#### `parallelism` 说明

1. `project` 模式下，`agent.parallelism > 1` 时，`learn current` 会并发分析独立证据焦点批次；每个批次使用独立短会话，结果回到主流程后按议程顺序合并和 checkpoint。
2. `workspace` 模式下，自动值为子项目数，上限 `6`。
3. 设置为大于 `0` 的数字时，使用该数字作为并发上限。
4. workspace 子项目任务会通过 goroutine worker 池并行执行；每个子项目内部是否并发分析证据焦点，由该子项目配置的 `agent.parallelism` 决定。

#### `retry` 说明

1. 当前会对 429 / 529 / overloaded 等可重试 Agent CLI 错误进行指数退避重试。
2. 等待时间从 `initial_interval` 开始，每次翻倍，并受 `max_interval` 限制。
3. `learn current` 等长耗时步骤会在进度行实时显示 Agent 错误、本次调用耗时和退避等待；同时终端会输出一条稳定的重试提示，包含等待时间和提取到的 API 原因。
4. 等待结束并进入下一次调用时，进度行会切换为“第 N 次尝试”；只有重试耗尽或不可重试错误才会显示最终 CLI 调用失败。

#### 切换 Agent

```yaml
agent:
  engine: "claude"
  model: ""
  commands:
    claude: "claude"
    codex: "codex"

skills:
  target: "codex"
  locale: "en-US"
  paths:
    claude: ".claude/skills/<project-name>-dev"
    codex: ".agents/skills/<project-name>-dev"
```

也可以在初始化时直接指定：

```bash
skills-seed init --mode project --agent codex
skills-seed init --mode project --agent codex --agent-model gpt-5-mini
skills-seed init --workspace --agent codex
```

### 工作流资源

用户工作流不写入 `config.yaml`，也不属于 `profile.mode`。使用命令把用户显式传入的目标、约束、背景或路径交给当前 Agent 推导为标准工作流，推导后的正文保存到 `.skills-seed/workflows/<id>/WORKFLOW.md`，原始输入记录和元数据保存到同目录 `metadata.yaml`：

```bash
skills-seed workflow --context "发布前检查环境变量和构建产物，发布后执行 smoke test"
```

未提供 `--name` 时，Agent 会根据 `--context` 生成英文工作流标题，并用标题 slug 作为 `<id>`；标题重复时自动追加序号。`--context` 可以是目标、约束、背景、路径或零散说明，Agent 会从这些显式输入推导标准工作流。同名工作流默认会与已有内容合并去重；需要完全替换时使用 `--overwrite`。

生成 skills 时，工作流会写入输出目录的 `workflows/`，对应脚本目录会复制到 `scripts/workflows/<id>/`。

### `.skills-seed` 目录结构

`.skills-seed/store/` 是持久化数据目录，不应删除；`.skills-seed/cache/` 是可重建缓存；`.skills-seed/runtime/` 只保存日志、渲染 prompt、Agent 输出和临时输入等运行时产物，可以在不需要排障时删除。

| 路径 | 作用 |
|---|---|
| `.skills-seed/store/project.db` | patterns、质量指标、文件指纹等索引数据 |
| `.skills-seed/store/documents/` | 画像、规范、状态和变更记录等可读 JSON 文档 |
| `.skills-seed/cache/snapshots/` | 可重建的文件快照缓存 |
| `.skills-seed/cache/commands/<command>/state.json` | 未完成命令的可恢复状态，包括尚未入库的焦点分析结果；全部持久化后自动清除，可删除并重新检测和规划 |
| `.skills-seed/runtime/logs/` | 运行日志 |
| `.skills-seed/runtime/rendered-prompts/` | 渲染后的 prompt 和 manifest |
| `.skills-seed/runtime/agent-outputs/` | Agent 输出归档 |

### `.skills-seed/context/`

`.skills-seed/context/` 不是 `config.yaml` 字段，但由 `skills-seed init` 创建，属于项目级可编辑上下文目录。它用于长期生效的项目背景、团队约束、术语和 workspace 约束。

常见路径：

| 路径 | 作用 |
|---|---|
| `.skills-seed/context/background.md` | 代码看不到的业务背景、外部系统和线上事实 |
| `.skills-seed/context/constraints.md` | 长期团队规则、兼容性、安全边界和禁止事项 |
| `.skills-seed/context/terminology.md` | 术语、别名、状态名和业务词到代码词的对应关系 |
| `.skills-seed/context/workspace.md` | workspace 级上下文，仅 workspace 模式生成 |

这些文件会与内置 prompt 合并，不会替换内置 prompt。合并后还会追加一个内置最终输出契约，保护 AI 返回的 JSON / Markdown 格式，避免用户上下文破坏解析。

`--context` 和 `--context-path` 是学习阶段的一次性命令参数，只影响当前 `learn current` 运行，不会写入 `.skills-seed/context/`，也不会传给 `generate skills`。长期规则写入 `context/constraints.md`；临时说明使用 `learn current --context` 或 `learn current --context-path`。

### `skills`

#### 字段

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `target` | `agent.engine` | 生成的 Skills 目标类型；可与 `agent.engine` 不同 |
| `locale` | `en-US` | AI 学习输出、沉淀内容和生成 Skills 使用的语言 |
| `paths.claude` | `.claude/skills/<project-name>-dev` | Claude Code skills 输出目录；workspace 根默认为 `<workspace-name>-workspace-dev` |
| `paths.codex` | `.agents/skills/<project-name>-dev` | Codex skills 输出目录；workspace 根默认为 `<workspace-name>-workspace-dev` |

#### 说明

1. `generate skills` 默认使用 `skills.target` 对应的 `skills.paths`。
2. 可通过 `skills-seed generate skills --output <path>` 临时指定输出目录。
3. `skills.locale` 支持 `zh-CN` 和 `en-US`，默认英文；它统一控制运行时 AI 自然语言输出、沉淀内容以及 `generate skills` 产物语言。
4. 新增自定义 engine 或 target 时，应分别添加 `agent.commands.<engine>` 和 `skills.paths.<target>`。

运行时 AI prompt 模板统一维护为英文单源模板，最终输出契约跟随 `skills.locale`；`profile.locale` 只影响工具输出、配置模板和 seed context 模板语言。

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

关闭 `gitignore` 后，文件过滤仍会应用内置安全边界、已生成 Skills 输出目录和 `exclude.paths`，但不会再跳过被 Git ignore 规则忽略的源码文件。

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
