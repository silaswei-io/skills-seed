# Skills Seed

**从代码库学习项目规范，并生成 Claude Code / Codex 可用的本地 skills。**

[简体中文](README.md) | [English](README.en.md)

Skills Seed 会分析当前代码、Git 历史和项目结构，把团队已有的写法沉淀为本地知识资产，再按当前 `agent.provider` 渲染到 `.claude/skills` 或 `.agents/skills`。所有数据默认保存在当前仓库的 `.skills-seed` 目录中。

## 功能

- 从当前代码库学习 patterns、业务方法、工具方法和最佳实践
- 从 Git 历史增量学习，并跳过已分析的 commit
- 生成项目画像 `project-profile.json` 和项目规范 `project-spec.json`
- 生成 Claude Code / Codex skills，包括 `SKILL.md` 与 `references/`
- 支持单项目模式和多子项目 workspace 模式
- workspace 根 skill 只负责路由和跨项目关系，子仓 skill 由各子仓独立学习和生成
- 支持 `check`、交互式修复、pre-commit hook
- 支持中文和英文模板、提示词、配置与终端输出

## 安装

```bash
go install github.com/silaswei-io/skills-seed/cmd/skills-seed@latest
skills-seed --help
```

如果命令不可用，请把 `$GOPATH/bin` 或 `$GOBIN` 加入 `PATH`。

源码构建：

```bash
git clone https://github.com/silaswei-io/skills-seed.git
cd skills-seed
go build -o skills-seed ./cmd/skills-seed
```

## 前置要求

- Go 1.25.6+
- Git 仓库
- 可用的 AI Agent CLI：默认是 `claude`，也可在配置中切换到 `codex`

## 快速开始：单项目

```bash
cd your-project
skills-seed init --mode project --locale zh-CN
skills-seed learn current
skills-seed generate-skills
```

默认 provider 为 `claude`，输出：

```text
your-project/
├── .skills-seed/
│   ├── config.yaml
│   ├── memory/
│   │   ├── project.db
│   │   ├── project-profile.json
│   │   └── project-spec.json
│   └── logs/
└── .claude/skills/skills-seed-skills/
```

把 `.skills-seed/config.yaml` 中的 `agent.provider` 改为 `codex` 后，会输出到 `.agents/skills/...`。

## 快速开始：Workspace

适合一个 Git 仓库下包含多个子项目，例如 `frontend/`、`backend/`、`gateway/`、`deploy/`。

```bash
cd your-workspace
skills-seed init --mode workspace --locale zh-CN
# 或：skills-seed init --workspace
```

初始化只扫描 workspace 根目录的第一层文件夹，并按常见项目标记识别子项目，如 `package.json`、`go.mod`、`pyproject.toml`、`Cargo.toml`、`pom.xml`、`build.gradle`、`composer.json`、`Gemfile`、`Chart.yaml`、`Dockerfile`、`openapi.yaml`。检查并按需修改 `.skills-seed/config.yaml`：

```yaml
project:
  mode: "workspace"

workspace:
  projects:
    - {id: "frontend", path: "frontend", type: "frontend", language: "typescript"}
    - {id: "backend", path: "backend", type: "backend", language: "go"}
  shared:
    - {path: "pkg"}
  contracts:
    - {path: "proto"}
  infra:
    - {path: "deploy"}

agent:
  parallelism: 0   # 0 表示自动：project=1，workspace=子项目数并带上限
```

然后执行：

```bash
cd frontend && skills-seed init --mode project --locale zh-CN && cd ..
cd backend && skills-seed init --mode project --locale zh-CN && cd ..
skills-seed learn current
skills-seed generate-skills
```

workspace 根仓要求每个子项目都是独立 Git 仓库，并且已经在子项目目录执行过 `skills-seed init --mode project`。`learn current` 会进入子项目自己的 `.skills-seed` 做增量学习；`generate-skills` 会先用子仓自己的配置生成子仓 skill，最后再生成根仓 workspace skill。两个命令都会按 `agent.parallelism` 并发处理子项目。

workspace 模式会生成：

- 当前 provider 的根目录 skill：负责 workspace 路由、跨项目规则和影响范围
- `.skills-seed/memory/workspace-profile.json`
- `.skills-seed/memory/workspace-spec.json`

子项目 skill、子项目画像、patterns 和文件 md5 指纹都保存在子项目自己的 `.skills-seed` 中。根 skill 会读取子项目配置中的 `agent.provider`、`output.skills_paths` 和已生成 skill 摘要来指向子项目 skill。已有手写 `SKILL.md` 没有 `generated-by: skills-seed` 标记时默认不会被覆盖。

