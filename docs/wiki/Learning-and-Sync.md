# 学习与同步

> **适用场景：** 你想更新已生成的 Skills，理解学习阶段，或决定何时只学习、只生成、续跑或重来。

## 选择正确命令

| 目标 | 命令 | 是否生成 Skills |
|---|---|---|
| 日常更新项目知识与产物 | `skills-seed sync` | 是，学习成功且需要刷新时生成 |
| 只学习，不改动生成目录 | `skills-seed learn current` | 否 |
| 已有学习数据，只重新渲染产物 | `skills-seed generate skills` | 是 |
| 预览哪些文件会参与学习 | `skills-seed preview files --mode incremental` | 否 |

完整参数见 [命令参考](../COMMANDS.md)。

## 当前代码学习链路

`learn current` / `sync` 的关键阶段如下：

```text
准备项目上下文
  -> 本地过滤与增量候选
  -> 学习议程规划
  -> 隔离源码证据分析
  -> 独立知识审查
  -> 知识准入与入库
  -> 权威规则与项目地图刷新
  -> 生成 Skills
```

本地代码负责路径范围、输入覆盖、结构化契约、来源安全和持久化。Agent 负责基于已给证据判断语义、边界和可复用价值。两者各自承担擅长的部分，避免把业务判断硬编码到本地过滤器中。

## 焦点、并发与审查

- 每个焦点围绕一个有证据的责任或行为边界分析。
- 源码分析可按配置并行处理独立焦点；并发数由 `agent.parallelism` 控制。
- 独立知识审查按完整焦点进行，避免按候选数量机械切分而丢失上下文。
- 每个完成焦点会保存可恢复状态；所有焦点通过后再进行跨焦点规范化和入库。

并发提高吞吐，但会增加 Agent 调用压力。应按服务限额和项目规模设置，而不是盲目提高数值。

## 成功与失败边界

学习成功不代表 Agent 自动证明所有代码都正确。它表示本轮可接受的知识已满足来源、范围、证据和结构校验。生成阶段只消费已保存结果，不重新调用 Agent。

遇到 Agent 调用失败、Schema 不兼容或输入覆盖校验失败时，系统不会把半完成结论直接写入模式库。先查看 [学习失败与人工修正](Learning-Failures-and-Manual-Repair.md) 中的 runtime 归档和续跑策略。

## 何时强制重新生成

以下情况通常应执行 `skills-seed generate skills`：

- 新增、更新或删除 Rule / Workflow 后。
- 手动维护 pattern 或 profile 后。
- 切换 Skills 目标或输出路径后。
- 需要验证生成模板变化后的交付内容时。

生成会重建 Skills Seed 管理的输出目录。对产物的手工修改应迁移回 `.skills-seed` 中相应的事实源。

---

**下一步：** [命令工作台](Command-Workbench.md) - 按目标选择学习、生成、检查或恢复命令。

**相关主题：** [Skills 与资源](Skills-and-Resources.md) - 评审生成交付；[学习失败与人工修正](Learning-Failures-and-Manual-Repair.md) - 处理失败、续跑或质量异常。

**语言：** [English](en/Learning-and-Sync.md)
