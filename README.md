<div align="center">

# Skills Seed

**让 AI Agent 先理解项目规则，再开始改代码。**

[![CI](https://img.shields.io/github/actions/workflow/status/silaswei-io/skills-seed/ci.yml?branch=main&label=ci&logo=github&style=flat-square)](https://github.com/silaswei-io/skills-seed/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/silaswei-io/skills-seed?style=flat-square)](https://github.com/silaswei-io/skills-seed/releases/latest)
[![Go Version](https://img.shields.io/github/go-mod/go-version/silaswei-io/skills-seed?style=flat-square)](go.mod)
[![License](https://img.shields.io/github/license/silaswei-io/skills-seed?style=flat-square)](LICENSE)

[简体中文](README.md) · [English](README.en.md)

[Wiki](docs/wiki/Home.md) · [快速开始](docs/wiki/Getting-Started.md) · [命令参考](docs/COMMANDS.md) · [配置参考](docs/CONFIGURATION.md)

</div>

Skills Seed 面向已有代码库。它将用户维护的项目约束、当前源码和可验证的项目知识组织为本地 Skills，让 Claude Code、Codex 等 Agent 在改动前理解规则、模块归属、可复用能力和影响边界。

它不是远端知识库，也不替代代码审查、测试或项目负责人判断。学习结果、用户规则、运行归档和生成产物默认保留在项目本地，可审阅、可更新、可提交。

## 快速开始

在 Git 项目根目录执行：

```bash
cd your-project
skills-seed init
skills-seed sync
```

先从[最新发布](https://github.com/silaswei-io/skills-seed/releases/latest)下载当前系统和架构对应的二进制，将其放入 `PATH` 后再执行以上命令。`init` 设置项目模式、分析 Agent 与生成目标；`sync` 学习当前代码并刷新生成的 Skills。后续可执行 `skills-seed update` 自动下载并校验官方发布，不需要 Go。完整前置条件、结果验证、单项目与 workspace 选择见 [快速开始 Wiki](docs/wiki/Getting-Started.md)。

## 它提供什么

- **明确规则优先**：团队用 Rule 保存不可从源码推导的强制约束和命令边界。
- **证据化项目知识**：从当前代码、结构和变更证据中沉淀模块地图、能力入口和可复用模式。
- **模块化 Skills**：Agent 先读简洁入口，再按任务加载 references、rules 与 workflows。
- **持续同步**：增量学习、焦点审查和可恢复状态让知识随项目演进更新。
- **Workspace 路由**：根目录处理跨项目关系，子项目保留独立知识与生成产物。

## 文档

| 需要解决的问题 | 阅读 |
|---|---|
| 第一次初始化、日常同步、恢复与排障 | [Wiki](docs/wiki/Home.md) |
| 所有命令和参数 | [命令参考](docs/COMMANDS.md) |
| 配置字段、默认值和运行时目录 | [配置参考](docs/CONFIGURATION.md) |
| 真实项目的生成效果 | [Medusa Demo 案例](docs/MEDUSA_DEMO_CASE.md) |
| 产品验收边界与维护者约束 | [最终目标](docs/ULTIMATE_GOAL.md) |
| 版本变化 | [更新日志](CHANGELOG.md) |
| 参与开发 | [Contributing](CONTRIBUTING.md) |

各文档的唯一职责与维护约定见 [Wiki 源说明](docs/wiki/README.md)。

## 开发

```bash
go test ./...
go vet ./...
staticcheck ./...
go build ./cmd/skills-seed
```

---

<div align="center">

基于 [MIT License](LICENSE) 发布。

</div>
