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
.skills-seed/memory/projects/<project-id>/project-profile.json
.skills-seed/memory/projects/<project-id>/project-spec.json
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
  -> AnalyzerService.AnalyzeCodebaseFullWithOptions
  -> Agent.AnalyzeCurrentCodebase
  -> 保存 patterns
  -> 按 --profile 策略生成或刷新 project-profile.json
  -> 输出后续步骤和 Token 消耗
```

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

workspace 子项目也有自己的 `project-spec.json` 和 `references/project-spec.md`。

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
  -> 调用 Agent.GenerateSkillsSummary
  -> 生成 project-spec.json
  -> 渲染 SKILL.md
  -> 渲染 references/
```

如果没有 patterns，命令会跳过生成。如果缺少项目画像，命令会提示先运行 `learn current` 或 `profile refresh`。

## Workspace 生成

workspace 模式流程：

```text
GeneratorService.GenerateWorkspaceSkills
  -> 按 agent.provider 生成 workspace 根 skill
  -> 读取 workspace.child_skill_policy
  -> root_only 时停止，不写子项目 skill
  -> 读取所有 patterns
  -> 按 project_id 分组
  -> 子项目存在 .skills-seed/config.yaml 时读取子项目 provider 和 output.skills_paths
  -> skip_existing 时跳过已存在解析后 SKILL.md 的子项目
  -> 为每个子项目读取 project-profile.json
  -> 生成子项目 project-spec.json
  -> 按子项目实际配置路径输出子项目 skills
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

子项目 skill 生成策略：

- 子项目有 `.skills-seed/config.yaml`：读取子项目自己的 `agent.provider` 与 `output.skills_paths` 来确定 skill 路径
- `skip_existing`：默认值。按子项目实际配置解析出的 `SKILL.md` 已存在时跳过，仍生成 workspace 根 skill
- `overwrite`：覆盖子项目实际配置路径下的 skill
- `root_only`：只生成 workspace 根 skill

CLI 可临时覆盖配置：

```bash
skills-seed generate-skills --overwrite
skills-seed generate-skills --root-only
```

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
