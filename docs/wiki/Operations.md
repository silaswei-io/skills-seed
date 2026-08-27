# 运行、恢复与排障

> **适用场景：** 同步中断、Agent 调用失败、结果异常，或你需要决定是查看归档、续跑还是明确重来。

## 先分辨失败位置

| 现象 | 首先检查 | 常见处理 |
|---|---|---|
| 文件范围不符合预期 | `preview files --mode incremental`、exclude 配置 | 修正范围或 Git ignore 设置 |
| 议程或源码分析失败 | runtime 输入、渲染 prompt、Agent 输出归档 | 修正 Agent、模型、Schema 或输入边界后续跑 |
| 独立审查失败 | 当前焦点的候选与证据归档 | 确认返回是否违反结构或证据范围 |
| 生成失败 | 已保存 profile、rules、patterns 与模板 | 修正数据或模板后重新生成 |
| 命令状态不兼容 | runtime / command state 版本诊断 | 按报错显式重来，不应静默清除状态 |

## Runtime 归档

`.skills-seed/runtime/` 保存可诊断但不应直接编辑的运行工件，通常包括：

- 渲染后的 prompt 与 manifest。
- Agent 原始输出、最终内容、stderr 与调用 manifest。
- 候选文件、焦点输入、恢复状态与 checkpoint。
- 已生成 Skills 的版本、模板、知识快照和文件哈希清单。
- 执行日志与错误诊断。

错误信息中的 `raw`、`stderr`、`manifest` 路径是第一手证据。优先阅读其中的具体上游错误，而不是仅根据顶层“Agent 调用失败”猜测原因。

## 续跑与重来

学习过程会持久化可恢复状态。正常中断、可恢复 Agent 失败或本地保存失败后，优先使用：

```bash
skills-seed sync --resume
```

续跑应复用已经完成的过滤、议程和焦点 checkpoint。仅在状态版本不兼容、输入边界已不再可信或用户明确希望重新开始时，再选择重来。系统应显式报告不兼容原因，不应静默删除全部状态。

续跑时会重新校验已保存的 Pattern 规范化决策；若决策把不同能力入口合并，系统会保留独立候选并替换该 checkpoint。此类修复不需要 `--restart`。

如果中断发生在模式入库之后，续跑可能显示“待分析 0”。这是正常状态：系统会继续完成源码基线、权威规则和项目地图刷新，并在最终学习摘要中保留已提交的模式数量和原始变更范围。

## Agent 与结构化输出问题

检查顺序：

1. 确认配置的 Agent CLI 和模型在当前环境可用。
2. 检查原始输出中的 HTTP、限流、认证、代理或 Schema 错误。
3. 对可重试的限流、过载错误，等待重试或调整重试配置。
4. 对输入或 JSON Schema 校验错误，修复调用方或输出契约；重试同一请求通常没有意义。
5. 需要切换 Agent 时，明确区分分析 Agent 和 Skills 输出目标。

## 日常观察命令

```bash
skills-seed preview files --mode incremental
skills-seed profile show
skills-seed patterns stats
skills-seed log
```

`skills-seed log` 读取 `.skills-seed/store/journal/` 中的运行记录；在工作区根目录下会合并子项目记录后展示。

完整的人工修正决策见 [学习失败与人工修正](Learning-Failures-and-Manual-Repair.md)；诊断命令与参数见 [命令参考](../COMMANDS.md)，配置细节见 [配置参考](../CONFIGURATION.md)。

---

**下一步：** [学习失败与人工修正](Learning-Failures-and-Manual-Repair.md) - 根据错误类别保留证据、续跑或明确重来。

**相关主题：** [配置与参考](Reference.md) - 查找运行时目录和配置字段；[命令工作台](Command-Workbench.md) - 选择诊断命令。

**语言：** [English](en/Operations.md)
