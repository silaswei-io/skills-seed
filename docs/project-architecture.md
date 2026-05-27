# Skills Seed 架构

[English](project-architecture.en.md)

## 定位

Skills Seed 是一个本地 CLI 工具，用来把项目已有代码和 Git 历史转成 AI 编码助手可读取的 skills。

主链路：

```text
init
  -> learn current / learn history / profile refresh
  -> patterns + project profile
  -> generate-skills
  -> output.skills_paths[agent.provider]
```

## 运行模式

### Project 模式

`project` 是默认模式。初始化根目录被当成一个项目处理。

```bash
skills-seed init --mode project
```

主要产物：

```text
.skills-seed/
  config.yaml
  memory/
    project.db
    project-profile.json
    project-spec.json
  logs/
<provider-skill-path>/<skill-name>/
```

### Workspace 模式

`workspace` 用于一个 Git 仓库下包含多个子项目的场景。

```bash
skills-seed init --mode workspace
```

初始化只扫描 workspace 根目录的第一层文件夹，并按常见标记自动发现子项目：

- `package.json`
- `pnpm-workspace.yaml`
- `deno.json`
- `go.mod`
- `go.work`
- `pyproject.toml`
- `requirements.txt`
- `Cargo.toml`
- `pom.xml`
- `build.gradle` / `build.gradle.kts`
- `composer.json`
- `Gemfile`
- `mix.exs`
- `CMakeLists.txt`
- `.csproj` / `.sln`
- `Dockerfile`
- `buf.yaml`
- `openapi.yaml` / `openapi.yml`
- `Chart.yaml`
- `main.tf`

可在 `.skills-seed/config.yaml` 中手动调整：

```yaml
project:
  mode: "workspace"

workspace:
  projects:
    - {id: "frontend", path: "frontend", type: "frontend", language: "typescript"}
    - {id: "backend", path: "backend", type: "backend", language: "go"}
  shared:
    - {path: "pkg"}
  contracts:
    - {path: "proto"}
  infra:
    - {path: "deploy"}
```

workspace 模式额外产物：

```text
.skills-seed/memory/
  workspace-profile.json
  workspace-spec.json

<workspace-root>/<provider-skill-path>/<workspace-skill>/
<child-project>/<provider-skill-path>/<project-skill>/
```

根 skill 的 `provider-skill-path` 来自 workspace 根配置中当前 `agent.provider` 对应的 `output.skills_paths`。子项目 skill 路径来自子项目自己的 `.skills-seed/config.yaml`。根 skill 负责路由、跨项目规则和影响范围；子项目 skill、子项目画像、patterns 和文件 md5 指纹都由子仓自己的 `.skills-seed` 管理。

## 初始化锁定

初始化模式保存在 `.skills-seed/config.yaml` 的 `project.mode`，运行状态保存在 `.skills-seed/memory/state.json`。

开始学习或生成后，模式会被锁定，避免 `project` 与 `workspace` 数据结构混用。

需要重新初始化时使用：

```bash
skills-seed reset --mode workspace
```

旧 `.skills-seed` 会备份到 `.skills-seed.backup/<timestamp>`。

## CLI 入口

正式入口：

```text
cmd/skills-seed/main.go
```

启动流程：

1. 向上查找 `.skills-seed`
2. 初始化 i18n
3. 创建根命令
4. 如已初始化，创建 `container.Container`
5. 初始化 logger
6. 注册命令并执行

当前命令：

- `init`
- `reset`
- `learn current`
- `learn history`
- `check`
- `generate-skills`
- `patterns merge`
- `profile show`
- `profile refresh`
- `view patterns`
- `hook install|uninstall|run`

## 分层

```text
cmd/skills-seed/       CLI 入口
internal/bootstrap/    启动编排
internal/command/      Cobra 命令
internal/container/    依赖组装
internal/domain/       领域模型和接口
internal/service/      应用服务
internal/infra/        配置、Git、存储
internal/agent/        Claude / Codex Agent
internal/prompts/      Prompt 加载
internal/templates/    Skills 模板加载
internal/i18n/         国际化
internal/pkg/          日志、进度、token 统计
internal/workspace/    workspace 发现与并发编排
embedfs/templates/     内置模板
```

依赖方向：

```text
command -> service -> domain interfaces
service -> infra implementations
service -> agent
service -> prompts/templates
```

## 核心模块

### `container`

