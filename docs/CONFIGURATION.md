# Skills Seed 配置说明

[简体中文](CONFIGURATION.md) | [English](CONFIGURATION.EN.md)

配置文件位于 `.skills-seed/config.yaml`。`skills-seed init` 会按当前项目生成默认配置；大多数路径都相对项目根目录或 `.skills-seed` 目录，具体以字段说明为准。

## 配置示例

### 默认结构

```yaml
project:
  name: "your-project"
  mode: "project"
  language: "go"
  locale: "zh-CN"
  git_remote: ""
  root_path: ""
  initialized_at: ""

workspace:
  init_children: false
  projects: []
  shared: []
  contracts: []
  infra: []

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
  provider: "claude"
  commands:
    claude: "claude"
    codex: "codex"
  timeout: 1800
  allow_user_plugins: false
  parallelism: 0

learning:
  max_commits: 50
  batch_size: 5

autofix:
  strategy: "patch"
  backup_path: "backups"

output:
  skills_paths:
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

### `project`

#### 字段

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `name` | 当前目录名 | 项目名称，init 时自动填充 |
| `mode` | `project` | 初始化模式：`project` 单项目，`workspace` 多子项目工作区 |
| `language` | `go` | 项目主要语言，可按项目改为 `typescript`、`python` 等 |
| `locale` | 自动检测；未识别时 `zh-CN` | CLI 输出、配置模板、prompt 和 skills 模板语言 |
| `git_remote` | 自动填充或空 | Git 远程仓库地址 |
| `root_path` | 当前项目绝对路径 | init 时写入，供运行时定位项目根目录 |
| `initialized_at` | init 时间 | 初始化时间 |

#### 说明

1. `mode` 在开始学习或生成 skills 后会被锁定，不能直接在 `project` 和 `workspace` 之间切换。
2. 需要重新选择模式时，使用 `skills-seed reset --mode project` 或 `skills-seed reset --mode workspace`。
3. `locale` 支持 `zh-CN` 和 `en-US`。

### `workspace`

#### 字段

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `init_children` | `false` | `learn current` 遇到缺失 `.skills-seed` 的子项目时，是否先自动初始化 |
| `projects` | `[]` | 子项目列表；workspace init 会尝试发现第一层目录中的项目 |
| `shared` | `[]` | 公共库或共享代码目录 |
| `contracts` | `[]` | API、IDL、协议等契约目录 |
| `infra` | `[]` | 部署、运维、基础设施目录 |

#### `projects` 项目字段

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `id` | 目录名规范化 | 子项目唯一标识 |
| `path` | 发现到的相对路径 | 子项目路径，相对 workspace 根目录 |
| `type` | 自动识别 | 子项目角色，例如 `backend`、`frontend`、`infra`、`contracts` |
| `language` | 自动识别 | 子项目主要语言 |

#### `shared` / `contracts` / `infra` 路径字段

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `path` | 无 | 路径，相对 workspace 根目录 |
| `description` | 空 | 路径用途说明 |

#### 行为

1. `skills-seed init --workspace` 只初始化根仓。
2. `skills-seed init --workspace --children` 会初始化根仓，并同步初始化 `workspace.projects` 中缺失 `.skills-seed` 的子项目。
3. `workspace.init_children: true` 后，`skills-seed learn current` 会在学习前补初始化缺失 `.skills-seed` 的子项目。
4. 子项目已有 `.skills-seed/config.yaml` 时不覆盖；如果子项目 agent 和根仓不同，只提示并保留子项目配置。

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
| `provider` | `claude` | 当前 Agent provider，对应 `commands` 和 `output.skills_paths` 的 key |
| `commands` | `claude: claude`、`codex: codex` | provider 到 CLI 命令的映射 |
| `timeout` | `1800` | 单次 AI 请求超时时间，单位秒 |
| `allow_user_plugins` | `false` | 是否允许 Agent 加载用户插件；默认关闭，避免批处理被用户插件影响 |
| `parallelism` | `0` | 并发 Agent 数；`0` 表示自动 |

#### `parallelism` 说明

1. `project` 模式下，自动值为 `1`。
2. `workspace` 模式下，自动值为子项目数，上限 `6`。
3. 设置为大于 `0` 的数字时，使用该数字作为并发上限。
4. 实现上是真并发：子项目任务会通过 goroutine worker 池并行执行。

#### 切换 Agent

```yaml
agent:
  provider: "codex"
  commands:
    claude: "claude"
    codex: "codex"

output:
  skills_paths:
    claude: ".claude/skills/skills-seed-skills"
    codex: ".agents/skills/skills-seed-skills"
```

也可以在初始化时直接指定：

```bash
skills-seed init --mode project --agent codex
skills-seed init --workspace --children --agent codex
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

### `output`

#### 字段

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `skills_paths.claude` | `.claude/skills/skills-seed-skills` | Claude Code skills 输出目录 |
| `skills_paths.codex` | `.agents/skills/skills-seed-skills` | Codex skills 输出目录 |

#### 说明

1. `generate-skills` 默认使用当前 `agent.provider` 对应的 `output.skills_paths`。
2. 可通过 `skills-seed generate-skills --output <path>` 临时指定输出目录。
3. 新增自定义 provider 时，应同时添加 `agent.commands.<provider>` 和 `output.skills_paths.<provider>`。

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
3. 生成的 skills 目录默认也会排除，包括配置中的 `output.skills_paths`、`.claude/skills/**` 和 `.agents/skills/**`。
