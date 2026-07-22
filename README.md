<div align="center">

# Skills Seed

**让 AI Agent 先理解项目规则，再开始改代码。**

[![CI](https://img.shields.io/github/actions/workflow/status/silaswei-io/skills-seed/ci.yml?branch=main&label=ci&logo=github&style=flat-square)](https://github.com/silaswei-io/skills-seed/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/silaswei-io/skills-seed?style=flat-square)](https://github.com/silaswei-io/skills-seed/releases/latest)
[![Go Version](https://img.shields.io/github/go-mod/go-version/silaswei-io/skills-seed?style=flat-square)](go.mod)
[![License](https://img.shields.io/github/license/silaswei-io/skills-seed?style=flat-square)](LICENSE)

[简体中文](README.md) · [English](README.en.md)

[核心特性](#核心特性) · [快速开始](#快速开始) · [真实项目案例](#真实项目案例) · [它会生成什么](#它会生成什么) · [Agent 成本建议](#agent-成本建议) · [常用命令](#常用命令) · [Workspace](#workspace) · [文档](#文档)

</div>

Skills Seed 面向已有代码库。它从当前代码、Git 历史、目录结构和检查命中记录中提取团队真实实践，把目录边界、业务入口、错误处理、工具方法、测试习惯和 API 约定沉淀成本地 Skills，供 Claude Code / Codex 直接加载。

适合你已经有一个老项目、业务系统或多仓 workspace，并希望 AI Agent 不再每次都从零理解项目规则。

## 核心特性

Skills Seed 的核心不是生成一份静态项目说明，而是让 Agent 的项目知识可以被学习、更新、协作和复用。

### 低成本上手和同步

- **交互式 `init` / `sync`**：第一次运行会引导选择项目模式、Agent、并发和执行计划；日常同步会自动判断首次生成、增量同步、续跑或重来。
- **自由配置**：Agent、Skills 目标、输出路径、语言、学习深度、切分粒度、并发、预算、排除规则、重试、hook 和 autofix 都可以通过配置或命令参数控制。

### 从真实代码沉淀知识

- **从代码中学习**：以当前代码、Git 历史、目录结构、check 命中和 review 记录为主要输入，不要求用户手写大段项目说明。
- **增量学习**：`learn current` 默认只处理新增、修改和删除文件，未变化文件跳过；删除文件会清理文件指纹并作为删除 diff 进入学习流程。
- **模式生命周期**：pattern 有 `active`、`stale`、`superseded`、`deprecated` 状态，默认只让 active 模式参与 check、generate 和统计；当前不自动物理删除过期模式，可用 `patterns delete` / `patterns compact` 治理。

### Skills 实时刷新、按需加载

- **模块化 Skills 同步**：生成入口 `SKILL.md` 和按职责拆分的 `references/`，Agent 先加载入口，再按任务读取模块；每次 `sync` 会按最新学习结果刷新本地 Skills。
- **Agent 与产物解耦**：可以用 Claude 分析项目，同时输出 Codex 可加载的 Skills；也可以反过来按团队成本和质量需求切换。

### 面向团队和复杂仓库

- **Workspace 支持**：根仓负责子项目路由、跨项目约束和影响范围，子项目保留自己的 `.skills-seed`、patterns 和生成产物。
- **本地数据，可团队协作**：配置、数据库、上下文、运行日志和生成的 Skills 都在仓库本地，团队可以按需提交 `.skills-seed/context/`、配置和生成结果。
- **自定义工作流**：`workflow` 命令可沉淀团队常用任务流程，让生成的 Skills 不只描述结构，也能指导实际改动步骤。
- **一次性上下文指导**：`--context` 和 `--context-path` 可给当前 `sync` / `learn current` 补充临时目标或边界，不污染长期项目上下文。

## 你会得到什么

生成结果不是泛泛的项目介绍，而是一套围绕当前仓库组织的 Agent 工作上下文：

- 哪些目录负责什么能力，改动时应该先看哪里。
- 哪些业务方法、工具方法、错误处理和测试习惯已经被团队长期使用。
- workspace 根仓中哪个子项目应该处理哪类需求，跨项目改动要按什么顺序看。
- 哪些规则在 `check` 或 review 中反复命中，应优先进入最终 Skills。

## 真实项目案例

以 [medusa-demo](https://github.com/silaswei-io/medusa-demo/tree/develop/.claude/skills/skills-seed-skills) 为例，直接使用默认 `sync` 后，Skills Seed 生成了面向 Medusa TypeScript monorepo 的项目级 Skill：入口 `SKILL.md` 只负责任务路由和约束来源，业务规则、架构边界、入口方法、验证命令和代码证据拆到 `references/` 中按需加载。Go 项目还会从真实 `go.mod` 与 `_test.go` 确定性生成多模块测试矩阵，不依赖 AI 是否学到测试命令。

仓库中的 `writing-docs`、`reviewing-prs` 等任务 Skill 负责固定流程和协作规则；Skills Seed 生成的项目 Skill 则在改代码前回答“这个项目应该怎么改、先读哪里、哪些规则有代码证据、该跑什么验证”。

medusa-demo 样例的实际生成结果、与其他 Skills 的对比和适用场景见 [Medusa Demo 案例](docs/MEDUSA_DEMO_CASE.md)。

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
| Codex | `.agents/skills/<project-name>-dev/SKILL.md` |
| Claude Code | `.claude/skills/<project-name>-dev/SKILL.md` |

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
.agents/skills/<project-name>-dev/
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

安装可用的 Agent CLI 后，可以在 `init` 时选择执行学习的 Agent 和生成 Skills 的目标格式。

## Agent 成本建议

`sync` / `learn current` 会把代码片段、结构信息和上下文交给 Agent 做批量分析，调用次数和 token 消耗都可能比较高。日常建议用速度快、价格低的 Agent 模型跑学习和同步；只有在规则质量明显不够、项目特别复杂或需要更强推理时，再切到更强模型。

先区分两个配置：

| 配置 | 作用 |
|---|---|
| `--agent` / `agent.engine` | 选择哪个 Agent CLI 执行分析和学习 |
| `--skills` / `skills.target` | 选择生成哪种 Agent 可加载的 Skills |

两者可以不同，例如用 Claude 分析项目，同时输出 Codex Skills。

初始化时直接指定：

```bash
skills-seed init --agent codex --skills codex
skills-seed init --agent claude --skills codex
skills-seed init --mode project --agent codex --skills codex --locale zh-CN --no-interactive
```

也可以直接修改 `.skills-seed/config.yaml`：

```yaml
agent:
  engine: "codex"
  commands:
    codex: "codex"
    claude: "claude"

skills:
  target: "codex"
```

模型档位由对应 Agent CLI 自己控制。若需要固定低成本模型，优先在 `codex` / `claude` CLI 的默认配置中设置；如果你的 CLI 只支持命令行参数，可以把 `agent.commands.<engine>` 指向一个包装脚本，由脚本内部调用带模型参数的 Agent CLI。

当前 `sync` / `learn current` 的文件筛选会先做本地过滤，再把候选元数据和可用的 CodeGraph 结构化上下文写入 runtime，由 AI 给出相关性建议并参与后续单元规划。缺少或异常的 CodeGraph 索引会自动初始化、同步或修复，只有真正不可用时才降级到内嵌 tree-sitter。候选/焦点文件清单会作为 runtime 输入文件按路径引用，不再直接堆进分析 prompt；文件筛选、分析单元规划和学习 prompt 也带有稳定决策规则，减少相同输入下的输出漂移。终端只展示关键阶段和精简的文件筛选结果，候选数量、耗时等排查细节写入运行时日志，避免大项目进度行被细节淹没。

从 0.13.11 开始，文件筛选、分析单元规划和模式策展属于自包含 prompt-only 任务；Claude 不再为这些任务重复读取仓库。`learn current` 会把分析结果和项目画像提交状态写入可恢复 checkpoint，策展或保存失败后使用 `sync --resume` 可直接继续，不会重跑已完成单元。终端会分别显示 AI 策展、结果校验和模式库写入，Token 汇总也会区分非缓存 Token 与上下文处理量。

从 0.13.2 开始，仓库通过 `staticcheck ./...` 清理静态检查噪音；除 `go test`、`go vet` 和构建外，发布前也建议运行 staticcheck 作为代码质量门禁。

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
| `skills-seed cli-skills` | 安装、更新或卸载由 skills-seed 管理的全局 CLI 操作 Skill |
| `skills-seed sync` | 交互式学习当前代码并刷新 Skills |
| `skills-seed workspace` | 管理 workspace 子项目 |
| `skills-seed patterns` | 查看、补充、修订或整理模式规则 |
| `skills-seed workflow` | 查看、添加或更新团队常用任务流程 |
| `skills-seed check` | 用已学习规则检查当前改动 |
| `skills-seed hook` | 安装、卸载或手动运行 Git hook |
| `skills-seed profile` | 查看或更新项目画像 |
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

推荐按这个顺序使用：

1. 先打开 `.skills-seed/context/README.md` 看填写指南。
2. 只修改和当前信息对应的文件；没有内容的段落可以留空。
3. 长期有效的信息写入 context 后，运行 `skills-seed sync` 让它进入后续学习和生成。
4. 只解释本次任务的限制或背景时，使用 `sync --context` 或 `sync --context-path`。

常见布局：

```text
.skills-seed/context/
├── README.md
├── background.md
├── constraints.md
├── terminology.md
└── workspace.md
```

| 场景 | 推荐方式 |
|---|---|
| 代码看不到的项目事实或背景 | 写入 `.skills-seed/context/background.md` |
| 长期团队规则、兼容性或禁止事项 | 写入 `.skills-seed/context/constraints.md` |
| 术语、别名、状态名 | 写入 `.skills-seed/context/terminology.md` |
| workspace 跨项目背景和协作约束 | 写入 `.skills-seed/context/workspace.md` |
| 本次同步临时说明 | 使用 `skills-seed sync --context` |
| 本次同步较长说明 | 使用 `skills-seed sync --context-path` |

默认占位文本不会进入 Agent 输入。不要复制代码、README 大段内容或一次性调试记录。

```bash
skills-seed sync --context "本次只关注兼容性边界"
skills-seed sync --context-path .skills-seed/run-context.md
```

## 文档

- [Medusa Demo 案例](docs/MEDUSA_DEMO_CASE.md)
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
