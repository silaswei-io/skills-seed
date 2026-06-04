# Skills Seed 配置说明

[简体中文](CONFIGURATION.md) | [English](CONFIGURATION.EN.md)

配置文件位于 `.skills-seed/config.yaml`。`skills-seed init` 会按当前项目生成默认配置；大多数路径都相对项目根目录或 `.skills-seed` 目录，具体以字段说明为准。

## 0.6.1 配置结构

0.6.1 延续 0.6.0 的干净配置结构，不保留旧字段兼容：

- 顶层 `project` 改名为 `profile`，表示当前配置文件所属项目或工作区本身，不表示 `project` 运行模式。
- `workspace` 下只保留 `projects`，不再提供 `shared`、`contracts`、`infra` 给用户手填。
- workspace 公共库、契约和基础设施影响会在 `learn current` 阶段根据仓库证据、子项目画像和一次性用户说明分析并沉淀到 workspace profile/spec，不从配置文件读取；生成阶段只消费已沉淀结果。
- workspace 根配置的 `profile.language` 默认留空，因为一个工作区可以包含多种语言子项目。

## 配置示例

### 默认结构

```yaml
profile:
  name: "your-project"
  mode: "project"
  language: "go"
  locale: "zh-CN"
  git_remote: ""
  root_path: ""
  initialized_at: ""

workspace:
  projects: []

analysis:
  codegraph:
    enabled: true
    required: false
    command: "codegraph"
    auto_init: true
    auto_sync: true
    max_nodes: 30
    max_code: 0

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
  max_commits: 50
  batch_size: 5

autofix:
  strategy: "patch"
  backup_path: "backups"

skills:
  target: "claude"
  paths:
    claude: ".claude/skills/skills-seed-skills"
    codex: ".agents/skills/skills-seed-skills"

logging:
  level: "DEBUG"
  logs_path: "logs"
  max_log_files: 30

exclude:
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
| `language` | `go` | 项目主要语言，可按项目改为 `typescript`、`python` 等 |
| `locale` | `zh-CN` | CLI 输出、配置模板、prompt 和 skills 模板语言 |
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
| `type` | 自动识别 | 子项目类型，例如 `backend`、`frontend`、`library`、`infra`、`contracts` |
| `language` | 自动识别 | 子项目主要语言 |

#### 行为

1. `skills-seed init --workspace` 会初始化根仓，并同步初始化当时检测到的子项目。
2. 后续新增或拷入 workspace 的子项目使用 `skills-seed workspace add .` 自动检测添加，或使用 `skills-seed workspace add <子项目>` 指定添加。
3. 子项目已有 `.skills-seed/config.yaml` 时不覆盖；如果子项目 agent 和根仓不同，只提示并保留子项目配置。
4. 子项目已有 `.skills-seed` 目录但缺少 `config.yaml` 时会报错，避免覆盖半初始化状态。
5. 只有 workspace 根目录第一层且拥有独立 `.git` 的目录会被识别为子项目。
6. `go.mod`、`package.json`、安装脚本、Helm/Terraform 等标记只用于识别 `type` 和 `language`，不再决定目录是否是项目。

### `analysis.codegraph`

#### 字段

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `enabled` | `true` | 是否启用 CodeGraph 结构化分析增强 |
| `required` | `false` | CodeGraph 不可用时是否直接失败；`false` 表示提醒后降级为普通文件分析 |
| `command` | `codegraph` | CodeGraph 命令路径 |
| `auto_init` | `true` | 目标项目没有 `.codegraph` 时是否自动执行 `codegraph init -i` |
| `auto_sync` | `true` | 目标项目已有索引时，分析前是否执行 `codegraph sync` |
| `max_nodes` | `30` | 传给 `codegraph context` 的最大符号节点数 |
| `max_code` | `0` | 传给 `codegraph context` 的最大代码块数；`0` 表示只提供结构摘要 |

#### 建议

1. 本地开发可保持默认值。
2. CI 或团队强约束环境中，如果必须使用 CodeGraph，可设置 `required: true`。
3. 如果不想自动创建或同步索引，可把 `auto_init` 或 `auto_sync` 改为 `false`。

### `agent`

#### 字段

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `engine` | `claude` | 执行分析、学习和生成摘要的 Agent 引擎，对应 `commands` 的 key |
| `commands` | `claude: claude`、`codex: codex` | engine 到 CLI 命令的映射 |
| `timeout` | `1800` | 单次 AI 请求超时时间，单位秒 |
| `allow_user_plugins` | `false` | 是否允许 Agent 加载用户插件；默认关闭，避免批处理被用户插件影响 |
| `parallelism` | `0` | 并发 Agent 数；`0` 表示自动 |
| `retry.max_retries` | `3` | 可重试错误的最大重试次数；配置为 `0` 时使用默认值 `3` |
| `retry.initial_interval` | `15` | 首次重试等待秒数；配置为 `0` 时使用默认值 `15` |
| `retry.max_interval` | `120` | 指数退避最大等待秒数；配置为 `0` 时使用默认值 `120` |

#### `parallelism` 说明

1. `project` 模式下，自动值为 `1`。
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
  paths:
    claude: ".claude/skills/skills-seed-skills"
    codex: ".agents/skills/skills-seed-skills"
```

也可以在初始化时直接指定：

```bash
skills-seed init --mode project --agent codex
skills-seed init --workspace --agent codex
```

### `learning`

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
| `paths.claude` | `.claude/skills/skills-seed-skills` | Claude Code skills 输出目录 |
| `paths.codex` | `.agents/skills/skills-seed-skills` | Codex skills 输出目录 |

#### 说明

1. `generate skills` 默认使用 `skills.target` 对应的 `skills.paths`。
2. 可通过 `skills-seed generate skills --output <path>` 临时指定输出目录。
3. 新增自定义 engine 或 target 时，应分别添加 `agent.commands.<engine>` 和 `skills.paths.<target>`。

### `logging`

#### 字段

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `level` | `DEBUG` | 日志级别：`DEBUG`、`INFO`、`WARN`、`ERROR` |
| `logs_path` | `logs` | 日志目录，相对 `.skills-seed` |
| `max_log_files` | `30` | 最多保留的日志文件数量，超过后自动清理旧日志 |

### `exclude`

#### 默认值

| Pattern | 说明 |
|---|---|
| `.*` | 点号开头的文件和目录，如 `.github`、`.cursor`、`.codegraph`、`.env` |
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

1. `exclude` 使用 glob 风格匹配，不是正则。不含 `/` 的模式（如 `*.log`）会同时对文件基名和完整路径匹配。
2. 排除规则会影响学习和分析。
3. 生成的 skills 目录默认也会排除，包括配置中的 `skills.paths`、`.claude/skills/**` 和 `.agents/skills/**`。
