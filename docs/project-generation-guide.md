# Skills Seed 生成说明文档

## 1. 文档目标

本文档说明 `skills-seed` 如何从代码与 Git 历史中生成模式库，再把模式库转换为 AI 编码助手可用的技能文档。

这里的“生成”包含三层：

1. 生成内部知识资产：`Pattern`、提交分析状态、规则预留结构
2. 生成项目画像：`.skills-seed/memory/project-profile.json`
3. 生成外部文档产物：`SKILL.md` 与 `references/`

## 2. 生成链路总览

`skills-seed` 当前存在两条主要生成路径：

### 路径 A：从当前代码库学习后生成

```text
learn current
  -> 分析当前代码库
  -> 提取 Pattern
  -> 存入 BoltDB
  -> 分析项目结构
  -> 保存 project-profile.json

generate-skills
  -> 读取 Pattern
  -> 读取 project-profile.json
  -> 生成 skills 文档
```

### 路径 B：从 Git 历史学习后再生成

```text
learn history
  -> 分析 Git 提交历史
  -> 提取 Pattern
  -> 存入 BoltDB

profile refresh
  -> 分析项目结构
  -> 保存 project-profile.json

generate-skills
  -> 读取 Pattern
  -> 读取 project-profile.json
  -> 生成 skills 文档
```

## 3. 初始化阶段产物

执行：

```bash
skills-seed init
```

会创建：

```text
.skills-seed/
  config.yaml
  memory/
  logs/
  prompts/
    system/
    project/
    custom/
```

其中：

- `config.yaml` 保存项目、Agent、学习、输出、日志配置
- `memory/` 用于放 BoltDB
- `logs/` 用于放命令执行日志
- `prompts/` 保存初始化生成的项目专属 prompt 与自定义覆盖 prompt；文件头部记录主程序版本和 Prompt 模板 SHA-256

## 4. 模式是如何生成的

### 4.1 `learn current`

执行：

```bash
skills-seed learn current
```

真实调用链：

```text
command/learn.runLearnCurrent
  -> AnalyzerService.AnalyzeCodebaseFull
  -> AnalyzerService:
     - 获取目录结构
     - 找入口文件
     - 收集示例文件
  -> Agent.AnalyzeCurrentCodebase
  -> 返回 []domain.Pattern
  -> PatternRepo.Save
  -> AnalyzerService.AnalyzeProjectFullWithLanguage
  -> 保存 .skills-seed/memory/project-profile.json
```

它适合：

- 新项目第一次接入
- 想快速从“当前代码状态”提炼规范
- 没有足够历史提交可学习

注意：`learn current` 不再直接生成 `SKILL.md` 或 `references/`。学习完成后需要显式运行：

```bash
skills-seed generate-skills
```

### 4.2 `learn history`

执行：

```bash
skills-seed learn history --limit=50
```

真实调用链：

```text
command/learn.runLearnHistory
  -> LearnerService.Learn
  -> GitRepository.GetCommits
  -> CommitAnalysisTracker.IsCommitAnalyzed
  -> Agent.BatchLearnFromCommits
  -> PatternRepo.FindSimilar / Save
  -> CommitAnalysisTracker.MarkCommitAnalyzed
```

它适合：

- 团队已有较稳定提交历史
- 希望从“演进过程”中总结模式
- 需要增量学习

### 4.3 模式结构

AI 返回并落库的核心对象是 `domain.Pattern`，主要字段有：

- `ID`
- `Name`
- `Category`
- `Description`
- `GoodExample`
- `BadExample`
- `Rule`
- `Confidence`
- `Frequency`
- `Merged`
- `MergedFrom`
- `BusinessMethod`

其中：

- `Confidence` 表示模式可信度
- `Frequency` 表示出现频次
- `BusinessMethod` 用于描述业务方法或工具方法

## 5. 模式如何存储

数据库文件：

```text
.skills-seed/memory/project.db
```

当前 BoltDB 逻辑：

- `patterns` bucket
  - 再按分类建立子 bucket
  - 每个 pattern 以 `ID` 为 key
- `metadata` bucket
  - 保存 `analyzed_commits`

这意味着：

1. 模式是按分类组织的
2. Git 历史学习支持增量处理
3. 相同提交不会反复学习

## 6. 文档是如何生成的

执行：

