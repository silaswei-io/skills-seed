# Rule 与 Workflow

Rule 和 Workflow 解决的是源码学习无法可靠推导的问题：团队的强制边界，以及测试、发布、验收等任务的明确过程。

> **适用场景：** 你需要保存长期约束、团队任务过程，或为当前学习提供不会持久化的一次性说明。

## Context：背景与一次性说明

`.skills-seed/context/` 用于项目背景、术语和代码看不到的长期事实。长期强制约束应使用 Rule，不应隐藏在 Context 中。

仅影响当前学习任务的说明使用 `sync --context` 或 `sync --context-path`；它们不会写入长期 Context，也不会成为生成 Skill 的永久规则。Context 文件布局和输入处理细节以 [配置参考](Reference.md) 链接的权威文档为准。

## Rule：长期约束

Rule 适合记录以下内容：

- 可修改与不可修改的范围。
- 接口、兼容性、安全和数据约束。
- 命令权限和需要人工确认的操作。
- 项目术语、底座边界或长期治理要求。

添加或增量优化 Rule：

```bash
skills-seed rule --name <rule-name> --content "<团队约束>"
skills-seed generate skills
```

同名 Rule 默认在现有内容基础上优化；只有显式 `--overwrite` 才应完整替换。Rule 的事实源是 `.skills-seed/rules/<id>/RULE.md`。

## Workflow：任务过程

Workflow 适合记录测试、部署、发布、验收、排障交接等步骤、命令、顺序和负责人要求：

```bash
skills-seed workflow --name <workflow-name> --content "<任务步骤>"
skills-seed generate skills
```

源码可以描述实现依赖和影响边界，但不能可靠推导“应执行哪些命令、谁批准、何时部署”。这些属于用户维护的 Workflow。

## 路由元数据

规则和工作流优化时可能生成 `summary` 与 `route_terms`。`summary` 是简短用途说明，`route_terms` 是从用户输入中提取的少量需求信号，用来提高入口 Skill 的命中率。它们不是新的规则、步骤、范围或命令权限：规则仍按显式项目/路径范围判断，工作流仍需打开完整正文后才能执行。旧资源没有这些字段时，生成流程会继续使用正文摘要并保持兼容。

## 工作流路径基准

工作流正文中的改动位置、验证目标和脚本引用都使用仓库相对路径：workspace 根工作流相对于 workspace 根目录；项目工作流以及使用 `--child` 的工作流相对于目标项目根目录。不要使用当前终端目录的相对路径或本机绝对路径。

## 工作流关联脚本

每个工作流使用自己的目录保存正文和脚本：

```text
.skills-seed/workflows/<workflow-id>/
├── WORKFLOW.md
└── scripts/
    └── <workflow-specific-script>
```

工作流需要脚本时，只能放入其 `scripts/` 目录；先用 `skills-seed workflow show <workflow-id>` 确认稳定 ID。执行 `skills-seed generate skills` 后，正文会生成到 `workflows/<workflow-id>.md`，脚本会复制到 `scripts/workflows/<workflow-id>/`，并由生成的工作流引用。不要把脚本直接写入生成目录。

## 命令策略

生成 Skill 应清楚区分：

| 策略 | 含义 |
|---|---|
| `forbidden` | 无论是否请求，都不应执行 |
| `describe_only` | 可以说明，不应由 Agent 执行 |
| `requires_authorization` | 需要明确授权后才能执行 |
| `allowed` | 在当前任务范围内可执行 |

自动化、CI 或构建文件本身不能自动授予命令权限；只有明确 Rule 或用户指令可以定义该边界。

## 修改后的检查

1. 用 `skills-seed rule` 或 `workflow` 查看保存结果。
2. 执行 `skills-seed generate skills` 刷新投影。
3. 打开生成 Skill 的 `rules/` 或 `workflows/`，确认内容和入口链接正确。
4. 对影响安全、部署或兼容性的 Rule，在真实任务中确认 Agent 遵循了边界。

完整的维护闭环见 [知识维护](Knowledge-Maintenance.md)，命令选择见 [命令工作台](Command-Workbench.md)，精确参数见 [命令参考](../COMMANDS.md)。

---

**下一步：** [知识维护](Knowledge-Maintenance.md) - 查看、修订并重新生成 Rule 与 Workflow 投影。

**相关主题：** [Workspace](Workspace.md) - 定义根规则的明确影响范围；[命令工作台](Command-Workbench.md) - 查找具体子命令。

**语言：** [English](en/Rules-and-Workflows.md)
