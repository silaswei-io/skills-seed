# 学习失败与人工修正

> **适用场景：** 学习过程报错、中断、续跑结果不符合预期，或生成 Skill 的知识质量需要人为修正。先保留证据、定位失败类别，再选择续跑、修正事实源或明确重来。

## 决策路径

```text
出现异常或质量问题
  -> 文件范围不对？先 preview files
  -> 调用失败或结构校验失败？阅读 runtime 归档
  -> 已保存知识不准确？用 patterns / rule / workflow 修正事实源
  -> 已完成 checkpoint 仍可信？sync --resume
  -> 状态不兼容或输入已失真？sync --restart
  -> 整个初始化状态不可用？确认备份后 reset
```

## 文件范围、输入与焦点问题

学习前或发现遗漏时，先验证工具实际看到的文件，而不是要求 Agent 猜测：

```bash
skills-seed preview files --mode full
skills-seed preview files --mode incremental
skills-seed preview files --mode incremental --focus <path>
```

检查 `exclude.paths`、`exclude.gitignore`、focus 路径与 Git 工作区状态。修正范围后再次预览；范围正确再启动 `learn current` 或 `sync`。不要用无关 Context 去弥补一个本应进入分析的源码文件。

## Agent、网络与结构化输出失败

错误信息中的 `raw`、`stderr`、`manifest` 会指向 `.skills-seed/runtime/agent-outputs/` 下的具体归档。按以下顺序处理：

1. 阅读原始错误，区分认证、限流、代理、服务过载、CLI 调用失败与结构化输出失败。
2. 确认当前配置的 Agent CLI、模型与网络代理在本机可用。
3. 对限流、短暂服务故障等可恢复原因，保留 checkpoint 后使用 `sync --resume`。
4. 对 JSON Schema、输入覆盖或来源范围校验失败，先修复对应的调用、提示词、配置或版本不兼容原因；重复同一输入通常无效。
5. 修复后使用 `sync --resume`，并检查是否复用了已经完成的焦点。

运行归档用于诊断，不能作为人工知识编辑入口。需要在工具本身修复提示词或结构契约时，应保留原始归档作为回归用例。

## 已保存模式质量异常

学习成功不表示每条模式都适合长期保留。发现以下问题时，不必重跑全部学习：

| 问题 | 人工修正 |
|---|---|
| 模式缺少关键用户事实 | `patterns update <id>`，或新增 Rule / Workflow |
| 模式把当前实现说成保证 | `patterns update <id>`，收窄为已证实事实并标出未知边界 |
| 模式完全错误或已废弃 | `patterns delete <id>` |
| 多条模式相近 | 先 `patterns compact --dry-run`，确认后执行整理 |
| 规则或工作流投影缺失 | 用 `rule show` / `workflow show` 检查事实源，再 `generate skills` |

修正模式、Rule 或 Workflow 后，始终执行 `skills-seed generate skills`。详细维护过程见 [知识维护](Knowledge-Maintenance.md)。

## 续跑、重来与重置

| 情况 | 操作 | 原因 |
|---|---|---|
| 正常中断、临时 Agent 失败、本地保存后失败 | `skills-seed sync --resume` | 复用有效 checkpoint，避免重复调用 |
| checkpoint 保存后目标仓库源码发生变化 | `skills-seed sync --resume` | 自动判定旧状态失效并重新规划；`--resume` 不会强制复用不匹配的议程 |
| 续跑时已保存的规范化决策合并了不同能力入口 | `skills-seed sync --resume` | 自动保留独立候选并替换非法 checkpoint |
| 增量分析报告遗漏某个证据焦点 | 升级后执行 `skills-seed sync --resume` | 复用有效议程与已完成焦点；运行时 Schema 要求逐焦点回执，本地准入拒绝候选时保留 `no_change` 决策 |
| 命令状态版本不兼容、候选文件或上下文已不可信 | `skills-seed sync --restart` | 显式清理本次恢复状态，再重新分析 |
| 需要丢弃全部已学习知识及用户 Rule、Workflow | `skills-seed reset all` → `skills-seed sync` | 选中资源会先移入备份；配置和 Context 保留 |
| `.skills-seed` 初始化状态整体不可用或需要切换模式 | `skills-seed reset ...` | 先备份旧状态，再重新初始化 |

不要把 `--restart` 当作自动重试，也不要为了一条 pattern 的表述问题执行 `reset all`。前者只针对当前 sync 计划；选择性 reset 会备份并移除所选知识，完整 reset 会备份并重建整个初始化状态。单条模式优先使用 `patterns update`。

## 生成后仍有问题

生成失败或输出不符合预期时，按来源逆向检查：

1. `profile show`、`patterns show`、`rule show`、`workflow show` 是否已有正确事实。
2. 生成目录中的入口、references、rules 与 workflows 是否对应这些事实。
3. 输入正确而投影错误时，修复生成模板或实现后执行 `generate skills`。
4. 生成目录中的手工编辑不属于修正方案，因为完整生成会覆盖它。

所有命令选择见 [命令工作台](Command-Workbench.md)；runtime 结构与配置位置见 [运行、恢复与排障](Operations.md)。

---

**下一步：** [知识维护](Knowledge-Maintenance.md) - 修正已保存 Pattern、Rule、Workflow 或项目画像。

**相关主题：** [运行、恢复与排障](Operations.md) - 查看 runtime 归档；[命令工作台](Command-Workbench.md) - 选择 `resume`、`restart` 或 `reset`。

**语言：** [English](en/Learning-Failures-and-Manual-Repair.md)