`internal/container/container.go` 组装运行时依赖：

- 配置仓储
- Git 仓储
- BoltDB pattern 仓储
- 项目画像仓储
- workspace 画像与规范仓储
- Prompt / Skills loader
- Claude 或 Codex Agent
- Analyzer / Learner / Checker / Generator / Merger 服务

历史学习的 commit tracker 由 BoltDB pattern 仓储实现。

### `domain`

主要模型：

- `Pattern`：代码模式。workspace 模式下包含 `project_id`、`scope_path`、`workspace_role`
- `ProjectProfile`：项目事实画像
- `ProjectSpec`：由画像和 patterns 生成的项目级开发规范
- `WorkspaceProfile`：workspace 子项目与共享路径画像
- `WorkspaceSpec`：workspace 根 skill 使用的路由和跨项目规则
- `Issue`：检查结果
- `CommitInfo` / `FileInfo`：Git 和文件值对象

### `service/analyzer`

负责当前代码库分析：

- 提取 patterns
- 分析项目结构
- 生成或增量刷新项目画像
- 支持 `learn current --focus ... --profile ...`
- `learn current` 使用 Agent token 作用域延迟输出 Token 消耗：project 模式在后续步骤之后刷新；workspace 模式在每个子项目完成日志之后刷新，并标明子项目
- workspace 并发学习时，子项目内部进度和后续命令提示不输出到终端，只保留子项目开始、摘要、Token 和完成日志

### `service/learner`

负责 Git 历史学习：

1. 读取提交历史
2. 查询 `analyzed_commits`
3. 跳过已分析 commit
4. 按 batch 调用 Agent
5. 保存或合并 patterns
6. 成功后标记 commit 已分析

### `service/generator`

负责 skills 输出：

1. 读取 patterns
2. 读取项目画像
3. 生成 `ProjectSpec`
4. 按 `generation.mode` 选择 template 摘要或 AI 摘要
5. 渲染 `SKILL.md`
6. 渲染 `references/project-overview.md`
7. 渲染 `references/project-spec.md`
8. 渲染分类 patterns 和 examples
9. 渲染可选 Agent 元数据

workspace 命令层会先进入每个独立 Git 子仓，调用子仓自己的 `GeneratorService` 生成子仓 skill；遇到没有 `generated-by: skills-seed` 标记的手写 `SKILL.md` 时跳过覆盖。所有子仓处理完后，根仓再生成 workspace 根 skill 和跨项目引用。

### `service/checker`

负责把文件、patterns 和最近提交交给 Agent 检查。默认检查暂存区，`--all` 检查全部 Git 跟踪文件。

### `service/merger`

负责显式合并相似 patterns：

```bash
skills-seed patterns merge
```

## 存储

```text
.skills-seed/
  config.yaml
  memory/
    project.db
    state.json
    project-profile.json
    project-spec.json
    workspace-profile.json
    workspace-spec.json
  logs/*.log
```

BoltDB `project.db` 中保存：

- `patterns`
- `metadata/analyzed_commits`

`analyzed_commits` 用于 `learn history` 增量跳过。

## 模板

Prompt 模板：

```text
embedfs/templates/prompts/common/
embedfs/templates/prompts/project/
embedfs/templates/prompts/workspace/
```

Skills 模板：

```text
embedfs/templates/skills/common/
embedfs/templates/skills/common/workspace/
embedfs/templates/skills/claude/
embedfs/templates/skills/codex/
```

模板会打包进二进制。`skills-seed --version` 会显示主程序版本、Prompt 模板 hash 和 Skills 模板 hash。

## 多语言与输出

当前支持：

- `zh-CN`
- `en-US`

终端文案、日志文案、配置模板、Prompt 模板和 Skills 模板都走 i18n 或嵌入模板，不应在业务代码中硬编码用户可见文本。

## 并发

`agent.parallelism` 控制 Agent 并发数：

- `0`：自动
- project 模式默认 `1`
- workspace 模式默认按子项目数计算，并有上限

workspace `learn current` 和 `generate-skills` 会按子项目并发执行；`learn current` 的每个子项目 patterns、画像和文件指纹都写入对应子仓自己的 `.skills-seed`。

并发学习时，Agent 层会先记录 token 统计并缓存控制台输出，命令层在子项目最终日志输出后再刷新对应 token 作用域，避免 Token 消耗日志插入其他子项目的进度或完成信息之间。
