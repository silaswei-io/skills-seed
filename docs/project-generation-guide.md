# Skills 生成链路

[English](project-generation-guide.en.md)

## 总览

Skills Seed 的生成链路分两段：

1. 学习阶段生成内部知识资产
2. 生成阶段把内部知识资产渲染成 AI skills

```text
代码 / Git 历史
  -> Pattern
  -> ProjectProfile
  -> ProjectSpec
  -> SKILL.md + references/
```

## 核心产物

### 内部知识资产

```text
.skills-seed/memory/project.db
.skills-seed/memory/project-profile.json
.skills-seed/memory/project-spec.json
```

workspace 模式额外包含：

```text
.skills-seed/memory/workspace-profile.json
.skills-seed/memory/workspace-spec.json
```

### 外部 skills 产物

```text
<skills-output>/
  SKILL.md
  agents/
  references/
    project-overview.md
    project-spec.md
    patterns/*.md
    examples/*.md
```

workspace 模式会生成根 skill 和每个子项目自己的 skill。

## 初始化

```bash
skills-seed init --mode project
skills-seed init --mode workspace
```

初始化会创建：

```text
.skills-seed/
  config.yaml
  memory/
  logs/
  prompts/
```

`project.mode` 决定后续学习和生成路径。开始学习或生成后不能直接切换模式。

## 从当前代码学习

```bash
skills-seed learn current
```

project 模式流程：

```text
runLearnCurrent
  -> 解析项目根目录、语言、focus 参数
  -> 比较普通项目文件 md5，生成增量 focus paths
  -> AnalyzerService.AnalyzeCodebaseFullWithOptions
  -> Agent.AnalyzeCurrentCodebase
  -> 保存 patterns
  -> 按 --profile 策略生成或刷新 project-profile.json
  -> 输出后续步骤和 Token 消耗
```

`learn current` 会在成功分析后把普通项目文件的 md5 写入 `.skills-seed/memory/project.db`。下一次执行会先比较文件指纹：

- 没有新增、修改或删除的可学习文件：跳过 patterns 学习和项目画像刷新
- 有新增或修改文件：只把这些文件作为增量 focus paths
- 只有删除文件：跳过 patterns 学习，并在已有画像基础上刷新项目画像

生成的 skills 输出目录默认排除，不需要手动写入 `exclude`。

`--profile`：

- `auto`：默认。首次或全量学习刷新画像；窄范围学习尽量跳过
- `skip`：只学习 patterns
- `refresh`：刷新画像

示例：

```bash
skills-seed learn current
skills-seed learn current --focus internal/service --profile skip
skills-seed learn current --focus internal/domain --profile refresh
```

`learn current` 不直接生成 `SKILL.md`。需要继续执行：

```bash
skills-seed generate-skills
```

单项目模式会把 Token 消耗作为学习输出的最后一条日志。workspace 模式按子项目并发学习，每个子项目的 Token 消耗跟随该子项目的完成日志输出，并带有子项目名称。

## 从 Git 历史学习

```bash
skills-seed learn history --limit=50
skills-seed learn history --since=30d
```

流程：

```text
LearnerService.Learn
  -> GitRepository.GetCommits
  -> CommitAnalysisTracker.IsCommitAnalyzed
  -> 过滤已学习 commit
  -> Agent.BatchLearnFromCommits
  -> PatternRepo.FindSimilar / Save
  -> CommitAnalysisTracker.MarkCommitAnalyzed
```

已分析 commit 存在 BoltDB metadata 的 `analyzed_commits` 中。下次历史学习会按 commit hash 跳过。

如果某个 batch 的 AI 调用失败，该 batch 的 commit 不会被标记，下次会重试。

## 项目画像

项目画像是稳定的项目事实输入：

```text
.skills-seed/memory/project-profile.json
```

生成方式：

```bash
skills-seed learn current
skills-seed profile refresh
```

`profile refresh` 只刷新画像，不学习 patterns。

画像包含：

- 项目摘要
- 架构说明
- 关键模块
- 依赖与数据流
- 常用工具方法
- 业务方法
- 配置和框架模式

## 项目规范

项目规范是给 AI 修改代码时优先读取的开发约束：

```text
.skills-seed/memory/project-spec.json
references/project-spec.md
```

`generate-skills` 会从项目画像和已学习 patterns 生成规范。规范包含：

- 模块和层次边界
- pattern rules
- 配置和框架规则
- 改动触点
- workspace 子项目范围信息

