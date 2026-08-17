# Skills Seed Wiki 源文件

本目录是 Skills Seed 的仓库内 Wiki。它面向使用者，按任务组织内容；命令参数与配置字段仍以同级 `docs/` 下的参考文档为准。

目录根部的 `Home.md` 与 `_Sidebar.md` 使用 GitHub Wiki 兼容命名。将本目录内容镜像到 GitHub Wiki 仓库即可发布；根 `_Sidebar.md` 已提供中英文完整导航。在仓库内阅读时，从 [首页](Home.md) 开始。

## 维护约定

- [README](../../README.md) 只承担项目介绍、安装、最短上手路径和文档入口，不重复教程、参数或运行时细节。
- 本 Wiki 是唯一的面向使用者的叙事型指南，解释何时执行什么操作、不同资源如何协作，以及出现问题时先看哪里。
- [命令工作台](Command-Workbench.md) 覆盖所有主命令与子命令的选择路径；不复制完整参数表。
- [知识维护](Knowledge-Maintenance.md) 与 [学习失败与人工修正](Learning-Failures-and-Manual-Repair.md) 说明知识质量异常时的检查、修正、续跑与重来边界。
- [命令参考](../COMMANDS.md) 是命令、参数和示例的唯一事实源；[配置参考](../CONFIGURATION.md) 是字段、默认值与配置语义的唯一事实源。
- [最终目标](../ULTIMATE_GOAL.md)、[Prompt 文档](../PROMPTS.md) 和贡献指南服务于维护者，不应在用户指南中复述实现细节。
- 新功能或行为边界变化时，更新对应 Wiki 页面；参数、默认值和输出路径变化时，同时更新权威参考与 [参考索引](Reference.md)。
- 中英文页面保持相同的信息架构；英文版本位于 [en/](en/Home.md)。
- 主题页以“适用场景”开场，以前后导航收尾；首页负责选择路径，不重复主题页内容。
- Wiki 说明用户可观察到的行为，不复制内部实现细节或不稳定的运行日志。
