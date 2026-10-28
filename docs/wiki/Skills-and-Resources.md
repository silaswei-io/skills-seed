# Skills 与资源

默认 `sync` / `learn current` 先用 CodeGraph 本地扫描（优先外置，缺失时用内置版），再按已有知识证据变化和本地调用关系选择关键语义缺口。候选按重要性顺序和证据容量动态组批，沿用独立审核和检查点恢复。AI 不可用时仍可生成本地导航，并明确记录待补充项。也可用 `learn current --semantic --focus <path>` 主动深挖。详见[学习与同步](Learning-and-Sync.md)。

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
│   ├── business-methods/        # 每个能力入口的详情
│   ├── code-navigation.md       # CodeGraph 查询入口
│   └── patterns/
│       ├── business.md          # 业务规则索引
│       ├── business/groups/      # 从已审查标题归纳的责任主题索引
│       └── business/items/      # 每条业务规则的详情
├── rules/
└── workflows/
```

实际目录会随项目模式、目标 Agent、已有知识和用户资源变化。

业务地图和能力入口文件仅负责路由。已审查规则按标题中的共同责任主题拆成轻量索引；每条规则仍有独立详情和证据，独立责任直接从地图进入详情。CodeGraph 的结构事实保存在 `.codegraph`，不再复制公开符号目录到 Skill。`code-navigation.md` 提供本地查询路径；经 AI 独立审查并通过本地符号校验的语义能力进入 `business-methods.md`，补上需求匹配、前置条件和结果/失败语义。Agent 只需加载命中的内容。

## 入口 Skill 应回答什么

`SKILL.md` 的任务不是复述所有知识，而是在改动前帮助 Agent 判断：

1. 当前任务属于哪个项目、模块或已审查的业务责任。
2. 是否先受显式 Rule 或 Workflow 约束。
3. 需要读取哪些项目参考、能力入口和源码位置。
4. 多条业务模式可能适用时，如何沿证据路径确认主责任，并在有明确关联时扩展读取。
5. 哪些影响边界或未知项应在实施前确认。

命中业务模式后只读取对应详情和证据入口；只有目标页明确给出跨边界关联时，才继续读取其他 reference。通用动词或技术词不能单独决定归属，源码仍是最终核验依据。

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
- 业务模式是否有自然的责任名称、清楚的适用边界和可定位的证据入口。
- 能力入口是否包含声明、前置条件、结果语义与可定位的来源。
- 模块地图是否描述真实责任与依赖，而非自依赖或猜测路径。
- 生成目录是否只包含有效投影，且链接与计划文件可用。

生成就绪检查会解析真实 Markdown 链接，并在消解符号链接后确认目标仍位于当前 Skill 根目录内。缺失目标或逃逸根目录的链接会阻止交付；泛型、索引和调用等合法源码表达式不会被当成本地链接。

质量测试与开发维护见 [质量与贡献](Quality-and-Contributing.md)。

---

**下一步：** [Rule 与 Workflow](Rules-and-Workflows.md) - 为生成 Skill 补充明确约束和任务过程。

**相关主题：** [知识维护](Knowledge-Maintenance.md) - 修正已保存知识；[质量与贡献](Quality-and-Contributing.md) - 以真实改动判断交付价值。

**语言：** [English](en/Skills-and-Resources.md)
