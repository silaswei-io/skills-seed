<div align="center">

# Skills Seed

**从代码库学习项目规范，并生成 Claude Code / Codex 可用的本地 skills。**

[简体中文](README.md) · [English](README.en.md)

`Claude Code` · `Codex` · `Project Skills` · `Workspace`

[快速开始](#快速开始) · [Agent 支持](#agent-支持) · [命令参考](docs/COMMANDS.md) · [配置参考](docs/CONFIGURATION.md)

</div>

Skills Seed 面向已经存在的项目。它会读取当前代码、Git 历史和项目结构，把团队真实使用的命名、错误处理、目录组织、业务方法、工具方法、测试习惯和 API 约定沉淀为本地知识，再生成 Claude Code / Codex 可以直接加载的 skills。

所有学习结果默认保存在当前仓库的 `.skills-seed` 中。生成的 skills 会按当前 `agent.provider` 输出到 `.claude/skills` 或 `.agents/skills`，不需要把项目代码上传到远端知识库。

## 能力

Skills Seed 关注的是“让 AI 助手理解这个项目应该怎么写”：

1. 从当前代码学习项目规范，提取 patterns、业务方法、工具方法和最佳实践。
2. 从 Git 历史增量学习，跳过已分析过的 commit，保留团队长期演进出的写法。
3. 生成项目画像和项目规范，让 AI 助手理解模块职责、核心依赖、业务边界和改动约束。
4. 生成 Claude Code / Codex skills，包括 `SKILL.md`、项目概览、规范、patterns 和示例。
5. 支持 workspace 根仓，子项目可以独立学习和生成，根 skill 负责路由、跨项目关系和影响范围。
6. 支持 `check` 和 pre-commit hook，用已学习的规则检查暂存区或整个 Git 跟踪文件集合。

## 工作方式

```text
init -> learn current / learn history -> generate-skills -> check
```

`init` 创建 `.skills-seed` 和默认配置。`learn` 从代码或历史提交中学习项目规则。`generate-skills` 把项目画像和 patterns 渲染为当前 Agent 可用的 skills。`check` 用这些规则检查后续改动。

## Agent 支持

当前内置支持 `claude` 和 `codex` 两种 provider：

- `claude`：默认 Agent，生成到 `.claude/skills/skills-seed-skills`，供 Claude Code 加载。
- `codex`：生成到 `.agents/skills/skills-seed-skills`，供 Codex 加载。

初始化时可直接指定 Agent：

```bash
skills-seed init --mode project --agent codex --locale zh-CN
skills-seed init --workspace --children --agent codex --locale zh-CN
```

`--agent` 会写入 `agent.provider`，并确保 `agent.commands` 和 `output.skills_paths` 中存在对应 provider。workspace 初始化子项目时，新建的子项目会继承根仓 Agent 配置；已有子项目配置不会被覆盖。

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
- 可用的 AI Agent CLI：默认 `claude`，可用 `--agent codex` 切换到 Codex

## 快速开始

单项目：

```bash
cd your-project
skills-seed init --mode project --agent codex --locale zh-CN
skills-seed learn current
skills-seed generate-skills
```

Workspace：

```bash
cd your-workspace
skills-seed init --workspace --children --agent codex --locale zh-CN
skills-seed learn current
skills-seed generate-skills
```

常用命令：

```bash
skills-seed check
skills-seed profile show
skills-seed patterns merge --dry-run
skills-seed hook install
```

## 默认约定

- 默认初始化模式是 `project`。
- 默认 Agent 是 `claude`。
- 默认数据目录是 `.skills-seed`。
- Claude 输出到 `.claude/skills/skills-seed-skills`。
- Codex 输出到 `.agents/skills/skills-seed-skills`。
- workspace 初始化只扫描根目录第一层子项目。
- `workspace.init_children` 默认 `false`；显式使用 `--children` 或开启配置后才会补初始化子项目。
- `agent.parallelism` 默认 `0`，表示自动并发：project=1，workspace=子项目数，上限 6。

已有子项目 `.skills-seed/config.yaml` 不会被覆盖；如果子项目 agent 和根仓不同，只提示并保留子项目配置。

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

## License

[MIT](LICENSE)
