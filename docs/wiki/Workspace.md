# Workspace

Workspace 模式适用于一个根目录下管理多个独立 Git 子项目的情况。它不把所有代码混成一个项目，而是保留事实归属：子项目学习自己的代码，根目录负责路由、共享约束和跨项目影响关系。

> **适用场景：** 你的根目录包含多个独立 Git 子项目，且需求需要先判断项目归属和跨项目影响。

## 何时使用

适合：

- 根目录包含多个可独立开发、测试或发布的 Git 子项目。
- 一项需求可能涉及多个项目，需要先判断归属与变更顺序。
- 团队希望每个子项目保留独立的 `.skills-seed` 和最终 Skill。

不适合：只有一个 Git 项目但目录很多的普通单仓；此时应使用项目模式，并通过模块画像和 references 路由。

## 初始化与添加项目

在 workspace 根目录初始化后，登记子项目：

```bash
skills-seed init
skills-seed workspace add <child-path>
skills-seed sync
```

实际交互和参数以 [命令参考](../COMMANDS.md) 为准。

## 责任划分

| 位置 | 负责内容 |
|---|---|
| Workspace 根 | 子项目路由、跨项目关系、根级 Rule 与影响说明 |
| 子项目 | 本项目源码学习、patterns、profile 和生成 Skill |
| 根级 Rule | 至少明确影响多个子项目，或影响工作区根路径 |
| 单项目 Rule | 仅归该子项目维护，不应错误提升到根级 |

根级 Rule 不会自动成为所有子项目的规则。资源归属必须由用户明确声明，系统不会根据自然语言猜测并扩大作用范围。

## 运行策略

- 根配置的 `agent.parallelism` 控制子项目并发。
- 每个子项目内部的焦点分析并发由其自身配置控制。
- 跨项目任务先由根 Skill 识别受影响项目，再进入子项目 Skill 与源码。
- 中断后优先恢复已有计划与 checkpoint，避免重新扫描无变化子项目。

配置项、恢复策略与根目录状态说明见 [配置与参考](Reference.md) 和 [运行、恢复与排障](Operations.md)。

---

**下一步：** [运行、恢复与排障](Operations.md) - 查看运行归档、状态与日常诊断入口。

**相关主题：** [Rule 与 Workflow](Rules-and-Workflows.md) - 定义子项目或根路径范围；[命令工作台](Command-Workbench.md) - 选择 workspace 子命令。

**语言：** [English](en/Workspace.md)
