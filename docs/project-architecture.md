# Skills Seed 项目架构文档

## 1. 项目定位

`skills-seed` 是一个基于 Go 的 CLI 工具，用来从项目当前代码库或 Git 提交历史中提取编码模式，再把这些模式整理成可供 AI 编码助手使用的技能文档

当前代码里的主目标有四个

1. 初始化项目本地工作区 `.skills-seed`
2. 从当前代码或 Git 历史学习模式
3. 基于已学模式检查代码并给出修复建议
4. 生成 `SKILL.md` 与 `references/` 参考文档

## 2. 实际入口与启动流程

当前正式入口是 `cmd/skills-seed/main.go`

启动流程如下

1. `utils.GetSeedPath()` 向上查找 `.skills-seed`
2. 根据配置或默认值初始化 i18n
3. 创建根命令 `skills-seed`
4. 若存在 `.skills-seed`，通过 `bootstrap` 构建 `container.Container`
5. 注册子命令
6. 执行 Cobra 命令

这里有一个重要特征

- `init` 和 `hook` 不依赖容器
- `learn`、`check`、`generate-skills`、`view` 依赖容器
- 如果项目未初始化，这些依赖容器的命令会直接报错

## 3. 实际 CLI 命令

当前 CLI 注册的命令有

- `init`
- `learn`
- `check`
- `generate-skills`
- `patterns`
- `profile`
- `view`
- `hook`

其中 `learn` 进一步拆成

- `learn current`
- `learn history`

其中新增职责

- `patterns merge`：显式合并相似 patterns，替代 `generate-skills --merge`
- `profile show` / `profile refresh`：查看或刷新项目画像

## 4. 分层架构

项目整体采用“命令层 -> 服务层 -> 领域层 / 基础设施层”的结构，外加 AI Agent 与模板层

### 4.1 目录分层

```text
cmd/skills-seed/
  main.go
internal/
  command/      CLI 命令入口
  bootstrap/    启动编排
  container/    依赖注入容器
  domain/       领域模型、仓储接口、领域错误
  service/      应用服务与流程编排
  infra/        配置、Git、BoltDB、事件总线
  agent/        AI 抽象与 Claude / Codex CLI 实现
  prompts/      Prompt 模板加载与项目覆盖
  templates/    Skills 模板加载器
  i18n/         多语言资源
  pkg/          日志、颜色、输出
  utils/        通用工具与交互选择器
embedfs/
  templates/    被 embed 进二进制的模板资源
```

### 4.2 依赖关系

```text
cmd -> bootstrap -> command -> container -> service
service -> domain interfaces
service -> infra implementations
service -> agent
service -> prompts/templates
infra/config + i18n + logger 为横切能力
```

## 5. 核心模块说明

### 5.1 命令层 `internal/command`

- `init`
  - 创建 `.skills-seed/`
  - 创建 `memory/`、`logs/`
  - 生成 `config.yaml`
- `learn`
  - `current`: 分析当前代码库，保存模式，并保存项目画像
  - `history`: 分析 Git 历史提交，保存模式
- `profile`
  - 查看或刷新 `.skills-seed/memory/project-profile.json`
- `check`
  - 分析暂存区或全量文件
  - 支持交互式修复策略选择
- `generate`
  - 从数据库读取模式、从项目画像读取项目概览输入，生成 skills 文档
- `patterns`
  - 显式合并相似 patterns
- `view`
  - 查看 patterns / rules
- `hook`
  - 安装、卸载、执行 pre-commit hook

### 5.2 容器层 `internal/container`

`internal/container/container.go` 负责统一组装依赖

- 配置仓储 `config.Repository`
- Git 仓储 `git.Repository`
- BoltDB 模式仓储 `boltdb.PatternRepository`
- 规则仓储 `boltdb.RuleRepository`
- Claude / Codex Agent
- EventBus / StatsHandler
- Analyzer / Learner / Checker / Generator / Merger 服务
- PromptLoader / SkillsLoader
- PromptLoader / SkillsLoader

容器也是整个项目里最能反映“真实架构”的位置

### 5.3 领域层 `internal/domain`

核心对象

- `Pattern`
  - 模式主聚合
  - 含分类、规则、好坏示例、置信度、频次、是否合并、业务方法等
- `BusinessMethod`
  - 用于描述项目特有的方法能力
- `Issue`
  - 代码检查发现的问题
- `Rule`
  - 基于模式的规则定义
- `CommitInfo`
  - Git 提交值对象
- `FileInfo`
  - 文件路径、内容、语言、状态

分类 `Category` 当前支持

- `naming`
- `error`
- `structure`
- `concurrency`
- `testing`
- `business`
- `api`
- `database`
- `utils`
- `middleware`
- `config`

### 5.4 服务层 `internal/service`

