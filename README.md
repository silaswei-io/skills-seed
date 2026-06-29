<div align="center">

# Skills Seed

**让 AI Agent 记住你的项目规则。**

[![CI](https://img.shields.io/github/actions/workflow/status/silaswei-io/skills-seed/ci.yml?branch=main&label=ci&logo=github&style=flat-square)](https://github.com/silaswei-io/skills-seed/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/silaswei-io/skills-seed?style=flat-square)](https://github.com/silaswei-io/skills-seed/releases/latest)
[![Go Version](https://img.shields.io/github/go-mod/go-version/silaswei-io/skills-seed?style=flat-square)](go.mod)
[![License](https://img.shields.io/github/license/silaswei-io/skills-seed?style=flat-square)](LICENSE)

[简体中文](README.md) · [English](README.en.md)

`Claude Code` · `Codex` · `Local Skills` · `Workspace` · `Code Review`

[快速开始](#快速开始) · [产物预览](#产物预览) · [Prompt 与一次性说明](#prompt-与一次性说明) · [设计理念](#设计理念) · [Workspace](#workspace) · [命令参考](docs/COMMANDS.md)

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

一次 `generate skills` 后，默认会得到类似这样的目录：

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
init -> learn current / learn history -> generate skills -> check
```

| 阶段 | 命令 | 产物 |
|---|---|---|
| 初始化 | `skills-seed init` | `.skills-seed/config.yaml`、本地数据库、默认 prompts |
| 学习当前代码 | `skills-seed learn current` | patterns、业务方法、工具方法、项目画像 |
| 学习历史提交 | `skills-seed learn history` | 从 Git 演进中提取长期规则 |
| 生成 skills | `skills-seed generate skills` | `SKILL.md`、项目概览、规范、patterns references |
| 检查后续改动 | `skills-seed check` | 基于已学习规则的问题、修复建议和 pattern hits |

0.9.0 起，模式去重和整合前移到入库阶段。`learn current`、`learn history` 和 `patterns add` 产生的候选模式会先经过 AI 策展和服务端校验，再写入本地模式库；`generate skills` 只读取已入库数据，不再承担合并或修正模式库的职责。需要显式整理历史模式库时，使用 `skills-seed patterns compact`。

0.10.4 起，默认入库策展使用本地确定性合并，并在内部按 pattern ID 维护唯一集合；当候选模式复用已有 ID，或历史模式库存在重复 ID 时，会先收敛为单条更高质量的模式再写入，避免重复 curated pattern ID 触发结构校验降级。

0.10.5 起，`learn current` 的单元分析不再把已有模式库注入每个单元 prompt，避免上下文随已保存模式数量持续膨胀；学习完成后的模式去重仍由本地确定性合并负责，需要语义级整理时可显式执行 `skills-seed patterns compact --ai`。

0.9.1 起，`learn current` 在候选文件较多时可先通过 AI 文件筛选收敛分析范围；`generate skills` 显式执行时会删除旧的 skills-seed 生成目录并完整重建。根命令中的 `completion` 已移除，中文 help 文案已统一。

`generate skills` 会按模式质量排序：优先沉淀综合分高、check 命中多、置信度高的规则，降低泛化规则和重复规则进入最终 skills 的概率。

0.7.0 起，学习和项目画像分析会在有边界输入时使用内嵌 tree-sitter 做轻量结构化预扫描，提取符号、导入、入口点和模块线索，辅助 Agent 优先判断需要读取的源码。它不再依赖外部 CodeGraph 命令或索引；配置位于 `analysis.structural`，其中 `max_symbols` 控制写入结构化上下文的符号数，`max_file_size` 控制单个源码文件大小上限。

AI Agent 遇到 429 / 529 / overloaded 这类可重试错误时会按 `agent.retry` 指数退避重试。长耗时步骤的进度行会在正常、等待重试、重试中三种状态间切换，例如 `分析当前代码库`、`分析当前代码库（API Error: 529 overloaded_error，本次调用 3m37s，15s 后重试）`、`分析当前代码库（第2次尝试）`；其中“本次调用”是失败前的 Agent 调用耗时，“15s 后重试”才是退避等待时间。

## Prompt 与一次性说明

`skills-seed init` 会生成 `.skills-seed/prompts/`。这些文件不是用来替换内置 prompt 的完整模板，而是会与内置 prompt 合并，作为项目上下文、workspace 约束或用户补充指令参与学习和生成。

0.7.1 起，默认 prompt 文件中的生成元数据、空脚手架和未填写占位内容会在渲染时自动过滤；只有用户实际写入的约束会进入 Agent 输入。每次渲染后的 prompt 会保存在 `.skills-seed/runtime/rendered-prompts/`，同目录 `.manifest.json` 会记录 base、project、workspace、instructions 等片段是否参与合并和各自长度，便于排查上下文来源。

0.7.2 起，项目画像分析会对模型输出中对象数组里的重复对象起始片段做窄范围 JSON 恢复；如果仍无法解析，会返回错误并保留已有画像，不再把 `unknown/解析失败` 占位画像当作成功结果保存。

0.7.3 起，当前代码学习会在 pattern 保存成功后才提交文件分析指纹，避免保存失败的文件在后续增量学习中被误判为已学习。Pattern、文件指纹、命中和评审评论记录会维护 `created_at/updated_at`，业务方法代码位置会以语言无关的快照元数据保存到 DB；使用 `patterns show <pattern-id>` 可查看单条模式的完整详情。

0.9.8 起，模式会单独保存 `evidence_locations` 作为模式级源码证据位置；`patterns show` 概览优先展示业务/工具方法的 `code_location`，没有业务方法时回退展示第一条证据位置，并在详情页输出完整证据位置列表。

0.8.0 起，Agent 输出会单独保存在 `.skills-seed/runtime/agent-outputs/`，运行日志只记录输出长度和归档路径，不再写入模型回复预览或 stdout/stderr 明文。0.10.3 起，合法 JSON 输出会在 `.md` 归档中格式化为可读的 `json` 代码块。业务方法位置统一使用 `code_location` 结构化元数据，生成的 business methods reference 会展示位置状态；项目 skill 和 references 也更紧凑，入口文档会引导 Agent 按任务读取最小必要参考。

0.9.6 起，`.skills-seed/runtime` 下的调试记录统一使用 `YYYYMMDD-HHMMSS.NNNNNNNNN-<kind>-<name>` 文件名前缀，包括 rendered prompt、Agent 输出归档和运行时输入临时目录，便于按时间排序排查一次运行中的上下文与模型输出。

0.9.0 起，学习和用户添加模式时会使用 `pattern-curate` 提示词做入库前策展：候选模式必须覆盖、重复规则必须整合、代码证据只能来自输入源码，非法或低质量候选会被丢弃。旧的生成前合并流程和 `patterns merge` 已移除，生成阶段保持只读。

0.9.1 起，模型输出解析会先经过更稳健的 JSON 修复流程，覆盖重复对象起始、非法转义、字符串内未转义引号和缺失闭合容器等常见异常。0.10.5 起，该流程进一步覆盖字符串内原始换行/控制字符、裸对象键和数组项缺失对象起始符等长上下文输出异常。

常见目录：

```text
.skills-seed/
├── config.yaml                 # 工具配置
├── prompts/                    # 可编辑 prompt 片段
├── store/                      # 持久化数据，不应删除
│   ├── project.db              # patterns、命中、指纹、评审等索引数据
│   └── documents/              # 可读 JSON 文档，例如画像、规范、状态和变更记录
├── cache/                      # 可重建缓存，例如文件快照和未完成分析计划
└── runtime/                    # 可删除运行时产物，例如日志、渲染 prompt 和 Agent 输出

.skills-seed/prompts/
├── project/
│   ├── project-profile.md      # 项目事实画像，所有相关 prompt 都会参考
│   ├── common.md               # 项目通用约束，所有相关 prompt 都会参考
│   └── <prompt-id>.md          # 可选：某个 prompt 的项目级补充
├── workspace/
│   ├── skill-workspace-profile.md
│   └── skill-workspace-spec.md
└── instructions/
    └── <prompt-id>.md          # 用户补充指令，追加到对应 prompt
```

运行时的合并顺序是：

```text
内置 prompt
+ project/project-profile.md
+ project/common.md
+ project/<prompt-id>.md
+ workspace/<prompt-id>.md
+ instructions/<prompt-id>.md
+ 内置最终输出契约
```

`instructions/<prompt-id>.md` 适合写稳定的团队要求，例如“学习 commit 时忽略临时调试代码”或“生成 skill 时优先保留 API 兼容性规则”。它会追加到内置 prompt 后面，但不能改变内置 prompt 要求的 JSON / Markdown 输出格式；最后会追加一个不可编辑的内置输出契约，避免用户补充指令破坏解析。

`--context` 和 `--context-file` 是学习阶段的一次性说明，只影响当前 `learn current` 命令，不会写入 `.skills-seed/prompts/`，也不会作为临时输入传给 `generate skills`。它们适合临时要求，例如：

```bash
skills-seed learn current --context "本次只关注兼容性边界"
skills-seed learn current --context-file .skills-seed/context.md
```

如果同一条规则长期有效，写入 `.skills-seed/prompts/instructions/<prompt-id>.md`；如果只是这次运行的解释或限制，使用 `--context` 或 `--context-file`。

`learn current`、`preview` 和结构化分析共用同一套文件选择策略：默认只分析源码、构建配置和依赖配置，继续跳过文档、生成产物、Git ignore 命中的路径、全局 `exclude` 命中的路径以及已生成 Skills 输出目录。

## 快速开始

### 单项目

```bash
cd your-project
skills-seed init --mode project --agent codex --skills codex --locale zh-CN
skills-seed learn current
skills-seed generate skills
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

workspace 模式适用于一个根目录下管理多个独立 Git 子项目的场景。初始化时会扫描第一层目录，只有拥有独立 `.git` 的目录会进入 `workspace.projects`，并为当时检测到的子项目初始化 `.skills-seed`。`go.mod`、`package.json`、安装脚本、Helm/Terraform 等文件只用于识别子项目类型和语言。

```bash
cd your-workspace
skills-seed init --workspace --agent codex --skills codex --locale zh-CN
skills-seed learn current
skills-seed generate skills
test -f .agents/skills/skills-seed-skills/SKILL.md
```

后续如果把新项目拷入 workspace 根目录，使用 `workspace add` 同步配置并初始化子仓：

```bash
skills-seed workspace add .
skills-seed workspace add backend frontend
```

workspace 根仓只负责编排、路由和跨项目关系；子项目用自己的 `.skills-seed` 独立学习、生成和保存 patterns。已有子项目 `.skills-seed/config.yaml` 不会被覆盖，如果子项目 agent 与根仓不同，只提示并保留子项目配置。

workspace 用户配置只保留 `workspace.projects`。公共库、契约和基础设施影响不再通过 `workspace.shared`、`workspace.contracts`、`workspace.infra` 手填，而是在 `learn current` 阶段结合仓库证据、子项目 `project-profile.json` 和一次性用户说明分析并沉淀到根仓 `workspace-profile.json` / `workspace-spec.json`；`generate skills` 只消费这些已沉淀结果，不再接收用户说明。

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
| `skills-seed workspace add .` | 在 workspace 根仓自动检测并添加所有子项目 |
| `skills-seed workspace add <child...>` | 在 workspace 根仓添加指定子项目 |
| `skills-seed learn current` | 从当前代码增量学习规则和画像 |
| `skills-seed learn history` | 从 Git 历史提交学习长期规则 |
| `skills-seed generate skills` | 生成当前 `skills.target` 的 skills |
| `skills-seed workflow --context "<说明>"` | 通过 Agent 从用户传入内容推导并保存工作流；未提供 `--name` 时自动生成名称，同名默认合并，重写使用 `--overwrite` |
| `skills-seed patterns add <描述>` | 用自然语言补充用户自定义模式 |
| `skills-seed patterns compact` | 显式整理已入库的相似 patterns |
| `skills-seed sync` | 一键执行学习/添加模式，然后生成 skills |
| `skills-seed check` | 检查暂存区或 Git 跟踪文件 |
| `skills-seed patterns stats` | 查看模式质量、命中次数和最近命中 |
| `skills-seed patterns show` | 查看 DB 中的 pattern 时间、业务方法位置和模式证据位置 |
| `skills-seed review import --from-file` | 导入本地评审评论 |
| `skills-seed hook install` | 安装本地 pre-commit hook |

完整参数见 [命令参考](docs/COMMANDS.md)。

用户传入的目标、约束、背景、路径或口语化说明会先经当前 Agent 推导为标准工作流，再保存到 `.skills-seed/workflows/<id>/WORKFLOW.md`；未提供 `--name` 时，`<id>` 来自 Agent 生成的英文标题 slug，重复标题会追加序号。原始输入记录和元数据写入同目录 `metadata.yaml`。同名工作流默认合并去重，完全替换时加 `--overwrite`。生成 skills 时写入 `workflows/`，关联脚本统一放到 `scripts/workflows/<id>/`。

## 本地与安全边界

- 默认不上传项目代码到远端知识库；学习结果写入当前仓库的 `.skills-seed`。
- `check` 和 `generate skills` 会调用配置中的 Agent CLI，因此是否联网取决于你使用的 `claude` / `codex` CLI。
- `.skills-seed/store/project.db` 是本地 BoltDB 文件，同一时间只能被一个 `skills-seed` 进程写入或打开；如果另一个命令正在学习、整理或查看 patterns，新的命令可能提示数据库正在被占用，等待当前命令结束后重试即可。
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
- 可用的 AI Agent CLI：默认 `claude`，可通过 `--agent codex` 或配置中的 `agent.engine` 切换；生成目标可通过 `--skills codex` 或配置中的 `skills.target` 切换

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
