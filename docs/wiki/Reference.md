# 配置与参考

本 Wiki 负责解释工作方式；以下文档是字段、参数与模板的权威参考。需要精确命令或默认值时，优先阅读它们。

> **适用场景：** 你已知道要做什么，但需要查找精确命令、参数、字段、默认值或维护者资料。

## 使用者参考

| 主题 | 文档 | 何时阅读 |
|---|---|---|
| 所有命令、子命令、参数与示例 | [命令参考](../COMMANDS.md) | 运行自动化、查找参数、排查命令行为 |
| 按目标选择命令与子命令 | [命令工作台](Command-Workbench.md) | 初始化、学习、修正、恢复或重置前 |
| 配置结构、默认值、目录与运行时调试 | [配置参考](../CONFIGURATION.md) | 调整 Agent、并发、过滤、输出与日志 |
| 人工维护 Pattern、Rule、Workflow 与画像 | [知识维护](Knowledge-Maintenance.md) | 已保存知识需要补充、修订、删除或整理 |
| 学习失败、续跑、重来与运行归档 | [学习失败与人工修正](Learning-Failures-and-Manual-Repair.md) | 失败诊断或质量异常修正 |
| 真实项目的生成效果 | [Medusa Demo 案例](../MEDUSA_DEMO_CASE.md) | 评估生成产物和落地方式 |
| 版本变化 | [更新日志](../../CHANGELOG.md) | 升级、回归或比较行为差异 |

## 维护者参考

| 主题 | 文档 | 说明 |
|---|---|---|
| 产品验收边界 | [最终目标](../ULTIMATE_GOAL.md) | 所有提示词、数据流和模板改动的最终依据 |
| Prompt 清单与冗余审计 | [Prompt 文档](../PROMPTS.md) | 检查运行时提示词责任与重复内容 |
| 贡献与本地检查 | [贡献指南](../../CONTRIBUTING.md) | 代码、测试和提交约定 |

## 配置决策速查

| 需求 | 主要配置或命令 |
|---|---|
| 使用特定分析 Agent 或模型 | `agent.engine`、`agent.model` |
| 调整分析吞吐 | `agent.parallelism` |
| 排除不应学习的路径 | `exclude.paths`、`exclude.gitignore` |
| 切换 Skills 消费目标或输出位置 | `skills.target`、`skills.paths.<target>` |
| 只预览或只学习 | `preview files`、`learn current` |
| 强制重新渲染 | `generate skills` |

字段名、可选值和完整行为以 [配置参考](../CONFIGURATION.md) 为准。

---

**下一步：** [质量与贡献](Quality-and-Contributing.md) - 用最终目标和质量门禁评审变更。

**相关主题：** [命令工作台](Command-Workbench.md) - 先按目标选择命令；[知识维护](Knowledge-Maintenance.md) - 维护已保存的知识。

**语言：** [English](en/Reference.md)
