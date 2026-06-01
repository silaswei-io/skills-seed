<div align="center">

# Skills Seed

**让 AI Agent 记住你的项目规则。**

[![CI](https://img.shields.io/github/actions/workflow/status/silaswei-io/skills-seed/ci.yml?branch=main&label=ci&logo=github&style=flat-square)](https://github.com/silaswei-io/skills-seed/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/silaswei-io/skills-seed?style=flat-square)](https://github.com/silaswei-io/skills-seed/releases/latest)
[![Go Version](https://img.shields.io/github/go-mod/go-version/silaswei-io/skills-seed?style=flat-square)](go.mod)
[![License](https://img.shields.io/github/license/silaswei-io/skills-seed?style=flat-square)](LICENSE)

[简体中文](README.md) · [English](README.en.md)

`Claude Code` · `Codex` · `Local Skills` · `Workspace` · `Code Review`

[快速开始](#快速开始) · [产物预览](#产物预览) · [设计理念](#设计理念) · [Workspace](#workspace) · [命令参考](docs/COMMANDS.md)

</div>

Skills Seed 面向已经存在的项目。它读取当前代码、Git 历史、目录结构和已沉淀的检查命中，把团队真实使用的命名、错误处理、目录组织、业务方法、工具方法、测试习惯和 API 约定整理成本地知识，再生成 Claude Code / Codex 可以直接加载的 skills。

它解决的是一个很具体的问题：AI Agent 第一次进入项目时，通常不知道这个项目“应该怎么写”。Skills Seed 会把这些隐性规则从真实代码里挖出来，变成可加载、可更新、可检查的本地 skills。

你得到的不是泛泛的项目介绍，而是一套围绕当前项目生成的 Agent 工作上下文：

- 哪些目录负责什么能力，改动时应该先看哪里。
- 哪些业务方法、工具方法、错误处理和测试习惯已经被团队长期使用。
- workspace 根仓中哪个子项目应该处理哪类需求，跨项目改动要按什么顺序看。
- 哪些规则在 `check` 或 review 中反复命中，应优先进入最终 skills。

所有数据默认保存在当前仓库本地。生成目标由 `skills.target` 决定，执行分析、学习和摘要任务的 Agent CLI 由 `agent.engine` 决定。因此可以用 `claude` 做分析和总结，同时输出 `codex` 可加载的 skills。

## 为什么值得用

| 你遇到的问题 | Skills Seed 做什么 |
|---|---|
| 每次让 AI 改老项目，都要重新解释项目结构 | 从代码、Git 历史和项目画像生成可复用 skills |
| 团队规范散在代码、review 和个人经验里 | 提取 patterns、业务方法、工具方法和测试习惯 |
| 多子项目 workspace 不知道该读哪个上下文 | 根 skill 负责路由，子仓 skill 独立沉淀 |
| AI 容易把生成内容再次学习进去 | 默认排除 `.skills-seed`、skills 输出目录和构建产物 |
| 规则生成后不知道有没有用 | `check` 记录 pattern hits，`patterns stats` 展示命中和质量 |

## 适用场景

- 老项目或业务系统希望交给 AI Agent 协助修改，但不想每次都重新解释项目结构和约束。
- 团队已经有稳定写法、目录约定、错误处理习惯、业务方法和测试策略，需要把这些经验沉淀给 AI。
- workspace 根目录下有多个独立 Git 子项目，希望子项目独立学习和生成，根仓只负责路由和跨项目影响判断。
- 希望用 `check` / pre-commit hook 把已学习的规则用于后续改动检查，并记录规则命中情况。
- 希望用本地 review comment 统计，观察哪些常见评审问题已经被模式规则提前覆盖。

## 产物预览

一次 `generate-skills` 后，默认会得到类似这样的目录：

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
        ├── error.md
        └── testing.md
```

`SKILL.md` 是 Agent 的入口，`references/` 保存更完整的项目画像、规范和模式细节。Agent 需要深入时再读取 references，而不是把所有内容都塞进入口文档。

## 工作流

```text
init -> learn current / learn history -> generate-skills -> check
```

| 阶段 | 命令 | 产物 |
|---|---|---|
| 初始化 | `skills-seed init` | `.skills-seed/config.yaml`、本地数据库、默认 prompts |
| 学习当前代码 | `skills-seed learn current` | patterns、业务方法、工具方法、项目画像 |
| 学习历史提交 | `skills-seed learn history` | 从 Git 演进中提取长期规则 |
| 生成 skills | `skills-seed generate-skills` | `SKILL.md`、项目概览、规范、patterns references |
| 检查后续改动 | `skills-seed check` | 基于已学习规则的问题、修复建议和 pattern hits |

`generate-skills` 会按模式质量排序：优先沉淀综合分高、check 命中多、置信度高的规则，降低泛化规则和重复规则进入最终 skills 的概率。

## 快速开始

### 单项目

```bash
cd your-project
skills-seed init --mode project --agent codex --locale zh-CN
skills-seed learn current
skills-seed generate-skills
test -f .agents/skills/skills-seed-skills/SKILL.md
```

生成后，Codex 默认读取：

```text
.agents/skills/skills-seed-skills/SKILL.md
```

Claude Code 默认读取：

```text
.claude/skills/skills-seed-skills/SKILL.md
```

### Workspace

workspace 模式适用于一个根目录下管理多个独立 Git 子项目的场景。初始化时会扫描第一层目录，检测子项目，并为当时检测到的子项目初始化 `.skills-seed`。

```bash
cd your-workspace
skills-seed init --workspace --agent codex --locale zh-CN
skills-seed learn current
skills-seed generate-skills
test -f .agents/skills/skills-seed-skills/SKILL.md
```

后续如果把新项目拷入 workspace 根目录，使用 `add` 同步配置并初始化子仓：

```bash
skills-seed add .
skills-seed add backend frontend
```

workspace 根仓只负责编排、路由和跨项目关系；子项目用自己的 `.skills-seed` 独立学习、生成和保存 patterns。已有子项目 `.skills-seed/config.yaml` 不会被覆盖，如果子项目 agent 与根仓不同，只提示并保留子项目配置。

## 设计理念

- **Local-first**：学习结果、配置和生成产物默认保存在当前仓库，不依赖远端知识库。
- **Existing-code-first**：以真实代码、Git 历史和检查命中为依据，而不是让用户手写一大份项目说明。
- **Agent-agnostic**：`agent.engine` 和 `skills.target` 分离，可以用一种 Agent 分析，生成另一种 Agent 的 skills。
- **Workspace-aware**：根仓负责路由和跨项目关系，子仓独立学习和生成，避免根仓污染子仓上下文。
- **Feedback-driven**：`check` 和 review hits 会反哺模式质量，让真正用得上的规则更容易进入最终 skills。

## Agent 与 Skills 目标

`agent.engine` 表示用哪个 Agent CLI 执行分析、学习和生成摘要。`skills.target` 表示生成哪种 Agent 可加载的 skills。

例如，用 Claude 做分析和总结，但生成 Codex skills：

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

内置目标：

| 名称 | 用途 | 默认输出 |
|---|---|---|
| `claude` | Claude Code skills | `.claude/skills/skills-seed-skills` |
| `codex` | Codex skills | `.agents/skills/skills-seed-skills` |

## 常用命令

| 命令 | 说明 |
|---|---|
| `skills-seed init` | 初始化单项目或 workspace 根仓 |
| `skills-seed add .` | 在 workspace 根仓自动检测并添加所有子项目 |
| `skills-seed add <child...>` | 在 workspace 根仓添加指定子项目 |
| `skills-seed learn current` | 从当前代码增量学习规则和画像 |
| `skills-seed learn history` | 从 Git 历史提交学习长期规则 |
| `skills-seed generate-skills` | 生成当前 `skills.target` 的 skills |
| `skills-seed check` | 检查暂存区或 Git 跟踪文件 |
| `skills-seed patterns stats` | 查看模式质量、命中次数和最近命中 |
| `skills-seed review import --from-file` | 导入本地评审评论 |
| `skills-seed hook install` | 安装本地 pre-commit hook |

完整参数见 [命令参考](docs/COMMANDS.md)。

## 本地与安全边界

- 默认不上传项目代码到远端知识库；学习结果写入当前仓库的 `.skills-seed`。
- `check` 和 `generate-skills` 会调用配置中的 Agent CLI，因此是否联网取决于你使用的 `claude` / `codex` CLI。
- 生成的 skills 目录、`.git/**`、`.skills-seed/**` 以及常见构建产物默认会被排除，避免生成内容回流到下一轮学习。
- 手写 `SKILL.md` 如果没有 `generated-by: skills-seed` 标记，默认不会被覆盖。

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
- 可用的 AI Agent CLI：默认 `claude`，可通过 `--agent codex` 或配置中的 `agent.engine` 切换

## 文档

- [命令参考](docs/COMMANDS.md)
- [配置参考](docs/CONFIGURATION.md)
- [Changelog](CHANGELOG.md)

## 开发

```bash
go test ./...
go vet ./...
go build -o skills-seed ./cmd/skills-seed
```

---

<div align="center">

基于 [MIT License](LICENSE) 发布。

</div>