```bash
skills-seed generate-skills
```

真实调用链：

```text
command/generate.runGenerate
  -> PatternRepo.Count
  -> 可选 MergerService.MergePatterns（兼容 --merge，不推荐）
  -> GeneratorService.GenerateSkills
```

`GeneratorService.GenerateSkills()` 的步骤如下：

1. 从仓储读取全部 `Pattern`
2. 把模式序列化成 JSON
3. 读取已有 `SKILL.md` 作为上下文
4. 调用 `Agent.GenerateSkillsSummary`
5. 使用 Skills 模板渲染主文档
6. 读取 `.skills-seed/memory/project-profile.json` 渲染 `references/project-overview.md`
7. 按分类生成参考文档

## 7. 生成文档的输入

### 7.1 模式输入

最核心的输入是数据库中的全部 `Pattern`。

### 7.2 配置输入

来自 `.skills-seed/config.yaml` 的关键字段：

- `project.name`
- `project.language`
- `project.locale`
- `agent.provider`
- `output.skills_paths`（优先按 provider 取值）
- `output.skills_path`（兼容旧字段）

### 7.3 项目画像输入

项目概览来自 `.skills-seed/memory/project-profile.json`。

该文件由以下命令生成或刷新：

```bash
skills-seed learn current
skills-seed profile refresh
```

`generate-skills` 只消费该文件，不会在缺失时隐式生成项目画像。

### 7.4 模板输入

模板来自嵌入式文件系统，并按 `agent.provider -> common` 的顺序查找：

- `embedfs/templates/skills/<provider>/SKILL*.tmpl`
- `embedfs/templates/skills/<provider>/references/project-overview*.tmpl`
- `embedfs/templates/skills/<provider>/references/patterns/*.tmpl`
- `embedfs/templates/skills/<provider>/references/examples/*.tmpl`
- `embedfs/templates/skills/<provider>/agents/*.tmpl`（可选附加元数据）
- `embedfs/templates/skills/common/**`（provider 模板缺失时的通用回退）

主程序版本来自 `internal/metadata.ProgramVersion`。Skills 模板不维护独立版本号，渲染时会注入 `SkillsTemplatesHash`，主 `SKILL.md` 文件头部记录该 SHA-256。

### 7.5 AI 汇总输入

`GenerateSkillsSummary` 会拿到：

- 全部模式 JSON
- 模式总数
- 已有技能文档内容
- 项目名
- 项目语言

它返回：

- `CategorySummaries`
- `KeyPatterns`
- `BusinessRules`
- `BestPractices`
- `CommonPatterns`

## 8. 生成后的输出结构

默认输出目录来自 `.skills-seed/config.yaml`：

```text
output.skills_paths[agent.provider] -> output.skills_path
```

生成结果结构如下：

```text
<skills-output-dir>/
  SKILL.md
  agents/             # 如果 provider 模板提供附加元数据
  references/
    project-overview.md
    patterns/
      api.md
      business.md
      concurrency.md
      config.md
      database.md
      error.md
      middleware.md
      naming.md
      structure.md
      testing.md
      utils.md
    examples/
      api.md
      business.md
      concurrency.md
      config.md
      database.md
      error.md
      middleware.md
      naming.md
      structure.md
      testing.md
      utils.md
```

是否真的生成某个分类文件，取决于：

- AI 是否返回该分类 summary
- 本地是否存在该分类模板

## 9. `SKILL.md` 的生成逻辑

主文档模板会接收以下关键数据：

- `ProgramVersion`
- `SkillsTemplatesHash`
- `ProjectName`
- `Language`
- `PatternCount`
- `AvgConfidence`
- `Categories`
- `LastUpdated`
- `CategorySummaries`
- `KeyPatterns`
- `BusinessRules`
- `BestPractices`
- `CommonPatterns`
- `STATS`

`STATS` 是本地额外计算的统计信息，包括：

- 总模式数
- 平均置信度
- 高置信度模式
- 高频模式
- 按分类分组

## 10. 分类参考文档的生成逻辑

### 10.1 `references/patterns/*.md`

每个分类模式文档会拿到：

- 分类摘要
- 分类下的模式名称列表
- 分类下完整 `PatternObjects`
- 使用场景
- 优先级
- 分类平均置信度
- 业务方法信息

这是“模式解释文档”。

