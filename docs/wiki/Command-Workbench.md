# 命令工作台

> **适用场景：** 你已知道要达成的结果，但需要选择正确的主命令和子命令。精确参数、默认值与完整示例始终以 [命令参考](../COMMANDS.md) 为准。

## 先按目标选择

| 目标 | 首选命令 | 是否调用 Agent | 是否修改长期状态 |
|---|---|---:|---:|
| 首次接入一个项目 | `init` → `sync` | `sync` 是 | 是 |
| 查看本轮会分析什么 | `preview files` | 否 | 否 |
| 只更新学习知识 | `learn current` | 是 | 是 |
| 更新知识并生成 Skills | `sync` | 是 | 是 |
| 只重建生成目录 | `generate skills` | 否 | 是 |
| 查看或修正知识 | `patterns ...`、`profile show` | 视子命令而定 | 视子命令而定 |
| 维护团队约束或任务过程 | `rule ...`、`workflow ...` | 写入时是 | 是 |
| 恢复或观察运行 | `sync --resume`、`log`、`hook ...` | 视命令而定 | 视命令而定 |

## 初始化、学习与生成

| 命令 | 何时使用 | 常用形式 | 后续动作 |
|---|---|---|---|
| `skills-seed` / `help` | 查询版本或命令帮助 | `skills-seed help learn current` | 按帮助确认参数 |
| `init` | 首次初始化单项目或工作区根 | `skills-seed init --mode project` | 执行 `sync` |
| `workspace add` | 为 workspace 登记子项目 | `skills-seed workspace add <path>` | 在根目录执行 `sync` |
| `preview files` | 学习前确认文件范围 | `skills-seed preview files --mode incremental --focus <path>` | 修正过滤或继续学习 |
| `learn current` | 只学习源码，不替换生成 Skill | `skills-seed learn current --focus <path>` | 检查 pattern/profile，再按需生成 |
| `sync` | 日常完整更新 | `skills-seed sync` | 查看生成目录与 `log` |
| `generate skills` | 已修改知识资源，只需重新投影 | `skills-seed generate skills` | 评审生成目录 |

`--context` 和 `--context-path` 只补充当前一次学习的背景，不会保存为 Rule、Workflow 或 Pattern。需要长期生效的约束或过程，应分别使用 Rule 和 Workflow。

## 知识检查与修正

| 命令 | 何时使用 | 常用形式 | 影响 |
|---|---|---|---|
| `patterns show` | 查看全部摘要或一条完整模式 | `skills-seed patterns show <id> --format json` | 只读 |
| `patterns stats` | 判断质量、分类或分数分布 | `skills-seed patterns stats` | 只读 |
| `patterns add` | 补充源码无法得出的用户模式 | `skills-seed patterns add --content "<说明>"` | Agent 整理并写入 |
| `patterns update` | 修订已有模式，保留其 ID | `skills-seed patterns update <id> --content "<修正>"` | Agent 整理并写入 |
| `patterns delete` | 删除错误、失效或不应沉淀的模式 | `skills-seed patterns delete <id>` | 删除已保存模式 |
| `patterns compact` | 合并语义相近的模式 | `skills-seed patterns compact --dry-run` | `--dry-run` 只预览 |
| `profile show` | 查看项目画像与项目地图摘要 | `skills-seed profile show` | 只读 |

修正后执行 `skills-seed generate skills`。如果质量问题来自当前源码变化，而不是模式正文，应通过 `learn current` 或 `sync` 重新学习，而不是用手工 Pattern 覆盖事实。

## Rule 与 Workflow

| 命令 | 何时使用 | 常用形式 | 关键边界 |
|---|---|---|---|
| `rule` | 新增或增量优化长期强制约束 | `skills-seed rule --name <name> --content "<约束>"` | 同名默认合并优化；`--overwrite` 才完整替换 |
| `rule show` | 查看规则摘要或全文 | `skills-seed rule show <id> --format json` | 只读 |
| `workflow` | 新增或增量优化任务过程 | `skills-seed workflow --name <name> --content "<过程>"` | 同名默认合并优化；冲突保留待确认项 |
| `workflow show` | 查看工作流摘要或全文 | `skills-seed workflow show <id> --format json` | 只读 |

在 workspace 根目录，Rule 的 `--child`、`--project`、`--path` 必须表达真实、明确的影响范围；不要让工具或 Agent 从描述猜测范围。Rule 或 Workflow 写入后均需执行 `generate skills`，详情见 [知识维护](Knowledge-Maintenance.md)。

## 自动化、恢复与重置

| 命令 | 何时使用 | 常用形式 | 风险边界 |
|---|---|---|---|
| `hook install` | 在提交前提供交互式学习选择 | `skills-seed hook install` | 默认跳过；非交互环境不阻塞 |
| `hook run` | 手动测试 hook 菜单 | `skills-seed hook run` | 只在交互环境有意义 |
| `hook uninstall` | 移除当前项目 hook | `skills-seed hook uninstall` | 不删除学习数据 |
| `cli-skills install` | 安装全局 CLI 操作 Skill | `skills-seed cli-skills install --target auto` | 不管理项目生成 Skill |
| `cli-skills uninstall` | 卸载该全局 CLI 操作 Skill | `skills-seed cli-skills uninstall --target codex` | 只删除固定全局目标 |
| `log` | 查看最近学习与生成变更 | `skills-seed log` | 只读摘要，不是详细诊断日志 |
| `sync --resume` | 从可恢复 checkpoint 继续 | `skills-seed sync --resume` | 优先于重新开始 |
| `sync --restart` | 明确丢弃本次恢复状态并重来 | `skills-seed sync --restart` | 仅用于状态不兼容或输入已失真 |
| `reset` | 备份后重置整个初始化状态 | `skills-seed reset --mode project` | 会移动旧 `.skills-seed` 至备份目录 |

`reset` 是重新初始化，不是普通故障恢复手段。学习中断或单个模式质量异常时，先看 [学习失败与人工修正](Learning-Failures-and-Manual-Repair.md)。

---

**下一步：** [知识维护](Knowledge-Maintenance.md) - 按 Pattern、Rule、Workflow 和画像的事实源修正知识。

**相关主题：** [学习与同步](Learning-and-Sync.md) - 理解学习阶段；[学习失败与人工修正](Learning-Failures-and-Manual-Repair.md) - 选择续跑、重来或 reset。

**语言：** [English](en/Command-Workbench.md)