## 日常命令

### 学习

```bash
# 从当前代码学习，并按需要生成或刷新项目画像
skills-seed learn current

# 只学习局部目录，不刷新项目画像
skills-seed learn current --focus internal/service --profile skip

# 局部学习，并基于已有画像做增量画像刷新
skills-seed learn current --focus internal/service --profile refresh

# 从 Git 历史学习，已学习 commit 会跳过
skills-seed learn history --limit=50
skills-seed learn history --since=30d
```

`--profile` 可选值：

- `auto`：默认值。首次或全量学习会刷新画像；窄范围改动会尽量跳过
- `skip`：只学习 patterns，不更新画像
- `refresh`：基于当前输入刷新画像

`learn current` 首次成功后会记录已分析文件的 md5。后续执行会先比较文件指纹：没有可学习文件变化时会同时跳过 patterns 学习和项目画像刷新；有变化时只围绕新增、修改或删除的文件做增量学习。workspace 模式按子项目隔离记录，一个子项目的变更不会触发其他子项目重新学习。

生成的 skills 目录默认不会参与学习，包括配置中的 `output.skills_paths`，以及 `.claude/skills/**`、`.agents/skills/**`。这可以避免 `SKILL.md` 和 `references/` 被下一轮学习当作普通项目文件。

`learn current` 会在学习日志结束后输出 Token 消耗。workspace 模式会在每个子项目学习日志末尾输出该子项目的 Token 消耗；并发处理多个子项目时，终端只显示子项目开始、摘要、Token 和完成信息，不输出每个子仓的 5 步进度和后续命令提示，避免日志交错。

### 画像与规范

```bash
skills-seed profile show
skills-seed profile refresh
```

`profile refresh` 只重建项目画像，不学习 patterns。`project-spec.json` 会在 `generate-skills` 时由画像和 patterns 生成。

### 生成 Skills

```bash
skills-seed generate-skills

# 需要先合并相似 patterns 时显式执行
skills-seed patterns merge
skills-seed generate-skills

# 临时指定输出路径
skills-seed generate-skills --output .agents/skills/my-project
```

生成阶段固定调用当前 Agent 做摘要合并和润色，再把结果写入模板；因此执行 `generate-skills` 前需要确保 `agent.provider` 对应的 CLI 可用。

生成内容包括：

```text
SKILL.md
agents/
references/
  project-overview.md
  project-spec.md
  patterns/*.md
  examples/*.md
```

### 检查与 Hook

```bash
# 默认检查暂存区
skills-seed check

# 检查所有 Git 跟踪文件
skills-seed check --all

# 安装 pre-commit hook
skills-seed hook install
```

## 初始化模式和锁定

初始化时必须选择一种模式：

```bash
skills-seed init --mode project
skills-seed init --mode workspace
```

开始学习或生成 skills 后，`project.mode` 会被锁定，不能直接在 `project` 和 `workspace` 之间切换。需要重新初始化时使用：

```bash
skills-seed reset --mode workspace
```

`reset` 会把旧 `.skills-seed` 备份到 `.skills-seed.backup/<timestamp>`。

## 配置

配置文件位于 `.skills-seed/config.yaml`。常用字段：

```yaml
project:
  name: "your-project"
  mode: "project"      # project 或 workspace
  language: "go"
  locale: "zh-CN"

analysis:
  codegraph:
    enabled: true       # 默认启用结构化分析增强；未安装 codegraph 时提醒并降级
    required: false     # true 表示 CodeGraph 不可用时直接失败
    command: "codegraph"
    auto_init: true     # 目标项目没有 .codegraph 时是否自动执行 codegraph init -i
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

output:
  skills_paths:
    claude: ".claude/skills/skills-seed-skills"
    codex: ".agents/skills/skills-seed-skills"

logging:
  level: "DEBUG"
  logs_path: "logs"
  max_log_files: 30
```

`analysis.codegraph.enabled` 默认为 `true`。如果机器未安装 `codegraph`，或目标项目还没有 `.codegraph/` 索引，`required: false` 会让 `skills-seed` 打印提醒并继续使用普通文件分析。需要强制使用 CodeGraph 的团队环境可把 `required` 改为 `true`。

## 文档

- [项目架构](docs/project-architecture.md)
- [生成链路说明](docs/project-generation-guide.md)
- [Changelog](CHANGELOG.md)

## 开发

```bash
go test ./...
go vet ./...
go build -o skills-seed ./cmd/skills-seed
```

## License

[MIT](LICENSE)
