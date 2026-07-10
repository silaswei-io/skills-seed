# Medusa Demo 案例

这个案例基于 `medusa-demo` 的 `.claude/skills/skills-seed-skills` 目录，使用默认 `sync` 生成。它展示的重点不是“能生成一份项目说明”，而是把大型 TypeScript monorepo 中分散在代码里的业务约束、模块边界、入口方法和验证命令，整理成 Agent 可以按任务加载的项目级 Skill。

样例地址：[medusa-demo/.claude/skills/skills-seed-skills](https://github.com/silaswei-io/medusa-demo/tree/develop/.claude/skills/skills-seed-skills)

## 生成结果

默认同步后，生成入口是 `SKILL.md`，详细内容拆在 `references/` 下：

```text
.claude/skills/skills-seed-skills/
├── SKILL.md
└── references/
    ├── project-overview.md
    ├── project-spec.md
    ├── business-methods.md
    ├── common-utils.md
    ├── modules.md
    └── patterns/
        ├── api.md
        ├── business.md
        ├── concurrency.md
        ├── config.md
        ├── database.md
        ├── error.md
        ├── middleware.md
        ├── naming.md
        ├── structure.md
        └── utils.md
```

这个入口 Skill 学到了 92 条 patterns，覆盖 10 个类别，平均置信度约 90%。项目画像中识别了 Medusa 的 monorepo 架构、workflow 编排、module service、API route、配置、事件、远程查询和验证命令等项目特征，并把更详细的信息分散到对应 reference 文件中。

## 它相比手写 Skills 的价值

medusa-demo 里同时存在 `writing-docs`、`reviewing-prs`、`triaging-issues`、`writing-releases` 等手写 Skills。这些 Skills 很有用，但它们主要描述固定任务流程，例如如何写 MDX 文档、如何审 PR、如何输出 review decision。它们通常由人维护，关注的是“做某类任务时遵守什么流程”。

Skills Seed 生成的 `skills-seed-skills` 关注的是另一个层面：当前代码库本身的项目语义。它会告诉 Agent：

- 改 API、业务流程、配置或外部依赖时，应该先读哪些项目 reference。
- 哪些规则来自真实代码证据，哪些只是导航性的观察。
- Medusa 中 workflow、step compensation、module service、route loading、DTO 命名、错误处理等约定分别落在哪些文件和模式里。
- 某类改动建议用什么验证命令，例如 API / contract 变更对应 `yarn validate:http-types`，持久化或查询变更对应 `yarn test`。

换句话说，手写 Skills 更像团队预先定义的任务操作手册；Skills Seed 生成的 Skill 更像从代码中抽取出来的项目地图和约束索引。两者不是替代关系，而是互补关系：做文档时仍应加载 `writing-docs`，但当文档或代码变更涉及 Medusa 的真实模块边界、业务状态机、生成产物或验证范围时，`skills-seed-skills` 能提供更贴近当前代码的上下文。

## Agent 实际会怎么用

当 Agent 接到“新增一个 API 字段”这类任务时，入口 `SKILL.md` 会把它路由到 `project-spec.md`、API patterns 和结构 patterns，而不是让 Agent 全量阅读整个仓库。它会看到接口契约、生成产物、转换层和验证命令之间的关系，从而更容易避免只改实现、不改类型或忘记生成校验的情况。

当任务涉及订单、支付、库存、履约等业务流时，入口会引导 Agent 先读 business pattern map，再按领域打开细分 reference。这样 Agent 不需要靠猜测理解状态转换、锁、补偿逻辑和事件发送顺序，而是先查看已从代码中提取的规则和证据。

当任务只涉及工具方法、配置、错误处理或 middleware 时，Agent 只需要读取对应类别的 reference。入口文档刻意保持克制，避免把 90 多条规则一次性塞进上下文。

## 适合展示的核心卖点

这个样例适合用来说明 Skills Seed 的几个核心价值：

| 价值 | 在 medusa-demo 中的体现 |
|---|---|
| 从真实代码学习 | patterns 来自当前 TypeScript monorepo 的模块、workflow、route、config 和测试命令 |
| 按需加载上下文 | `SKILL.md` 负责路由，细节放入 `references/`，Agent 按任务读取 |
| 保留证据和边界 | 规则区分硬约束与参考观察，并提醒当前代码优先 |
| 适配复杂仓库 | 能整理 monorepo 中多模块、跨模块编排、生成产物和验证命令 |
| 与手写 Skills 互补 | 手写 Skills 管流程，Skills Seed 补项目语义和代码约束 |

## 为什么这能体现 Skills Seed 的价值

大型项目最难传给 Agent 的，往往不是 README 里已经写清楚的内容，而是散落在代码、目录、命名、异常分支、测试命令和历史改动里的隐性约定。medusa-demo 的生成结果说明，Skills Seed 可以在默认同步下把这些约定沉淀成可路由、可更新、可协作的本地 Skill。

这让 Agent 在动手改代码前先获得项目级判断：任务属于哪个模块、应该读哪些 reference、哪些规则有证据、哪些命令能验证当前改动。对团队来说，这比每次手动补充上下文更稳定，也比维护一份很长的手写项目说明更容易持续更新。
