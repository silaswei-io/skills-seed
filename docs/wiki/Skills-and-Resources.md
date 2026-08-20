# Skills 与资源

> **适用场景：** 你需要理解生成目录的职责、评审入口 Skill，或区分 references、rules 与 workflows。

## 生成产物的职责

生成的 Skill 不是单一超长说明文档。入口文件只保留任务判断、优先级和参考路由，详细内容按职责拆分，供 Agent 在需要时读取。

典型结构：

```text
<project>-dev/
├── SKILL.md
├── agents/
├── references/
│   ├── project-overview.md
│   ├── project-spec.md
│   ├── modules.md
│   ├── business-methods.md
│   └── patterns/
├── rules/
└── workflows/
```

实际目录会随项目模式、目标 Agent、已有知识和用户资源变化。

## 入口 Skill 应回答什么

`SKILL.md` 的任务不是复述所有知识，而是在改动前帮助 Agent 判断：

1. 当前任务属于哪个项目、模块或开发焦点。
2. 是否先受显式 Rule 或 Workflow 约束。
3. 需要读取哪些项目参考、能力入口和源码位置。
4. 多个焦点命中时，如何先选主焦点，并在有明确关联时扩展读取。
5. 哪些影响边界或未知项应在实施前确认。

焦点命中后先读取主焦点的首读参考和代表证据入口；只有目标页明确给出跨边界关联时，才继续读取其他 reference。通用动词或技术词不能单独决定焦点。焦点的责任摘要、适用属性和风险信号用于理解边界，不替代源码核验。

Rule 与 Workflow 的 `summary`、`route_terms` 也只用于入口导航；规则范围和工作流权限仍以完整资源正文及显式范围为准。

它不应把“当前代码优先”写成可覆盖用户明确约束的笼统规则，也不应把技术服务名或偶然关键词当作稳定业务路由。

## References、Rules 与 Workflows

| 目录 | 内容 | 维护方式 |
|---|---|---|
| `references/` | 项目画像、模块地图、能力入口、源码模式 | 由学习结果生成 |
| `rules/` | 用户长期约束的投影 | 从 `.skills-seed/rules/` 生成 |
| `workflows/` | 用户维护的任务步骤和关联脚本 | 从 `.skills-seed/workflows/` 生成 |

Agent 使用 references 理解“项目现在如何实现”，使用 rules 理解“哪些边界必须遵守”，使用 workflows 执行“团队要求的任务过程”。三者不能互相替代。

## 如何评审生成质量

评审重点应回到实际开发决策：

- 入口是否能命中需求、排障、契约、配置、边界和验证任务。
- 强制约束是否来自正确的权威来源，且未被代码观察覆盖。
- 业务焦点是否有自然名称、稳定路由词和必要的多命中消歧规则。
- 能力入口是否包含声明、前置条件、结果语义与可定位的来源。
- 模块地图是否描述真实责任与依赖，而非自依赖或猜测路径。
- 生成目录是否只包含有效投影，且链接与计划文件可用。

质量测试与开发维护见 [质量与贡献](Quality-and-Contributing.md)。

---

**下一步：** [Rule 与 Workflow](Rules-and-Workflows.md) - 为生成 Skill 补充明确约束和任务过程。

**相关主题：** [知识维护](Knowledge-Maintenance.md) - 修正已保存知识；[质量与贡献](Quality-and-Contributing.md) - 以真实改动判断交付价值。

**语言：** [English](en/Skills-and-Resources.md)
