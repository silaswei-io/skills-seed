# 知识维护

> **适用场景：** 你发现已学习模式不准确、不完整或重复，或需要维护 Rule、Workflow 与项目画像。目标是修正事实源，再重新生成，而不是直接编辑生成 Skill。

## 先判断应维护什么

| 发现的问题 | 应维护的对象 | 不应做的事 |
|---|---|---|
| 源码已变化，旧结论失效 | 重新学习：`learn current` 或 `sync` | 用用户模式掩盖源码事实 |
| 一条模式表述错误、范围太大或缺少边界 | `patterns update` | 编辑生成目录中的 reference |
| 一条模式不应存在 | `patterns delete` | 只在生成目录删文件 |
| 多条模式语义重复 | `patterns compact --dry-run` 后执行整理 | 直接编辑数据库或 runtime |
| 团队明确的长期禁止项或契约 | `rule` | 假设源码能自动推导 |
| 发布、验收、部署等任务步骤 | `workflow` | 让源码学习把过程推断为事实 |
| 项目地图或权威文件已变化 | `learn current --profile refresh` | 手改 profile JSON |

## 重新学习前重置知识

当错误或过时的知识已成体系，不适合逐条修订时，使用 `reset` 清理对应事实源后重新学习。选择性重置不会删除配置、长期 Context 或已生成 Skills；实际移除的资源会先进入 `.skills-seed.backup/<timestamp>/knowledge/`。

| 目标 | 命令 | 后续动作 |
|---|---|---|
| 从干净知识状态完整重新学习 | `skills-seed reset all` | `skills-seed sync` |
| 仅重学源码模式、画像和分析状态 | `skills-seed reset patterns` | `skills-seed sync` |
| 仅移除团队 Rule | `skills-seed reset rules` | 按需要重新添加 `rule`，再 `generate skills` |
| 仅移除任务 Workflow | `skills-seed reset workflows` | 按需要重新添加 `workflow`，再 `generate skills` |

`patterns` 会连带清理项目画像、文件快照、可恢复 checkpoint 和学习历史，确保下一次 `sync` 不会复用旧的分析结果。范围可组合，例如 `skills-seed reset patterns rules`；`all` 必须单独使用，且选择性重置不能与 `--mode`、`--workspace`、`--locale` 或 `--skills-locale` 同时使用。

只需要修订一条模式时，优先使用 `patterns update`；只需要继续被中断的学习时，优先使用 `sync --resume`。完整 `skills-seed reset`（不带范围）用于重新初始化或切换 project/workspace 模式。

`patterns update` 是项目范围命令，必须在已初始化的项目或工作区目录中执行；如果只需要查看帮助，可直接使用 `skills-seed patterns update --help`，不会打开项目运行时。

## 模式的人工维护闭环

1. 用 `skills-seed patterns stats` 查看数量、分类和质量指标。
2. 用 `skills-seed patterns show --sort score` 定位重点，再用 `patterns show <id> --format json` 阅读完整证据、范围和表述。
3. 根据问题选择 `add`、`update`、`delete` 或 `compact --dry-run`。
4. 若整理预览符合预期，执行同一条 `compact` 命令但移除 `--dry-run`。
5. 运行 `skills-seed generate skills`，检查入口路由和对应 reference 投影。

示例：修订一条过度推断的模式，应明确写出保留的事实、需要收窄的范围与未知项：

```bash
skills-seed patterns update <pattern-id> \
  --content "保留已证实的实现事实；将适用范围限制为有证据的位置；未知保证必须标为待源码确认。"
skills-seed generate skills
```

`patterns add` 与 `patterns update` 会调用 Agent 整理自然语言，因此写入后必须复查结果。`patterns compact` 不调用 Agent，适合先用 `--dry-run` 审核确定性整理结果。

## 规则与工作流的维护闭环

Rule 是权威约束，Workflow 是人为定义的任务过程；两者都以 `.skills-seed` 中的文件为事实源，并在生成时投影到 Skill。

```bash
# 新增或补充长期规则；同名时默认保留已有内容并增量优化。
skills-seed rule --name <rule-name> --content "<明确约束>"

# 新增或补充任务过程；同名时默认合并并保留待确认冲突。
skills-seed workflow --name <workflow-name> --content "<明确步骤>"

# 阅读实际保存的内容，再生成投影。
skills-seed rule show <rule-id> --format json
skills-seed workflow show <workflow-id> --format json
skills-seed generate skills
```

只有确实要废弃旧正文和旧范围时才使用 `--overwrite`。在 workspace 根目录修改 Rule 时，应显式给出 `--child`、`--project` 或 `--path`，且只覆盖真实受影响的子项目或根路径。范围不明确时先补充说明，不要覆盖或扩大规则。

## 项目画像与长期背景

`profile show` 用于检查当前项目画像。权威来源、结构或模块关系已经改变时，刷新画像：

```bash
skills-seed profile show
skills-seed learn current --profile refresh
skills-seed generate skills
```

代码看不到但长期存在的业务背景、术语或外部系统信息可维护在 `.skills-seed/context/`。一次性说明应使用 `sync --context` 或 `--context-path`，不应误写入长期 Context、Rule 或 Pattern。

## 不可直接编辑的内容

- `.skills-seed/runtime/`：仅保存诊断、checkpoint、Agent 输出和生成清单；删改会破坏恢复或审计线索。
- `.skills-seed/store/` 的数据库与画像文件：通过 CLI 维护，避免破坏一致性和来源记录。
- `.agents/skills/`、`.claude/skills/` 等生成输出目录：下次 `generate skills` 会完整重建。

若需要处理失败状态、Agent 输出或恢复计划，转到 [学习失败与人工修正](Learning-Failures-and-Manual-Repair.md)。所有子命令与参数见 [命令工作台](Command-Workbench.md) 和 [命令参考](../COMMANDS.md)。

---

**下一步：** [学习失败与人工修正](Learning-Failures-and-Manual-Repair.md) - 处理运行归档、checkpoint 与不可恢复状态。

**相关主题：** [Rule 与 Workflow](Rules-and-Workflows.md) - 理解两类用户资源的边界；[命令工作台](Command-Workbench.md) - 查找维护子命令。

**语言：** [English](en/Knowledge-Maintenance.md)
