# 快速开始

本页完成一次单项目初始化。workspace 请阅读 [Workspace](Workspace.md)。

> **适用场景：** 你准备第一次为已有 Git 项目生成 Skills，或需要重新确认首次初始化的最短路径。

## 前置条件

- 当前目录是 Git 项目，或可从当前子目录找到 Git 根目录。
- 已安装 `skills-seed`。
- 已安装并可运行用于学习的 Agent CLI。分析 Agent 与生成 Skills 的目标可以不同。

从[最新发布](https://github.com/silaswei-io/skills-seed/releases/latest)下载当前系统和架构对应的压缩包，解压后将 `skills-seed`（Windows 为 `skills-seed.exe`）放入 `PATH`，再验证：

```bash
skills-seed --version
```

以后更新已安装 CLI：

```bash
skills-seed update
```

该命令下载并校验官方发布资产，不要求本机安装 Go，也不会影响项目 `.skills-seed`。从源码构建仅适用于开发者：

```bash
go build -o skills-seed ./cmd/skills-seed
./skills-seed --version
```

## 初始化项目

在项目根目录执行：

```bash
skills-seed init
```

交互式初始化会选择项目模式、分析 Agent、生成目标、语言、并发和基础配置。脚本环境可显式给出这些选择：

```bash
skills-seed init --mode project --agent codex --skills codex --locale zh-CN --no-interactive
```

初始化后，`.skills-seed/` 是项目知识的本地工作区。它保存配置、用户资源、学习状态、运行归档和本地数据；是否提交其中的可协作内容由团队 Git 策略决定。

## 选择分析 Agent 与产物目标

`agent.engine` 选择执行分析和学习的 Agent CLI，`skills.target` 选择最终 Skills 的消费目标。两者可以不同，例如用一个 Agent 学习项目，同时生成另一种 Agent 可加载的 Skills。

日常同步优先选择速度和成本合适的模型；只有在项目复杂、证据判断不足或生成质量需要更强推理时，再提高模型能力。模型名称、目标路径和所有配置字段以 [配置参考](Reference.md) 链接的权威文档为准。

## 第一次学习并生成

```bash
skills-seed sync
```

`sync` 是推荐的日常入口：它先学习当前变更，成功后再生成 Skills。首次运行通常会分析较多文件，后续运行优先使用增量状态。

生成目标由 `skills.target` 决定，常见入口为：

| 目标 | 默认入口 |
|---|---|
| Codex | `.agents/skills/<project>-dev/SKILL.md` |
| Claude Code | `.claude/skills/<project>-dev/SKILL.md` |

## 验证结果

1. 查看终端的学习和生成完成摘要。
2. 打开生成的 `SKILL.md`，确认它能将任务路由到项目 references、用户 rules 和 workflows。
3. 使用 `skills-seed profile show`、`skills-seed patterns stats` 或 `skills-seed log` 检查学习状态。
4. 让目标 Agent 在真实改动前加载入口 Skill，并确认它能定位正确模块和约束。

若第一次学习中断，不要先删除 `.skills-seed`；按 [运行、恢复与排障](Operations.md) 判断能否 `sync --resume`。

## 下一步

- 想知道学习结果包含什么，读 [核心概念](Concepts.md)。
- 想控制范围、模型、并发或输出路径，读 [配置与参考](Reference.md)。
- 想写入不可从源码推导的约束，读 [Rule 与 Workflow](Rules-and-Workflows.md)。

---

**下一步：** [核心概念](Concepts.md) - 区分 Rule、Workflow、Context 和源码学习知识。

**相关主题：** [Workspace](Workspace.md) - 初始化多项目根目录；[学习失败与人工修正](Learning-Failures-and-Manual-Repair.md) - 处理首次学习中断。

**语言：** [English](en/Getting-Started.md)
