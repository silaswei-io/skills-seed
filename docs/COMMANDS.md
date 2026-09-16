# Skills Seed 命令说明

[简体中文](COMMANDS.md) | [English](COMMANDS.EN.md)

本文是完整命令参考。所有命令都支持 `--help`。需要读取 `.skills-seed/config.yaml` 的命令必须先执行 `skills-seed init`。

## 命令总览

| 阶段 | 命令 | 用途 | 常见入口 |
|---|---|---|---|
| 基础信息 | [`skills-seed`](#skills-seed) | 查看全局帮助、版本和模板 hash | `skills-seed --help` |
| 初始化 | [`skills-seed init`](#skills-seed-init) | 初始化单项目或 workspace 根仓 | `skills-seed init --mode project` |
| Workspace | [`skills-seed workspace`](#skills-seed-workspace) | 添加或管理 workspace 子项目 | `skills-seed workspace add .` |
| 重置 | [`skills-seed reset`](#skills-seed-reset) | 重新初始化或按范围重置项目知识 | `skills-seed reset all` |
| 学习 | [`skills-seed learn`](#skills-seed-learn) | 从当前代码学习 patterns | `skills-seed learn current` |
| 生成 | [`skills-seed generate`](#skills-seed-generate) | 根据已核验知识和用户资源生成 skills | `skills-seed generate skills` |
| 预览 | [`skills-seed preview`](#skills-seed-preview) | 预览 full 或 incremental 分析会选中的文件 | `skills-seed preview files` |
| 模式管理 | [`skills-seed patterns`](#skills-seed-patterns) | 添加、删除、整理和查看 patterns | `skills-seed patterns show` |
| 工作流 | [`skills-seed workflow`](#skills-seed-workflow) | 添加或更新用户任务工作流 | `skills-seed workflow --name <名称> --content "<Markdown>"` |
| 规则 | [`skills-seed rule`](#skills-seed-rule) | 添加或更新用户权威规则 | `skills-seed rule --name <名称> --content "<Markdown>"` |
| 项目画像 | [`skills-seed profile`](#skills-seed-profile) | 查看项目画像 | `skills-seed profile show` |
| 一键同步 | [`skills-seed sync`](#skills-seed-sync) | 学习当前代码并生成 skills | `skills-seed sync` |
| CLI 更新 | [`skills-seed update`](#skills-seed-update) | 从官方发布资产更新已安装 CLI | `skills-seed update` |
| 变更记录 | [`skills-seed log`](#skills-seed-log) | 查看学习变更记录 | `skills-seed log` |
| Git Hook | [`skills-seed hook`](#skills-seed-hook) | 安装、卸载或手动运行 pre-commit hook | `skills-seed hook install` |
| 帮助 | [`skills-seed help`](#skills-seed-help) | 查看任意命令路径的帮助 | `skills-seed help learn current` |

## 常见工作流

| 场景 | 推荐命令顺序 | 说明 |
|---|---|---|
| 初始化单项目 | `skills-seed init --mode project` → `skills-seed sync` | 创建配置、学习当前代码并生成 skills |
| 初始化 workspace | `skills-seed init --workspace` → `skills-seed workspace add .` → `skills-seed sync` | 根仓编排子项目学习，再生成子项目和根仓 skills |
| 日常增量更新 | `skills-seed sync` | 学习当前变更，有实际学习变化时生成 skills |
| 更新已安装 CLI | `skills-seed update` | 下载并校验官方发布二进制；不读取项目状态，也不需要 Go |
| 只补充一条规则 | `skills-seed patterns add --content "<内容>"` → `skills-seed generate skills` | 用自然语言添加 pattern 后重新生成 |
| 查询任务工作流 | `skills-seed workflow show --format json` | 返回轻量摘要；按需用 `workflow show <id> --format json` 读取详情 |
| 更新任务工作流 | `skills-seed workflow --name <名称> --content "<Markdown>"` → `skills-seed generate skills` | 同名时默认与已有内容合并优化；`--overwrite` 仅用于完整替换 |
| 更新长期规则 | `skills-seed rule --name <名称> --content "<Markdown>"` → `skills-seed generate skills` | 同名时默认保留已有权威要求并增量优化；`--overwrite` 仅用于完整替换正文和范围 |
| 提交前更新 | `skills-seed hook install` | 安装 pre-commit hook，在提交前选择同步、只学习或跳过 |
| 查看沉淀变化 | `skills-seed log` | 像 `git log` 一样查看最近学习和生成带来的变更 |
| 排查沉淀结果 | `skills-seed patterns show` → `skills-seed profile show` | 查看已学习 patterns 和项目画像是否符合预期 |

<!-- COMMAND_TREE_START -->
## 自动生成命令索引

> 本节由 Cobra command tree 生成，用于校验命令、子命令和参数默认值是否与 CLI 实现一致；详细场景说明仍以各命令章节为准。

| 命令 | 摘要 | 子命令 | 参数 |
|---|---|---|---|
| `skills-seed` | 为 AI 助手培育项目技能 | `clear`, `cli-skills`, `generate`, `hook`, `init`, `learn`, `log`, `patterns`, `preview`, `profile`, `reset [all\|patterns\|rules\|workflows]...`, `rule`, `sync`, `update`, `workflow`, `workspace` | `--help, -h` = `false`<br>`--version, -v` = `false` |
| `skills-seed clear` | 清理可重建运行时数据 | `runtime` | `--help, -h` = `false` |
| `skills-seed clear runtime` | 清理 runtime 目录 | - | `--dry-run` = `false`<br>`--force` = `false`<br>`--help, -h` = `false` |
| `skills-seed cli-skills` | 管理全局 skills-seed CLI Skills | `install`, `uninstall` | `--help, -h` = `false` |
| `skills-seed cli-skills install` | 安装/更新全局 CLI Skills | - | `--help, -h` = `false`<br>`--target, -t` = `auto` |
| `skills-seed cli-skills uninstall` | 卸载全局 CLI Skills | - | `--help, -h` = `false`<br>`--target, -t` = `auto` |
| `skills-seed generate` | 生成 AI Agent skills | `skills` | `--help, -h` = `false` |
| `skills-seed generate skills` | 生成 AI Agent skills | - | `--help, -h` = `false`<br>`--output, -o` = `` |
| `skills-seed hook` | 管理 Git hooks | `install`, `run`, `uninstall` | `--help, -h` = `false` |
| `skills-seed hook install` | 安装 Git pre-commit hook | - | `--help, -h` = `false` |
| `skills-seed hook run` | 手动运行 pre-commit hook | - | `--help, -h` = `false` |
| `skills-seed hook uninstall` | 卸载 Git pre-commit hook | - | `--help, -h` = `false` |
| `skills-seed init` | 初始化 skills-seed 项目 | - | `--agent-model` = ``<br>`--agent` = ``<br>`--help, -h` = `false`<br>`--locale, -l` = ``<br>`--mode` = `project`<br>`--no-interactive` = `false`<br>`--skills-locale` = ``<br>`--skills-name` = ``<br>`--skills` = ``<br>`--workspace` = `false` |
| `skills-seed learn` | 从当前代码学习 | `current` | `--help, -h` = `false` |
| `skills-seed learn current` | 从当前代码学习 | - | `--context-path` = `[]`<br>`--context` = ``<br>`--focus, -f` = `[]`<br>`--force` = `false`<br>`--help, -h` = `false`<br>`--language, -l` = ``<br>`--profile` = `auto` |
| `skills-seed log` | 查看学习变更记录 | - | `--help, -h` = `false` |
| `skills-seed patterns` | 管理已学习的 patterns | `add (--content <description> \| --content-path <path>)`, `compact`, `delete <pattern-id>`, `show [pattern-id]`, `stats`, `update <pattern-id> (--content <description> \| --content-path <path>)` | `--help, -h` = `false` |
| `skills-seed patterns add (--content <description> \| --content-path <path>)` | 用自然语言添加用户自定义模式 | - | `--category, -c` = ``<br>`--content-path` = `[]`<br>`--content` = ``<br>`--help, -h` = `false` |
| `skills-seed patterns compact` | 整理相似 patterns | - | `--category, -c` = ``<br>`--dry-run` = `false`<br>`--help, -h` = `false` |
| `skills-seed patterns delete <pattern-id>` | 删除指定 pattern | - | `--help, -h` = `false` |
| `skills-seed patterns show [pattern-id]` | 查看已学习 pattern 的概览或完整详情 | - | `--format` = `table`<br>`--help, -h` = `false`<br>`--sort` = `updated` |
| `skills-seed patterns stats` | 查看 pattern 质量指标 | - | `--help, -h` = `false` |
| `skills-seed patterns update <pattern-id> (--content <description> \| --content-path <path>)` | 修订指定 pattern | - | `--category, -c` = ``<br>`--content-path` = `[]`<br>`--content` = ``<br>`--help, -h` = `false` |
| `skills-seed preview` | 预览分析输入 | `files` | `--help, -h` = `false` |
| `skills-seed preview files` | 预览将被分析的文件 | - | `--focus, -f` = `[]`<br>`--help, -h` = `false`<br>`--mode` = `full` |
| `skills-seed profile` | 查看项目画像 | `show` | `--help, -h` = `false` |
| `skills-seed profile show` | 显示当前项目画像摘要 | - | `--help, -h` = `false` |
| `skills-seed reset [all\|patterns\|rules\|workflows]...` | 备份并重置 skills-seed 初始化状态 | - | `--help, -h` = `false`<br>`--locale, -l` = ``<br>`--mode` = `project`<br>`--skills-locale` = ``<br>`--workspace` = `false` |
| `skills-seed rule` | 管理用户权威规则 | `show [rule-id]` | `--child` = ``<br>`--content` = ``<br>`--help, -h` = `false`<br>`--name` = ``<br>`--overwrite` = `false`<br>`--path` = `[]`<br>`--project` = `[]` |
| `skills-seed rule show [rule-id]` | 查看已有规则的范围或完整原文 | - | `--child` = ``<br>`--format` = `table`<br>`--help, -h` = `false` |
| `skills-seed sync` | 一键同步 skills | - | `--context-path` = `[]`<br>`--context` = ``<br>`--help, -h` = `false`<br>`--no-interactive` = `false`<br>`--restart` = `false`<br>`--resume` = `false` |
| `skills-seed update` | 从官方发布资产更新 Skills Seed | - | `--help, -h` = `false`<br>`--version` = `latest` |
| `skills-seed workflow` | 管理用户工作流 | `show [workflow-id]` | `--child` = ``<br>`--content` = ``<br>`--help, -h` = `false`<br>`--name` = ``<br>`--overwrite` = `false` |
| `skills-seed workflow show [workflow-id]` | 查看已有工作流的摘要或完整详情 | - | `--child` = ``<br>`--format` = `table`<br>`--help, -h` = `false` |
| `skills-seed workspace` | 管理工作区子项目 | `add .\|project-id-or-path...` | `--help, -h` = `false` |
| `skills-seed workspace add .\|project-id-or-path...` | 向工作区添加子项目 | - | `--help, -h` = `false` |
<!-- COMMAND_TREE_END -->

## 使用约定

### `skills-seed`

#### 命令概述

`skills-seed` 是根命令，用于查看全局帮助、版本信息，并进入各个业务命令。

#### 全局参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--help`, `-h` | `false` | 查看当前命令帮助 |
| `--version`, `-v` | `false` | 输出版本和内置模板 hash |

#### 常用示例

```bash
skills-seed --help
skills-seed --version
skills-seed <command> --help
```

#### 版本输出

```text
skills-seed version <version>
prompt-templates-sha256: <hash>
skills-templates-sha256: <hash>
```

#### 注意事项

1. `skills-seed <command> --help` 可查看任意命令的详细参数。
2. `--version` 输出的是当前二进制版本，文档链接会指向对应 tag，避免与 `main` 分支文档不一致。

### `skills-seed cli-skills`

#### 命令概述

管理全局 `skills-seed-cli` 操作 Skill，用于让 Claude/Codex 等 Agent 知道如何使用 `skills-seed`。它不管理项目生成的业务 Skill。

#### 命令形式

| 命令形式 | 说明 | 常用示例 |
|---|---|---|
| `skills-seed cli-skills install` | 安装或更新全局 CLI 操作 Skill | `skills-seed cli-skills install` |
| `skills-seed cli-skills uninstall` | 卸载由 skills-seed 生成的全局 CLI 操作 Skill | `skills-seed cli-skills uninstall --target codex` |

#### 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--target`, `-t` | `auto` | 目标：`auto`、`claude`、`codex` 或 `all`。`auto` 只处理当前机器检测到可用的 Agent 全局目录。 |
| `--help`, `-h` | `false` | 查看帮助 |

#### 注意事项

1. `init` 交互中只会在检测到当前环境的全局 CLI Skill 未安装或版本/指纹过期时询问是否安装/更新。
2. 需要指定 Claude、Codex 或强制两者都处理时，使用 `cli-skills install|uninstall --target ...`。
3. 安装会完整重建选中的 `skills-seed-cli` 目录；卸载会直接删除该固定目标目录。`generated-by` 仅作为来源元数据。

## 顶层命令

### `skills-seed init`

#### 命令概述

在 Git 仓库中初始化 `.skills-seed/`、默认配置、数据库、项目上下文和 skills 模板。支持单项目模式和 workspace 模式。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed init` | 初始化当前仓库 | `skills-seed init --mode project --agent codex --skills codex --locale zh-CN` | 必须在 Git 仓库根目录执行；已存在 `.skills-seed` 时不覆盖 |

#### `init` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--mode` | `project` | 初始化模式：`project` 单项目，`workspace` 多子项目根仓 |
| `--agent` | 空 | 初始化时写入的执行 Agent engine，例如 `claude` 或 `codex`；留空时使用内置默认值 |
| `--agent-model` | 空 | skills-seed 调用 Agent CLI 时使用的模型名；留空继承本机 Agent CLI 默认配置 |
| `--skills` | 空 | 初始化时写入的 skills 输出类型，例如 `claude` 或 `codex`；留空时使用内置默认值 |
| `--skills-name` | 空 | 生成的 Skill 名称；原样设置入口名称和目标目录名，留空按项目名生成 |
| `--workspace` | `false` | `--mode workspace` 的快捷参数 |
| `--locale`, `-l` | 空 | 工具输出、配置模板与 seed context 模板语言：`zh-CN` 或 `en-US`；留空时使用内置默认值 `zh-CN` |
| `--skills-locale` | 空 | AI 输出、沉淀内容和生成 Skills 语言：`zh-CN` 或 `en-US`；留空时使用内置默认值 `en-US` |
| `--help`, `-h` | `false` | 查看 `init` 帮助 |

#### 常用示例

```bash
skills-seed init --mode project --locale zh-CN
skills-seed init --mode project --agent claude --skills codex --locale zh-CN
skills-seed init --mode project --agent codex --agent-model gpt-5-mini --skills codex --locale zh-CN
skills-seed init --mode workspace --locale zh-CN
skills-seed init --workspace
skills-seed init --workspace --agent codex --skills codex
```

#### 注意事项

1. `--agent` 会设置 `agent.engine`，并确保 `agent.commands` 中存在对应 engine。
2. `--skills` 会设置 `skills.target`，并确保 `skills.paths` 中存在对应 target 的默认输出目录。
3. `--workspace` 会初始化根仓，并同步初始化当前检测到的子仓。
4. 新初始化的子仓会继承根仓 `agent.engine`、`agent.commands`、`agent.model` 和 `skills.target`；Skill 名称及目录按子项目生成，不继承根的 `skills.name`。
5. 已初始化的子仓会跳过；如果子仓 agent 与根仓不同，只提示，不覆盖。
6. 初始化成功后会输出相对 `.skills-seed` 位置和当前版本 tag 对应的 README 文档地址。
7. workspace 子仓发现只认根目录第一层的独立 Git 仓库；标记文件只用于识别类型和语言。

### `skills-seed workspace`

#### 命令概述

管理 workspace 模式下的子项目。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed workspace add .` | 自动检测并添加所有子仓 | `skills-seed workspace add .` | 只适用于 workspace 模式根仓 |
| `skills-seed workspace add <子仓...>` | 只添加指定子仓 | `skills-seed workspace add backend frontend` | 参数可以是检测到的子仓 id 或 path |

#### `workspace` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--help`, `-h` | `false` | 查看 `workspace` 帮助 |

#### 注意事项

1. `workspace add` 使用和 `init --workspace` 相同的发现规则：只有第一层目录中拥有独立 `.git` 的目录会被视为子仓。
2. `go.mod`、`package.json`、安装脚本、Helm/Terraform 等文件只用于识别子仓 `type` 和 `language`。
3. workspace 配置不再提供 `shared`、`contracts`、`infra` 字段；跨项目影响由 `learn current` 分析并沉淀到 workspace profile/spec，生成阶段只消费已沉淀结果。
4. 子仓没有 `.skills-seed` 时，会按 project 模式初始化。
5. 子仓已有 `.skills-seed/config.yaml` 时会跳过并保留原配置。
6. 子仓已有 `.skills-seed` 目录但缺少 `config.yaml` 时会报错，避免覆盖半初始化状态。

### `skills-seed reset`

#### 命令概述

`reset` 有两种明确用途：不带范围时，备份整个 `.skills-seed` 后重新初始化；带范围位置参数时，只重置选中的活跃知识。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed reset` | 备份并重新初始化当前仓库 | `skills-seed reset --mode workspace` | 移动整个 `.skills-seed` 到备份后重新创建配置 |
| `skills-seed reset all` | 重置全部可重置知识 | `skills-seed reset all` | 保留配置、Context 和生成输出；完成后运行 `sync` |
| `skills-seed reset patterns rules workflows` | 只重置指定知识 | `skills-seed reset patterns rules` | 可组合多个范围 |

#### 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--mode` | `project` | 重置后的初始化模式：`project` 或 `workspace` |
| `--workspace` | `false` | `--mode workspace` 的快捷参数 |
| `--locale`, `-l` | 空 | 重置后工具输出与配置模板语言：`zh-CN` 或 `en-US`；留空时使用内置默认值 `zh-CN` |
| `--skills-locale` | 空 | 重置后 AI 输出、沉淀内容和生成 Skills 语言：`zh-CN` 或 `en-US`；留空时使用内置默认值 `en-US` |
| `--help`, `-h` | `false` | 查看 `reset` 帮助 |

#### 常用示例

```bash
skills-seed reset --mode project
skills-seed reset --mode workspace
skills-seed reset --workspace
skills-seed reset all
skills-seed reset patterns rules workflows
```

#### 注意事项

1. 选择性重置会将实际移除的资源移动到 `.skills-seed.backup/<timestamp>/knowledge/`；配置、`.skills-seed/context/` 和已生成 Skills 保持不变。
2. 选择性重置完成后运行 `skills-seed sync`，重新学习并覆盖旧生成输出。
3. 范围 `all`、`patterns`、`rules`、`workflows` 可以组合，只有 `all` 必须单独使用；范围不能与 `--mode`、`--workspace`、`--locale` 或 `--skills-locale` 同时使用。
4. `profile.mode` 在学习或生成后会锁定，不能直接在配置中切换模式；需要切换模式时使用不带范围的完整 `reset`。

### `skills-seed learn`

#### 命令概述

从当前代码库学习编码模式、能力入口和最佳实践，并写入 `.skills-seed` 数据库。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed learn current` | 从当前代码库增量学习 | `skills-seed learn current --focus internal/service --profile skip` | 会比较文件 md5，只学习新增、修改或删除的文件；提示词或模板升级后可加 `--force` 重新学习当前扫描范围 |

#### `learn` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--help`, `-h` | `false` | 查看 `learn` 帮助 |

#### `learn current` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--language`, `-l` | 配置或自动识别 | 项目主要语言 |
| `--focus`, `-f` | 空 | 只学习指定目录或文件；可重复使用，路径必须在项目根目录内 |
| `--profile` | `auto` | 权威规则与项目地图刷新策略：`auto`、`skip`、`refresh` |
| `--context` | 空 | 本次学习的一次性补充说明，会传给 AI Agent，不写入 `.skills-seed/context/` |
| `--context-path` | 空 | 从文件或目录读取本次学习的一次性补充说明；可重复传入，不写入 `.skills-seed/context/` |
| `--help`, `-h` | `false` | 查看 `learn current` 帮助 |

#### `--profile` 取值

| 取值 | 说明 |
|---|---|
| `auto` | 项目地图不存在或权威输入内容版本变化时刷新；否则复用现有地图 |
| `skip` | 只学习源码知识，不刷新权威规则与项目地图 |
| `refresh` | 基于当前输入强制重新提取权威规则并刷新项目地图 |

#### 常用示例

```bash
skills-seed learn current
skills-seed learn current --focus internal/service --profile skip
skills-seed learn current --force --profile refresh
skills-seed learn current -f internal/agent -f internal/service
skills-seed learn current --context "只关注兼容性边界"
skills-seed learn current --context-path .skills-seed/context.md
```

#### 注意事项

1. 首次成功后会记录已分析文件的 md5；没有可学习文件变化时，会跳过源码知识分析、评审和入库。`auto` 仍会比较权威输入内容版本，权威文件变化时继续刷新权威规则与项目地图。
2. 生成的 skills 目录默认排除，包括配置中的 `skills.paths`、`.claude/skills/**` 和 `.agents/skills/**`。
3. workspace 根仓只编排，不把子仓 patterns 写入根仓。
4. project 模式下，`agent.parallelism > 1` 会并发分析独立证据焦点批次；结果仍按学习议程序合并和 checkpoint。
5. workspace 根配置的 `agent.parallelism` 控制子项目并发；每个子项目内证据焦点并发由该子项目配置决定。
6. workspace 子项目完成后，根仓还会继续分析工作区画像、工作区规范并保存关系产物；终端会显示对应进度，避免长耗时 Agent 调用看起来像卡住。
7. workspace 根仓会对工作区关系事实输入记录 md5；当 `workspace.projects`、子项目画像和本次一次性说明未变化，且 workspace profile/spec 已存在时，会跳过根仓画像和规范分析。skills 产物由 `generate skills` 或 `sync` 强制全量重建。
8. 长期有效的项目上下文写入 `.skills-seed/context/`；`--context` 和 `--context-path` 只影响本次命令。
9. `learn current` 会基于文件快照识别新增、修改、删除三类状态；分析完成后按当前作用范围覆盖快照，下一次学习会从新的干净快照计算 diff。
10. 有 focus、diff、sample 或入口文件等边界输入时，源码证据分析和项目地图刷新会使用 `learning.current.structural` 的结构化上下文；默认 `provider: auto` 优先使用 CodeGraph，并在 CodeGraph 命令或索引不可用时回退内嵌 tree-sitter。显式 `provider: codegraph` 会强制使用 CodeGraph。没有边界输入时不会因此全仓扫描。
11. Agent 遇到 429 / 529 / overloaded 等可重试错误时，会按 `agent.retry` 重试；当前进度行会显示 Agent 错误、本次调用耗时和退避等待，终端也会输出包含等待时间和 API 原因的稳定提示，并在下一次调用开始时切换为“第 N 次尝试”。

### `skills-seed generate`

#### 命令概述

生成 AI Agent 相关产物。当前支持 `skills` 子命令。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed generate skills` | 从已核验 patterns、权威规则和项目地图生成 skills | `skills-seed generate skills --output .agents/skills/my-project` | 默认输出到当前 `skills.target` 的 `skills.paths` |

#### `generate` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--help`, `-h` | `false` | 查看 `generate` 帮助 |

#### `generate skills` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--output`, `-o` | 当前 `skills.target` 的 `skills.paths` | 临时指定 skills 输出目录 |
| `--help`, `-h` | `false` | 查看 `generate skills` 帮助 |

#### 常用示例

```bash
skills-seed generate skills
skills-seed generate skills --output .agents/skills/my-project
```

一次性补充说明只在学习阶段使用，例如 `skills-seed learn current --context-path .skills-seed/run-context.md`。`generate skills` 只消费已沉淀的项目地图、已审查 patterns、提取出的权威规则、用户 Rule/Workflow 和 `.skills-seed/context/` 中的长期背景；生成的项目规范只是确定性投影，不是输入事实源。

#### 项目上下文说明

`.skills-seed/context/` 中的文件会与内置 prompt 合并，不会替换内置 prompt。常用持久补充位置：

- `.skills-seed/context/background.md`：代码看不到的业务背景、外部系统和线上事实。
- `.skills-seed/context/terminology.md`：术语、别名、状态名和业务词到代码词的对应关系。
- `.skills-seed/context/workspace.md`：workspace 级上下文，仅 workspace 模式生成。

合并顺序为内置 prompt、`context/background.md`、`context/terminology.md`、`context/workspace.md`，最后追加内置最终输出契约。长期权威规则通过 `skills-seed rule` 维护在 `.skills-seed/rules/`，由生成器投影到 Skill。

#### 生成内容

```text
SKILL.md
agents/
references/
  project-overview.md
  project-spec.md
  modules.md
  business-methods.md
  patterns/*.md
rules/*.md
workflows/*.md
scripts/workflows/
```

Agent 在加载正文前只用 `SKILL.md` frontmatter 的 `name` 和 `description` 判断是否命中。生成器会在 `description` 中保留项目名，并覆盖需求实现、问题排查、业务流程、接口与数据契约、配置、模块边界、Rule/Workflow、生成产物和验证等通用项目任务；正文只负责命中后的最小 reference 路由。`SKILL.md` 也会包含摘要阶段产出的关键洞察（如果 Agent 返回该字段），用于补充入口 skill 中的项目判断依据。

#### 注意事项

1. workspace 模式会先用每个子项目自己的配置重新生成子项目 skill，再生成根仓 workspace skill。
2. 配置指定的输出目录由 skills-seed 独占管理，每次生成都会完整重建；持久自定义内容应写入 `.skills-seed/context/`、patterns 或 workflow。
3. 生成排序主要依据模式质量分和置信度。
4. `generate skills` 不做生成输入指纹校验；显式执行时会删除旧的 skills-seed 生成目录并按当前画像、patterns 和工作流完整重建。

### `skills-seed preview`

#### 命令概述

预览当前配置下 full 或 incremental 分析会进入分析范围的文件，不调用 AI Agent。适合排查 `exclude.paths`、`exclude.gitignore`、focus 路径和文件过滤策略是否符合预期。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed preview files` | 预览将被分析的文件 | `skills-seed preview files --mode incremental --focus internal/service` | 结果会写入 runtime 目录，终端只输出报告目录和文件名，不学习 patterns |

#### `preview` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--help`, `-h` | `false` | 查看 `preview` 帮助 |

#### `preview files` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--mode` | `full` | 预览模式：`full`/`first` 预览全量选择，`incremental`/`current` 预览当前快照 diff |
| `--focus`, `-f` | 空 | 只预览这些路径下的文件；可重复使用 |
| `--help`, `-h` | `false` | 查看 `preview files` 帮助 |

#### 常用示例

```bash
skills-seed preview files
skills-seed preview files --mode full
skills-seed preview files --mode incremental
skills-seed preview files --mode incremental --focus internal/service
```

#### 注意事项

1. `preview files` 和 `learn current` 共用文件过滤策略，可用于确认哪些文件会进入学习分析。
2. `--mode incremental` 会基于当前文件快照展示新增、修改和删除候选；如果还没有快照，结果会接近首次学习范围。
3. 预览结果会写入 `.skills-seed/runtime/preview/files/`，终端只输出报告目录和文件名；确认后可删除该报告文件。

### `skills-seed patterns`

#### 命令概述

管理已学习的 patterns。支持添加用户自定义模式、整理语义相近的 patterns、查看 DB 字段和模式质量指标。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed patterns add --content <内容>` | 用自然语言定义模式，AI 生成结构化 pattern | `skills-seed patterns add --content "对外契约字段保持向后兼容" --category api` | 会调用 AI Agent |
| `skills-seed patterns update <pattern-id> --content <内容>` | 修订指定 pattern，保留原 ID 和归属信息 | `skills-seed patterns update resp-extra-update-logging --content "补充审计日志要求"` | 会调用 AI Agent |
| `skills-seed patterns delete <pattern-id>` | 删除指定 pattern | `skills-seed patterns delete plugin-source-editing-rule` | workspace 根目录会同步删除已关联子项目模式 |
| `skills-seed patterns compact` | 使用本地规则整理相似 patterns | `skills-seed patterns compact --category api --dry-run` | `--dry-run` 可先预览，不写数据库 |
| `skills-seed patterns stats` | 查看模式质量指标 | `skills-seed patterns stats` | 不调用 AI Agent，不修改数据库 |
| `skills-seed patterns show [pattern-id]` | 无参数查看概览，传入 ID 查看完整详情 | `skills-seed patterns show business-create-order --format json` | 不调用 AI Agent，不修改数据库 |

#### `patterns` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--help`, `-h` | `false` | 查看 `patterns` 帮助 |

#### `patterns add` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--category`, `-c` | 空 | 指定模式分类，如 `business`、`api`、`testing`；留空由 AI 自动推断 |
| `--content` | 空 | 用户输入的自然语言模式内容，必填 |
| `--content-path` | 空 | 从文件或目录读取自然语言模式内容或一次性参考材料；可重复传入 |
| `--help`, `-h` | `false` | 查看 `patterns add` 帮助 |

workspace 根目录执行 `patterns add` 时，会先写入根模式库；如果描述中命中子项目 id 或 path，也会同步写入对应子项目模式库。skills 由 `sync` 或显式 `generate skills` 统一重新生成。

#### `patterns update` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--category`, `-c` | 空 | 指定修订后的模式分类；留空沿用现有分类 |
| `--content` | 空 | 用户输入的自然语言修订内容，必填 |
| `--content-path` | 空 | 从文件或目录读取自然语言修订内容或一次性参考材料；可重复传入 |
| `--help`, `-h` | `false` | 查看 `patterns update` 帮助 |

#### `patterns delete` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--help`, `-h` | `false` | 查看 `patterns delete` 帮助 |

#### `patterns compact` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--category`, `-c` | 空 | 只整理指定分类，如 `business`、`api`、`testing`；留空表示全部 |
| `--dry-run` | `false` | 只预览整理结果，不写入数据库 |
| `--help`, `-h` | `false` | 查看 `patterns compact` 帮助 |

#### `patterns stats` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--help`, `-h` | `false` | 查看 `patterns stats` 帮助 |

#### `patterns show` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--format` | `table` | 输出格式：`table` 或 `json` |
| `--help`, `-h` | `false` | 查看 `patterns show` 帮助 |
| `--sort` | `updated` | 概览排序：`updated`、`score` 或 `category` |

#### 常用示例

```bash
skills-seed patterns add --content "对外契约字段保持向后兼容"
skills-seed patterns add --content "错误必须包装上下文" --category error
skills-seed patterns add --content-path docs/pattern-notes.md --category database
skills-seed patterns update resp-extra-update-logging --content "补充响应额外字段更新的审计日志要求"
skills-seed patterns update resp-extra-update-logging --content-path docs/pattern-update.md
skills-seed patterns delete plugin-source-editing-rule
skills-seed patterns compact
skills-seed patterns compact --category api
skills-seed patterns compact --category business --dry-run
skills-seed patterns stats
skills-seed patterns show
skills-seed patterns show --sort score
skills-seed patterns show business-create-order
skills-seed patterns show business-create-order --format json
```

#### 注意事项

1. `patterns compact` 使用本地确定性合并，不调用 Agent。
2. 不确定整理结果时先使用 `--dry-run`。
3. `patterns stats` 展示 specificity、confidence、effective score 等质量指标，方便判断哪些模式更适合进入生成物。
4. `patterns show` 无参数时显示模式概览列表，默认按更新时间倒序；可用 `--sort score` 看高价值规则，`--sort category` 使用分类分组视角。位置列优先使用能力/工具入口的 `code_location`，没有能力入口时回退到模式级 `evidence_locations` 的第一条证据位置。传入 `pattern-id` 时显示单条模式完整详情，包括正/反例、质量指标、workspace 归属、证据位置、能力入口字段、代码位置历史和语言无关符号快照。
5. `patterns stats` 和 `patterns show` 不调用 AI，也不修改数据，但仍需要打开 `.skills-seed/store/project.db`；如果数据库被其他 `skills-seed` 命令占用，CLI 会提示等待当前命令结束或检查残留进程。

### `skills-seed profile`

#### 命令概述

查看项目地图的兼容画像文件。该文件位于 `.skills-seed/store/documents/project-profile.json`，用于生成 `references/project-overview.md`。`learn current --profile auto` 会在地图缺失或权威输入内容变化时刷新权威规则与项目地图；`--profile refresh` 可强制刷新。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed profile show` | 显示当前项目画像摘要 | `skills-seed profile show` | 不调用 AI Agent，不修改数据库 |

#### `profile` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--help`, `-h` | `false` | 查看 `profile` 帮助 |

#### 常用示例

```bash
skills-seed profile show
skills-seed learn current --profile auto
skills-seed learn current --profile refresh
```

#### 注意事项

1. `profile show` 适合快速确认当前画像内容。
2. 权威规则与项目地图刷新属于当前学习流程；`auto` 根据项目地图是否存在及权威输入内容版本确定是否刷新，不依赖 Agent 回显输入边界。
3. 刷新时先提取显式权威规则，再由独立 Agent 重读相同权威来源复核遗漏和候选表述；两阶段都返回完整章节结果。构建脚本、CI 和自动化声明可以描述现有实现，但不会单独授予 Agent 命令执行权限。

### `skills-seed rule`

Rule 是用户维护的权威约束。当前 Agent 会在不改变权威语义和作用域的前提下，结合目标项目上下文整理 `--content`，随后保存到 `.skills-seed/rules/<id>/RULE.md`，再由 `generate skills` 投影到 `rules/<id>.md`。无范围参数时规则只归属当前项目或工作区根；使用 `--child`、`--project` 或 `--path` 可声明其他明确范围。Agent 不从规则原文猜测路径和归属。

```bash
skills-seed rule --name foundation-code --content "未经明确授权不得修改底座代码。" --path "internal/platform/**"
skills-seed rule --name shared-contract --content "修改公共契约前必须确认消费者。" --project backend,frontend
skills-seed rule show foundation-code --format json
```

同名 Rule 默认把新内容与已有正文交给 Agent 合并、去重和优化。已有作用范围会保留，本次显式传入的 `--project` 和 `--path` 会去重追加；使用 `--overwrite` 时才完整替换正文和显式范围。

### `skills-seed workflow`

#### 命令概述

管理可复用的任务工作流。根命令使用当前 Agent 和目标项目上下文整理 `--content`，再保存完整 Markdown 内容；`show` 子命令只读查询已有工作流，不修改文件。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed workflow --name <名称> --content <Markdown>` | 创建工作流，或将新内容合并到同名工作流 | `skills-seed workflow --name release --content "增加发布后的 smoke test"` | Agent 保留已有有效步骤并整理新增内容，不得虚构步骤；名称和内容均为必填 |
| `skills-seed workflow show` | 列出当前作用域的轻量摘要 | `skills-seed workflow show --format json` | 列表不包含完整内容 |
| `skills-seed workflow show <id>` | 查看一个工作流的完整内容 | `skills-seed workflow show release --format json` | 返回指定工作流的完整详情 |
| `skills-seed workflow show --child <id>` | 查询 workspace 子项目工作流 | `skills-seed workflow show --child backend --format json` | `--child` 只能在 workspace 根项目使用 |

#### 参数

| 命令 | 参数 | 默认值 | 说明 |
|---|---|---:|---|
| `workflow` | `--name` | 空 | 工作流名称；更新已有工作流时应显式提供 |
| `workflow` | `--content` | 空 | 工作流原文或草稿；Agent 会整理为可执行 Markdown |
| `workflow` | `--overwrite` | `false` | 完全替换同名工作流；不得在用户未确认时启用 |
| `workflow` | `--child` | 空 | 把工作流写入指定 workspace 子项目 |
| `workflow show` | `--format` | `table` | 输出 `table` 或 `json` |
| `workflow show` | `--child` | 空 | 查询指定 workspace 子项目 |

#### 注意事项

1. 更新已有工作流前，可先执行 `workflow show --format json`，再按需读取匹配详情。
2. 工作流正文不强制固定章节；校验、回滚等部分只在任务需要时保留。
3. 默认合并会保留冲突要求并标记 `To confirm`，不会擅自选择一方；只有 `--overwrite` 会忽略旧正文。
4. 写入完成后运行 `skills-seed generate skills`。

### `skills-seed sync`

#### 命令概述

一键同步：学习当前代码 → 生成 skills。`--context` 和 `--context-path` 只作为本次学习背景传给分析提示词；需要用自然语言补充用户模式时使用 `patterns add/update`。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed sync` | learn current → generate skills | `skills-seed sync` | 优先恢复未完成的 sync 状态；有学习变化时生成 skills |
| `skills-seed sync --context <背景>` | learn current with context → generate skills | `skills-seed sync --context "私有化部署，不是 SaaS"` | 给本次分析提供一次性背景，不写入用户模式 |

#### 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--context` | 空 | 本次学习的额外背景，只影响 learn current 提示词 |
| `--context-path` | 空 | 从文件或目录读取本次学习的额外背景；可重复传入 |
| `--resume` | `false` | 继续上一次未完成的 sync 状态 |
| `--restart` | `false` | 清理本次 sync 的恢复状态并重新开始 |
| `--help`, `-h` | `false` | 查看 `sync` 帮助 |

#### 常用示例

```bash
skills-seed sync
skills-seed sync --context "私有化部署，不是 SaaS"
skills-seed sync --context-path docs/plan.md --context-path docs/specs
skills-seed sync --restart
skills-seed sync --resume
```

#### 注意事项

1. `sync` 默认会先执行 `learn current`；只有本轮学习写入新/更新模式或 workspace 关系产物变化时，才继续执行 `generate skills`。
2. `sync --context` 不会添加用户模式，只影响本次学习分析；需要补充用户模式时使用 `patterns add` 或 `patterns update`。
3. 规范化决策会立即写入 checkpoint；后续本地校验或入库失败时，`sync --resume` 会直接复用已保存候选和议程，只重放尚未完成焦点，不会重新执行候选筛选、议程规划或已完成证据包。
4. 如果续跑只剩后续提交阶段，学习摘要会使用 checkpoint 中已提交的模式数量和原始变更范围，避免误显示为 0 或跳过必要生成。

### `skills-seed update`

更新已安装的 Skills Seed CLI，不涉及任何项目学习状态。

```bash
skills-seed update
skills-seed update --version v0.20.7
```

命令从官方 GitHub Release 下载当前系统和架构对应的发布包，并使用该 Release 的 `checksums.txt` 校验后再替换当前可执行文件。它不需要 Go、不会执行 `go install`，也不会读取或修改项目 `.skills-seed`。

需要能够访问 GitHub Releases，且当前可执行文件所在目录可写。网络请求会继承 `HTTPS_PROXY`、`HTTP_PROXY`、`NO_PROXY` 环境变量；每个请求最长等待 2 分钟。运行时会动态展示查询、下载、校验和安装四个阶段，下载阶段显示已下载大小、总大小和百分比（服务器未提供总大小时显示已下载大小）。Windows 会在当前进程退出后完成替换；请重新启动 CLI 再执行下一条命令。

### `skills-seed hook`

#### 命令概述

管理 Git pre-commit hook。安装后，提交前会打开交互式菜单，可选择同步并生成 skills、只学习或跳过本次。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed hook install` | 写入 `.git/hooks/pre-commit` | `skills-seed hook install` | 提交前打开选择菜单 |
| `skills-seed hook uninstall` | 删除 `.git/hooks/pre-commit` | `skills-seed hook uninstall` | 不删除 `.skills-seed` 数据 |
| `skills-seed hook run` | 手动打开 hook 菜单 | `skills-seed hook run` | 非交互式环境会直接跳过 |

#### `hook` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--help`, `-h` | `false` | 查看 `hook` 帮助 |

#### 子命令参数

| 子命令 | 参数 | 默认值 | 说明 |
|---|---|---:|---|
| `hook install` | `--help`, `-h` | `false` | 查看 `hook install` 帮助 |
| `hook uninstall` | `--help`, `-h` | `false` | 查看 `hook uninstall` 帮助 |
| `hook run` | `--help`, `-h` | `false` | 查看 `hook run` 帮助 |

#### 常用示例

```bash
skills-seed hook install
skills-seed hook uninstall
skills-seed hook run
```

#### 注意事项

1. `hook run` 的默认选项是跳过，避免提交时默认触发高成本 AI 学习。
2. 非交互式终端会直接跳过，不阻塞脚本、IDE 或 Git 自动流程。
3. `hook uninstall` 只移除 hook 文件，不清理学习数据。

### `skills-seed log`

#### 命令概述

查看最近沉淀到项目技能中的变更记录。此命令读取 `.skills-seed/store/journal/` 中的运行记录。输出形式类似 `git log`，不打印详细诊断日志。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed log` | 查看最近学习变更 | `skills-seed log` | 按时间倒序输出所有变更记录 |

#### 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--help`, `-h` | `false` | 查看 `log` 帮助 |

#### 常用示例

```bash
skills-seed log
```

#### 注意事项

1. `sync`、`learn current`、`generate skills` 会写入运行记录。
2. 工作区根目录会合并子项目运行记录后再展示。
3. 详细诊断日志仍保留在 `.skills-seed/runtime/logs/`，用于排障。

### `skills-seed help`

#### 命令概述

查看任意命令路径的帮助信息。该命令由 Cobra 提供。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed help [command]` | 查看指定命令帮助 | `skills-seed help learn current` | 等价于对应命令的 `--help` |

#### 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--help`, `-h` | `false` | 查看 `help` 命令帮助 |

#### 常用示例

```bash
skills-seed help init
skills-seed help learn current
```

#### 注意事项

1. `skills-seed help <command>` 适合查看多级子命令。
2. `skills-seed <command> --help` 与它输出的帮助内容一致。