#### `analyzer`

负责“分析当前代码库或项目结构”，主要调用 Agent

- `AnalyzeProject`
- `AnalyzeCurrentCodebase`
- `AnalyzeCodebaseFull`
- `GenerateProjectOverview`

当前 `learn current` 会先走 `AnalyzeCodebaseFull()` 提取 patterns，再走项目分析生成 `.skills-seed/memory/project-profile.json`。`GenerateProjectOverview` 是历史保留能力；主生成链路现在由 `generate-skills` 从 project profile 渲染 `references/project-overview.md`

#### `learner`

负责“从 Git 历史学习模式”的主流程编排

1. 读取提交历史
2. 过滤已分析提交
3. 批量调用 Agent
4. 与已有模式做相似性合并
5. 保存模式
6. 标记提交已分析
7. 发布事件

#### `checker`

负责“把文件 + 模式 + 最近提交”送给 Agent 检查

- `Check()`: 检查暂存区
- `CheckAll()`: 检查所有 Git 跟踪文件
- `CheckFiles()`: 统一检查逻辑

#### `generator`

负责“把模式库变成 skills 文档”

1. 读取全部模式
2. 序列化为 JSON
3. 调用 Agent 生成技能摘要
4. 渲染 `SKILL.md`
5. 读取 project profile 并生成 `references/project-overview.md`
6. 生成 `references/patterns/*.md`
7. 生成 `references/examples/*.md`

#### `merger`

负责调用 Agent 做更高级的模式去重与合并

#### `autofix`

负责应用修复结果，支持四种策略

- `patch`
- `backup`
- `stash`
- `branch`

### 5.5 基础设施层 `internal/infra`

#### 配置 `infra/config`

配置文件位置

```text
.skills-seed/config.yaml
```

配置通过模板生成，并尽量保留注释

主要配置段

- `project`
- `agent`
- `learning`
- `autofix`
- `output`
- `logging`
- `exclude`

#### Git `infra/git`

通过调用本地 `git` 命令完成

- 读取提交历史
- 读取 diff
- 读取暂存文件
- 读取全部文件
- stash / checkout / create branch

#### 存储 `infra/storage/boltdb`

数据库位置

```text
.skills-seed/memory/project.db
```

当前 bucket 设计

- `patterns`
  - 下面再按分类建子 bucket
- `metadata`
  - 保存 `analyzed_commits`
- `rules`
  - 规则仓储会使用这个 bucket

注意当前实现现状

- `PatternRepository` 初始化时只显式创建了 `patterns` 与 `metadata`
- `rules` bucket 没有在初始化时创建
- 因此 `RuleRepository.Save/Get` 在真正写规则时存在空 bucket 风险

#### 事件 `infra/events`

提供简单的发布订阅总线

当前事件包括

- `pattern.learned`
- `pattern.merged`
- `learning.completed`
- `code.analyzed`
- `issue.found`
- `issue.fixed`
- `checking.completed`
- `skills.generated`
- `skills.updated`
- `generation.completed`

容器默认给所有事件挂了两个 handler

- `LoggingHandler`
- `StatsHandler`

### 5.6 Agent 层 `internal/agent`

当前唯一实现是 `internal/agent/claude/claude.go`

它通过本地 `claude` CLI 驱动 AI 能力，核心接口包括

- `AnalyzeCode`
- `LearnFromCommit`
- `BatchLearnFromCommits`
- `GenerateFixes`
- `GenerateSkillsSummary`
- `MergePatterns`
- `AnalyzeProject`
- `AnalyzeCurrentCodebase`

特征

- 提示词先读取 embed 模板，再按需叠加 `.skills-seed/prompts/project/*.project.md`
- 允许继续叠加 `.skills-seed/prompts/custom/*.override.md`
- 输出按 JSON 解析
- 某些场景有 fallback 逻辑
- 包含简单的速率限制重试

### 5.7 Prompt 与模板层 `internal/prompts`、`internal/templates`、`embedfs/templates`

模板分两类

1. Prompt 模板
   - `embedfs/templates/prompts/<provider>/*.tmpl`
   - `embedfs/templates/prompts/common/*.tmpl`
   - `embedfs/templates/prompts/project/*.tmpl`
2. Skills 模板
   - `embedfs/templates/skills/<provider>/*.tmpl`
   - `embedfs/templates/skills/<provider>/references/**/*.tmpl`
   - `embedfs/templates/skills/<provider>/agents/*.tmpl`
   - `embedfs/templates/skills/common/**/*.tmpl`

所有模板都通过 `embed.FS` 打包进二进制。初始化时会生成项目专属 prompt 文件，文本资产不再硬编码在 Go 代码里

