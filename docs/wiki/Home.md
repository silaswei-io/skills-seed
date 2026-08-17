# Skills Seed

Skills Seed 将明确约束、源码证据和项目导航组织为本地 Skills，使 Agent 在改动前理解规则、归属、可复用能力和影响边界。

> **从未使用过？** 在 Git 项目根目录执行 `skills-seed init`，再执行 `skills-seed sync`。完整前置条件和验证步骤见 [快速开始](Getting-Started.md)。

## 选择你的路径

### 第一次接入

1. 阅读 [快速开始](Getting-Started.md)，完成初始化与首次生成。
2. 阅读 [核心概念](Concepts.md)，区分 Rule、Workflow、Context 与源码学习知识。
3. 打开生成的 `SKILL.md`，确认入口能路由到项目参考和用户资源。

### 日常使用

1. 在项目根目录执行 `skills-seed sync` 更新知识与产物。
2. 需要理解阶段、并发、入库或重新生成时，阅读 [学习与同步](Learning-and-Sync.md)。
3. 需要选择命令或子命令时，阅读 [命令工作台](Command-Workbench.md)。
4. 需要维护团队资源时，阅读 [Rule 与 Workflow](Rules-and-Workflows.md) 和 [知识维护](Knowledge-Maintenance.md)。

### 团队维护与排障

- 多个独立子项目：阅读 [Workspace](Workspace.md)。
- 同步中断、输出异常或需人工修正：阅读 [学习失败与人工修正](Learning-Failures-and-Manual-Repair.md)。
- 查找精确参数、字段和默认值：阅读 [配置与参考](Reference.md)。
- 评审生成质量或参与开发：阅读 [质量与贡献](Quality-and-Contributing.md)。

## 工作闭环

```text
用户维护的 Rule / Workflow / Context
                  +
当前源码、结构与变更证据
                  |
                  v
          学习并保存项目知识
                  |
                  v
       生成模块化、按需加载的 Skills
                  |
                  v
Agent 在改动前读取入口并按任务加载参考
```

## 你得到的不是

| 不是 | 而是 |
|---|---|
| 一份静态、超长的项目说明 | 模块化、按任务加载的本地工作上下文 |
| 由模型猜测出的团队规范 | 有明确来源的 Rule 与源码证据化知识 |
| 远端不可审计记忆 | 可审阅、可更新、可提交的项目资源 |

## 设计原则

- **明确规则优先**：用户维护的 Rule 定义强制边界；源码只能提供可复用实践和导航证据。
- **证据优先于概括**：低频样例、猜测和常识不应被提升为项目规范。
- **按需加载**：入口 Skill 保持简洁，详细项目知识位于 references、rules 与 workflows。
- **本地可审阅**：项目状态、运行归档和生成产物均存于项目本地。
- **持续更新**：知识随源码变化被补充、修订、撤销或降级，而非一次生成后永久冻结。

完整产品目标见 [最终目标](../ULTIMATE_GOAL.md)。

---

**从这里开始：** [快速开始](Getting-Started.md)

**语言：** [English](en/Home.md)