### 10.2 `references/examples/*.md`

每个分类示例文档会拿到：

- 分类名
- 分类摘要
- 前 3 个模式名作为示例输入

这是“示例入口文档”，内容主要受模板控制。

## 11. 项目概览文档的生成方式

当前链路已经把项目概览拆成两层：

- `learn current` / `profile refresh` 负责分析并保存 `.skills-seed/memory/project-profile.json`
- `generate-skills` 负责读取 `project-profile.json` 并渲染 `references/project-overview.md`

因此现在的实际行为是：

1. `learn current` 提取 patterns，并额外生成项目画像
2. `profile refresh` 可以单独刷新项目画像
3. `generate-skills` 检查项目画像是否存在
4. 如果存在，使用 provider 的 `project-overview.md.tmpl` 生成完整项目概览
5. 如果不存在，命令明确失败并提示先运行 `learn current` 或 `profile refresh`

`project-overview.md` 是输出产物，不再作为知识源。稳定知识源是 `.skills-seed/memory/project-profile.json`。

## 12. 模式合并是如何工作的

执行：

```bash
skills-seed patterns merge
```

会先调用：

```text
MergerService.MergePatterns
  -> Agent.MergePatterns
```

合并过程：

1. 读取全部模式或指定分类模式
2. 交给 AI 判断哪些模式应合并
3. 删除被合并的旧模式
4. 写入新的合并后模式

合并后的模式会带：

- `Merged = true`
- `MergedFrom = [...]`

## 13. 检查与修复链路如何复用生成资产

执行：

```bash
skills-seed check
```

会复用已生成的模式库：

1. 读取待检查文件
2. 读取已学模式
3. 读取最近提交
4. 交给 Agent 分析
5. 输出 `Issue`

如果进入自动修复，还会触发：

- `GenerateFixes`
- `AutofixService`

修复策略有：

- `patch`
- `backup`
- `stash`
- `branch`

## 14. 生成链路中的关键限制

### 14.1 当前依赖本地 `claude` CLI

如果本地没有 `claude` 命令，AI 能力不可用。

### 14.2 `learn current` 与 `learn history` 产物风格可能不同

- `current` 更像一次性全量静态扫描
- `history` 更像从提交演进中抽取共识

### 14.3 `project-overview.md` 由项目画像生成

`project-overview.md` 不再由 `learn current` 直接写出，也不再使用 TODO 占位。

当前规则：

- `learn current` 或 `profile refresh` 生成 `.skills-seed/memory/project-profile.json`
- `generate-skills` 读取 project profile 并渲染 `references/project-overview.md`
- 如果 project profile 不存在，`generate-skills` 会失败并提示先运行 `learn current` 或 `profile refresh`

### 14.4 路径解析仍然偏直接

`output.skills_path` 当前没有统一做绝对路径归一，配置写法需要谨慎。

## 15. 推荐使用顺序

按当前实现，最稳妥的使用顺序是：

### 场景 A：新项目初始化

```bash
skills-seed init
skills-seed learn current
skills-seed generate-skills
```

### 场景 B：已有稳定提交历史

```bash
skills-seed init
skills-seed learn history --limit=50
skills-seed profile refresh
skills-seed patterns merge
skills-seed generate-skills
```

### 场景 C：日常迭代

```bash
skills-seed learn history --since=7d
skills-seed profile refresh   # 项目结构或关键模块变化时执行
skills-seed generate-skills
skills-seed check
```

## 16. 当前实现上的改进建议

从生成链路角度，建议优先补这几项：

1. 统一 `output.skills_path` 的相对路径解析
2. 补齐 `check --fix` / `learn staged` 的职责拆分
3. 继续统一 README、i18n、CLI 实现里的命令命名
4. 修正 Makefile 的构建入口

## 17. 总结

当前项目已经具备完整的“知识提取 + 文档生成”主链路：

```text
代码 / Git 历史
  -> AI 提取 Pattern
  -> BoltDB 存储
  -> AI 汇总
  -> 模板渲染
  -> AI skills 文档
```

真正已经稳定打通的是：

- 模式提取
- 模式存储
- 技能主文档生成
- 分类参考文档生成

当前还处于“能力已具备、接线未完全完成”的部分主要是：

- 完整项目概览生成
- 规则体系完整落地
- 文档与实现的一致性收口