workspace 子项目在各自子仓的 `.skills-seed` 中保存自己的 `project-spec.json` 和 `references/project-spec.md`。

## 生成 Skills

```bash
skills-seed generate-skills
```

project 模式流程：

```text
GeneratorService.GenerateSkills
  -> 解析 output.skills_paths[agent.provider]
  -> 读取 patterns
  -> 读取 project-profile.json
  -> generation.mode=template 时直接从已学习数据生成摘要
  -> generation.mode=ai 时调用 Agent.GenerateSkillsSummary 做摘要合并
  -> 生成 project-spec.json
  -> 渲染 SKILL.md
  -> 渲染 references/
```

如果没有 patterns，命令会跳过生成。如果缺少项目画像，命令会提示先运行 `learn current` 或 `profile refresh`。

`generation.mode` 默认是 `template`，生成阶段不额外调用 AI。因为 patterns、项目画像和项目规范已经由学习阶段产生，template 模式更适合稳定批量生成和 workspace 多子仓场景。需要让 AI 对大量 patterns 做摘要润色或压缩时，可在 `.skills-seed/config.yaml` 中改为：

```yaml
generation:
  mode: "ai"
```

## Workspace 生成

workspace 模式流程：

```text
skills-seed generate-skills
  -> 逐个进入 workspace.projects 中的独立 Git 子仓
  -> 使用子仓自己的 .skills-seed/config.yaml 生成子仓 skill
  -> 如果子仓目标 SKILL.md 没有 generated-by 标记，视为手写 skill 并跳过覆盖
  -> 回到 workspace 根仓生成根 skill
  -> 根 skill 读取子仓 provider、output.skills_paths 和已生成 skill 摘要
  -> 生成 workspace-overview.md 和 cross-project-rules.md
```

根 skill 的职责：

- 判断改动属于哪个子项目
- 指导 AI 读取对应子项目 skill
- 描述共享目录、契约目录和部署目录
- 描述跨项目改动顺序和风险

子项目 skill 的职责：

- 描述本项目架构
- 描述本项目边界和 rules
- 提供本项目 patterns 和 examples

子项目 skill 由子仓自己的配置和数据生成。可以在子仓目录单独执行 `skills-seed learn current` 和 `skills-seed generate-skills`，也可以在 workspace 根仓执行 `skills-seed generate-skills` 统一编排所有子仓。子项目画像、patterns、文件 md5 指纹和 skill 都保存在子仓自己的 `.skills-seed` 中；workspace 根仓只在最后读取子项目配置和已生成的 skill 内容来生成路由。

## 模板选择

Skills 模板按 provider 查找，再回退到 common：

```text
embedfs/templates/skills/<provider>/
embedfs/templates/skills/common/
```

workspace 根模板位于：

```text
embedfs/templates/skills/common/workspace/
```

Prompt 模板：

```text
embedfs/templates/prompts/common/
embedfs/templates/prompts/project/
embedfs/templates/prompts/workspace/
```

所有模板都参与 hash 计算。生成文件头部会记录主程序版本和模板 hash。

## 输出路径

默认输出路径来自：

```text
output.skills_paths[agent.provider]
```

workspace 模式也只使用当前 `agent.provider` 的输出路径；根 skill 写到 workspace 根目录，子项目 skill 写到各自子项目目录。

示例：

```yaml
agent:
  provider: "codex"

output:
  skills_paths:
    claude: ".claude/skills/skills-seed-skills"
    codex: ".agents/skills/skills-seed-skills"
```

也可以临时指定：

```bash
skills-seed generate-skills --output .agents/skills/my-project
```

相对路径会基于项目根目录解析。

## 模式合并

相似 patterns 可以显式合并：

```bash
skills-seed patterns merge
skills-seed generate-skills
```

`generate-skills --merge` 仍兼容，但推荐使用 `patterns merge`。

## 推荐流程

### 新单项目

```bash
skills-seed init --mode project
skills-seed learn current
skills-seed generate-skills
```

### 已有提交历史的项目

```bash
skills-seed init --mode project
skills-seed learn history --limit=50
skills-seed profile refresh
skills-seed patterns merge
skills-seed generate-skills
```

### Workspace

```bash
skills-seed init --mode workspace
# 检查 .skills-seed/config.yaml 中 workspace.projects
skills-seed learn current
skills-seed generate-skills
```

### 日常迭代

```bash
skills-seed learn current --focus <path> --profile skip
skills-seed learn history --since=7d
skills-seed generate-skills
skills-seed check
```