主程序版本统一记录在 `internal/metadata.ProgramVersion`。Prompt 模板与 Skills 模板不维护独立语义版本，运行时按嵌入式模板目录计算 SHA-256；`skills-seed --version` 会打印主程序版本、Prompt 模板 hash、Skills 模板 hash，生成文件头部也会记录对应模板 hash

### 5.8 多语言 `internal/i18n`

当前支持

- `zh-CN`
- `en-US`

默认语言是 `zh-CN`，但配置模板默认会按系统环境尽量选择语言

### 5.9 日志与输出

- `logger` 同时负责控制台输出和 JSON 日志文件
- 日志目录默认是 `.skills-seed/logs`
- `output` 和 `colors` 提供了补充的终端输出能力

## 6. 核心业务数据流

### 6.1 初始化流

```text
skills-seed init
  -> 检查 .git
  -> 创建 .skills-seed/
  -> 创建 memory/ logs/
  -> 生成 config.yaml
  -> 初始化日志
```

### 6.2 当前代码学习流

```text
skills-seed learn current
  -> AnalyzerService.AnalyzeCodebaseFull
  -> Agent.AnalyzeCurrentCodebase
  -> 返回 patterns
  -> 保存到 BoltDB
  -> AnalyzerService.AnalyzeProjectFullWithLanguage
  -> 保存 .skills-seed/memory/project-profile.json
```

`learn current` 不再直接输出 `SKILL.md` 或 `references/`

### 6.3 Git 历史学习流

```text
skills-seed learn history
  -> GitRepository.GetCommits
  -> 过滤 analyzed_commits
  -> Agent.BatchLearnFromCommits
  -> 保存 / 合并 Pattern
  -> MarkCommitAnalyzed
```

### 6.4 代码检查流

```text
skills-seed check
  -> 读取 staged files 或 all files
  -> 读取 patterns
  -> 读取 recent commits
  -> Agent.AnalyzeCode
  -> 返回 issues
  -> 可选进入 autofix
```

### 6.5 文档生成流

```text
skills-seed generate-skills
  -> 读取所有 Pattern
  -> 读取 .skills-seed/memory/project-profile.json
  -> Agent.GenerateSkillsSummary
  -> 渲染 SKILL.md
  -> 渲染 references/project-overview.md
  -> 渲染 references/patterns/*.md
  -> 渲染 references/examples/*.md
```

## 7. 产物与目录

初始化后主要目录

```text
.skills-seed/
  config.yaml
  memory/project.db
  logs/*.log
```

生成 skills 后主要目录

```text
<skills-output-dir>/
  SKILL.md
  agents/*.yaml
  references/
    project-overview.md
    patterns/*.md
    examples/*.md
```

## 8. 当前实现与文档不一致处

这是阅读代码后最值得记录的部分

### 8.1 README 与 CLI 实现不一致

旧文档和部分历史 i18n 文案曾出现 `skills-seed learn`、`analyze`、`scan` 等入口；当前主线入口以 `learn current/history`、`profile refresh`、`generate-skills` 为准

### 8.3 `project-overview.md` 的真实生成方式

- `learn current` 会通过项目分析结果保存 `.skills-seed/memory/project-profile.json`
- `profile refresh` 可单独刷新项目画像
- `generate-skills` 从项目画像渲染 `references/project-overview.md`
- 缺少项目画像时，`generate-skills` 明确失败并提示先刷新画像

也就是说，`project-overview.md` 已经从占位文件改为由稳定中间产物生成的输出文件

### 8.4 Makefile 与仓库结构

Makefile 使用

```makefile
./cmd/skills-seed
```

当前正式入口位于 `cmd/skills-seed/main.go`

### 8.5 输出路径说明与实现有落差

配置注释说 `output.skills_path` 相对项目根目录，但生成阶段直接把这个字符串交给 `os.MkdirAll()` / `filepath.Join()` 使用，没有统一做“相对项目根目录”的解析归一

### 8.6 RuleRepository 目前更像预留能力

- 容器创建了 `RuleRepo`
- `view rules` 可以读取规则
- 但学习链路目前主要保存 `Pattern`
- 规则 bucket 初始化也未补齐

## 9. 结论

从代码事实看，当前项目已经形成了一个清晰的主骨架

- CLI 命令层
- bootstrap 启动层
- 容器式依赖组装
- 领域模型与仓储接口
- 服务编排层
- Git / BoltDB / 配置 / 事件等基础设施
- Claude / Codex CLI 驱动的 Agent 能力
- 模板化的技能文档输出

它已经具备“学习模式 -> 存储模式 -> 检查代码 -> 生成技能文档”的闭环，但仍有几个明显的演进点

1. 把项目概览生成真正接入命令流
2. 清理 README / i18n / Makefile 与实现不一致的问题
3. 完整打通 rule 存储与使用链路
4. 明确输出路径与相对路径解析规则
