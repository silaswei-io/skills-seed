<div align="center">

# Skills Seed

**让 AI Agent 先理解项目规则，再开始改代码。**

[![CI](https://img.shields.io/github/actions/workflow/status/silaswei-io/skills-seed/ci.yml?branch=main&label=ci&logo=github&style=flat-square)](https://github.com/silaswei-io/skills-seed/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/silaswei-io/skills-seed?style=flat-square)](https://github.com/silaswei-io/skills-seed/releases/latest)
[![Go Version](https://img.shields.io/github/go-mod/go-version/silaswei-io/skills-seed?style=flat-square)](go.mod)
[![License](https://img.shields.io/github/license/silaswei-io/skills-seed?style=flat-square)](LICENSE)

[简体中文](README.md) · [English](README.en.md)

[快速开始](#快速开始) · [它会生成什么](#它会生成什么) · [常用命令](#常用命令) · [Workspace](#workspace) · [文档](#文档) · [HTML 页面](docs/html/skills-seed.html)

</div>

Skills Seed 面向已有代码库。它从当前代码、Git 历史、目录结构和检查命中记录中提取团队真实实践，把目录边界、业务入口、错误处理、工具方法、测试习惯和 API 约定沉淀成本地 Skills，供 Claude Code / Codex 直接加载。

适合你已经有一个老项目、业务系统或多仓 workspace，并希望 AI Agent 不再每次都从零理解项目规则。

## 核心特性

- **两步使用**：`init` 负责交互式初始化，`sync` 负责学习、续跑和刷新 Skills。
- **以现有代码为准**：优先从真实代码、Git 历史和检查命中提取规则，而不是要求手写大段项目说明。
- **Agent 与产物解耦**：可以用 Claude 分析项目，同时输出 Codex 可加载的 Skills。
- **支持 workspace**：根仓负责路由和跨项目关系，子项目独立沉淀自己的上下文。
- **本地优先**：配置、数据库、运行日志和生成产物默认保存在当前仓库。
- **保守学习**：学习提示词优先丢弃弱证据和事实摘要，只沉淀能指导后续改动的项目特有模式。

## 你会得到什么

生成结果不是泛泛的项目介绍，而是一套围绕当前仓库组织的 Agent 工作上下文：

- 哪些目录负责什么能力，改动时应该先看哪里。
- 哪些业务方法、工具方法、错误处理和测试习惯已经被团队长期使用。
- workspace 根仓中哪个子项目应该处理哪类需求，跨项目改动要按什么顺序看。
- 哪些规则在 `check` 或 review 中反复命中，应优先进入最终 Skills。

## 快速开始

在 Git 项目根目录执行：

```bash
cd your-project
skills-seed init
skills-seed sync
```

`init` 和 `sync` 都有交互界面：

| 命令 | 作用 |
|---|---|
| `skills-seed init` | 初始化当前项目或 workspace，选择 Agent、语言、并发和执行计划 |
| `skills-seed sync` | 学习当前代码，有变化时刷新 Skills；发现未完成任务时提示续跑或重来 |

默认生成入口：

| 目标 | 路径 |
|---|---|
| Codex | `.agents/skills/skills-seed-skills/SKILL.md` |
| Claude Code | `.claude/skills/skills-seed-skills/SKILL.md` |

### 选择使用方式

| 使用方式 | 适合场景 | 命令入口 |
|---|---|---|
| 交互式单项目 | 第一次给已有项目生成 Skills | `skills-seed init` → `skills-seed sync` |
| Workspace | 根目录下有多个独立 Git 子项目 | `skills-seed init` 中选择 workspace → `skills-seed sync` |
| CI / 脚本 | 团队模板或自动化环境中不适合交互 | `skills-seed init --no-interactive`、`skills-seed sync --resume --no-interactive` |

## 它解决什么问题

| 你遇到的问题 | Skills Seed 的处理方式 |
|---|---|
| AI 每次进项目都不知道该读哪些文件 | 生成入口 `SKILL.md`，按任务引导 Agent 读取相关 references |
| 团队规范只存在于代码和 review 经验里 | 从代码、Git 历史和检查命中中提取 patterns |
| 老项目有业务入口、工具方法和隐性边界 | 沉淀业务方法、模块职责、项目画像和验证建议 |
| workspace 下多个子项目上下文容易混乱 | 根仓负责路由，子项目独立学习和生成 Skills |
| 后续改动可能偏离已学习规则 | 用 `check` / hook 检查改动并记录规则命中 |

## 工作方式

```text
existing code / git history / check hits
        -> .skills-seed/store
        -> generated Skills
        -> Claude Code / Codex loads SKILL.md
```

1. `init` 创建 `.skills-seed/config.yaml`、本地数据库和可编辑项目上下文。
2. `sync` 分析当前代码和历史沉淀，保存 patterns、项目画像、业务入口和验证建议。
3. 生成的 `SKILL.md` 只作为入口，Agent 需要深入时再读取 `references/`。

## 它会生成什么

一次 `skills-seed sync` 后，Codex 目标默认会生成：

```text
.agents/skills/skills-seed-skills/
├── SKILL.md
├── agents/
│   └── openai.yaml
└── references/
    ├── project-overview.md
    ├── project-spec.md
    ├── business-methods.md
    ├── modules.md
    └── patterns/
        ├── business.md
        ├── concurrency.md
        ├── config.md
        ├── database.md
        ├── error.md
        ├── middleware.md
        ├── structure.md
        └── utils.md
```

`SKILL.md` 是 Agent 入口；`references/` 保存更完整的项目画像、规范、业务入口和模式细节。Agent 先读入口，再按任务深入相关参考文件，避免一次性加载过多上下文。

## 安装

```bash
go install github.com/silaswei-io/skills-seed/cmd/skills-seed@latest
skills-seed --version
```

如果命令不可用，请确认 `$GOPATH/bin` 或 `$GOBIN` 已加入 `PATH`。

源码构建：

```bash
git clone https://github.com/silaswei-io/skills-seed.git
cd skills-seed
go build -o skills-seed ./cmd/skills-seed
./skills-seed --help
```

## 前置要求

| 依赖 | 要求 |
|---|---|
| Go | `go.mod` 当前要求 Go 1.25.6+ |
| Git | 需要在 Git 仓库中初始化和学习 |
| Agent CLI | 默认使用 `claude`，也可以在初始化时选择 `codex` |

`agent.engine` 决定用哪个 Agent CLI 执行分析和学习；`skills.target` 决定生成哪种 Agent 可加载的 Skills。两者可以不同，例如用 Claude 分析项目，同时输出 Codex Skills。

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

## 与手写项目说明的区别

| 手写说明 | Skills Seed |
|---|---|
| 依赖人维护，容易过期 | 从当前仓库和 Git 历史增量更新 |
| 通常只有概览，缺少证据位置 | patterns、业务入口和 references 保留来源线索 |
| 多项目 workspace 容易混在一起 | 根仓路由，子项目独立沉淀 |
| 很难反馈哪些规则真的有用 | `check` 和 review hits 会反哺模式质量 |

## 常用命令

日常只需要 `init` 和 `sync`。其它主命令用于维护、补充和排查；子命令、参数和完整示例见 [命令参考](docs/COMMANDS.md)。

| 命令 | 用途 |
|---|---|
| `skills-seed init` | 交互式初始化当前项目或 workspace |
| `skills-seed sync` | 交互式学习当前代码并刷新 Skills |
| `skills-seed workspace` | 管理 workspace 子项目 |
| `skills-seed patterns` | 查看、补充、修订或整理模式规则 |
| `skills-seed workflow` | 添加或更新团队常用任务流程 |
| `skills-seed check` | 用已学习规则检查当前改动 |
| `skills-seed hook` | 安装、卸载或手动运行 Git hook |
| `skills-seed profile` | 查看或刷新项目画像 |
| `skills-seed preview` | 预览同步时可能纳入的文件范围 |
| `skills-seed log` | 查看学习和生成变更记录 |

脚本或 CI 中不适合交互时：

```bash
skills-seed init --mode project --agent codex --skills codex --locale zh-CN --no-interactive
skills-seed sync --resume --no-interactive
```

## Workspace

workspace 模式适用于一个根目录下管理多个独立 Git 子项目的场景。根仓负责路由和跨项目关系，子项目使用自己的 `.skills-seed` 独立学习、生成和保存 patterns。

```bash
cd your-workspace
skills-seed init
skills-seed sync
```

在 `init` 界面中选择 workspace 模式即可。初始化时会扫描第一层目录，只有拥有独立 `.git` 的目录会进入 `workspace.projects`。后续新增子项目时，使用 `skills-seed workspace` 管理，具体命令见 [Workspace 命令](docs/COMMANDS.md#skills-seed-workspace)。

## 本地数据与安全边界

`skills-seed init` 会创建 `.skills-seed/`：

| 路径 | 说明 |
|---|---|
| `.skills-seed/config.yaml` | 当前项目配置 |
| `.skills-seed/store/project.db` | patterns、命中、文件指纹和评审评论索引 |
| `.skills-seed/store/documents/` | 项目画像、规范、状态和变更记录 |
| `.skills-seed/context/` | 可编辑项目上下文，用于代码看不到的信息和长期规则 |
| `.skills-seed/cache/` | 可重建缓存 |
| `.skills-seed/runtime/` | 日志、渲染 prompt、Agent 输出和临时输入 |

Skills Seed 不维护远端知识库；学习结果默认写入当前仓库。`sync` 和 `check` 会调用配置中的 Agent CLI，是否联网取决于你使用的 `claude` / `codex`。

## 项目上下文

`skills-seed init` 会生成 `.skills-seed/context/`。这些文件不是内置 prompt 覆盖目录，而是给 AI 学习、检查和生成时参考的项目上下文。

| 场景 | 推荐方式 |
|---|---|
| 代码看不到的项目事实或背景 | 写入 `.skills-seed/context/project.md` |
| 长期团队规则、兼容性或禁止事项 | 写入 `.skills-seed/context/rules.md` |
| 术语、别名、状态名 | 写入 `.skills-seed/context/glossary.md` |
| 本次同步临时说明 | 使用 `skills-seed sync --context` |
| 本次同步较长说明 | 使用 `skills-seed sync --context-path` |

长期有效的信息放进 context；只解释本次任务的限制或背景，使用 `sync --context`。

## HTML 页面

面向使用者的静态介绍页可直接在浏览器打开：

[打开 `docs/html/skills-seed.html`](docs/html/skills-seed.html)

## 文档

- [命令参考](docs/COMMANDS.md)
- [配置参考](docs/CONFIGURATION.md)
- [更新日志](CHANGELOG.md)
- [Contributing](CONTRIBUTING.md)

## 开发

```bash
make build
make test
make lint
```

也可以直接使用 Go 命令：

```bash
go test ./...
go vet ./...
go build -o skills-seed ./cmd/skills-seed
```

---

<div align="center">

基于 [MIT License](LICENSE) 发布。

</div>
