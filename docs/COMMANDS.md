# Skills Seed 命令说明

[简体中文](COMMANDS.md) | [English](COMMANDS.EN.md)

本文是完整命令参考。所有命令都支持 `--help`。需要读取 `.skills-seed/config.yaml` 的命令必须先执行 `skills-seed init`。

## 命令总览

| 阶段 | 命令 | 用途 | 常见入口 |
|---|---|---|---|
| 基础信息 | [`skills-seed`](#skills-seed) | 查看全局帮助、版本和模板 hash | `skills-seed --help` |
| 初始化 | [`skills-seed init`](#skills-seed-init) | 初始化单项目或 workspace 根仓 | `skills-seed init --mode project` |
| Workspace | [`skills-seed workspace`](#skills-seed-workspace) | 添加或管理 workspace 子项目 | `skills-seed workspace add .` |
| 重置 | [`skills-seed reset`](#skills-seed-reset) | 备份并重新初始化 `.skills-seed` | `skills-seed reset --mode workspace` |
| 学习 | [`skills-seed learn`](#skills-seed-learn) | 从当前代码或 Git 历史学习 patterns | `skills-seed learn current` |
| 生成 | [`skills-seed generate`](#skills-seed-generate) | 根据画像和 patterns 生成 skills | `skills-seed generate skills` |
| 预览 | [`skills-seed preview`](#skills-seed-preview) | 预览 full 或 incremental 分析会选中的文件 | `skills-seed preview files` |
| 模式管理 | [`skills-seed patterns`](#skills-seed-patterns) | 添加、删除、整理和查看 patterns | `skills-seed patterns show` |
| 工作流 | [`skills-seed workflow`](#skills-seed-workflow) | 添加或更新用户任务工作流 | `skills-seed workflow --context "..."` |
| 评审统计 | [`skills-seed review`](#skills-seed-review) | 导入评审评论并统计 pattern 防漏效果 | `skills-seed review stats` |
| 项目画像 | [`skills-seed profile`](#skills-seed-profile) | 查看或刷新项目画像 | `skills-seed profile show` |
| 一键同步 | [`skills-seed sync`](#skills-seed-sync) | 学习当前代码并生成 skills | `skills-seed sync` |
| 变更记录 | [`skills-seed log`](#skills-seed-log) | 查看学习变更记录 | `skills-seed log` |
| 提交检查 | [`skills-seed check`](#skills-seed-check) | 检查暂存区或所有跟踪文件 | `skills-seed check` |
| Git Hook | [`skills-seed hook`](#skills-seed-hook) | 安装、卸载或手动运行 pre-commit hook | `skills-seed hook install` |
| 帮助 | [`skills-seed help`](#skills-seed-help) | 查看任意命令路径的帮助 | `skills-seed help learn current` |

## 常见工作流

| 场景 | 推荐命令顺序 | 说明 |
|---|---|---|
| 初始化单项目 | `skills-seed init --mode project` → `skills-seed sync` | 创建配置、学习当前代码并生成 skills |
| 初始化 workspace | `skills-seed init --workspace` → `skills-seed workspace add .` → `skills-seed sync` | 根仓编排子项目学习，再生成子项目和根仓 skills |
| 日常增量更新 | `skills-seed sync` | 学习当前变更，有实际学习变化时生成 skills |
| 只补充一条规则 | `skills-seed sync --pattern "<描述>"` | 用自然语言添加 pattern 后生成 |
| 更新任务工作流 | `skills-seed workflow --context "<说明>"` → `skills-seed generate skills` | `--context` 会先经 Agent 从目标、约束、背景或路径推导标准工作流；未提供 `--name` 时自动生成名称，同名默认合并，完全替换时加 `--overwrite` |
| 提交前更新 | `skills-seed hook install` | 安装 pre-commit hook，在提交前选择同步、只学习或跳过 |
| 查看沉淀变化 | `skills-seed log` | 像 `git log` 一样查看最近学习和生成带来的变更 |
| 排查沉淀结果 | `skills-seed patterns show` → `skills-seed profile show` | 查看已学习 patterns 和项目画像是否符合预期 |

<!-- COMMAND_TREE_START -->
## 自动生成命令索引

> 本节由 Cobra command tree 生成，用于校验命令、子命令和参数默认值是否与 CLI 实现一致；详细场景说明仍以各命令章节为准。

| 命令 | 摘要 | 子命令 | 参数 |
|---|---|---|---|
| `skills-seed` | 为 AI 助手培育项目技能 | `check`, `generate`, `hook`, `init`, `learn`, `log`, `patterns`, `preview`, `profile`, `reset`, `review`, `sync`, `workflow`, `workspace` | `--help, -h` = `false`<br>`--version, -v` = `false` |
| `skills-seed check` | 检查暂存的文件 | - | `--all, -a` = `false`<br>`--help, -h` = `false`<br>`--interactive, -i` = `true` |
| `skills-seed generate` | 生成 AI Agent skills | `skills` | `--help, -h` = `false` |
| `skills-seed generate skills` | 生成 AI Agent skills | - | `--help, -h` = `false`<br>`--no-references` = `false`<br>`--output, -o` = `` |
| `skills-seed hook` | 管理 Git hooks | `install`, `run`, `uninstall` | `--help, -h` = `false` |
| `skills-seed hook install` | 安装 Git pre-commit hook | - | `--help, -h` = `false` |
| `skills-seed hook run` | 手动运行 pre-commit hook | - | `--help, -h` = `false` |
| `skills-seed hook uninstall` | 卸载 Git pre-commit hook | - | `--help, -h` = `false` |
| `skills-seed init` | 初始化 skills-seed 项目 | - | `--agent` = ``<br>`--help, -h` = `false`<br>`--locale, -l` = ``<br>`--mode` = `project`<br>`--no-interactive` = `false`<br>`--skills-locale` = ``<br>`--skills` = ``<br>`--workspace` = `false` |
| `skills-seed learn` | 从 Git 历史学习 | `current`, `history` | `--help, -h` = `false` |
| `skills-seed learn current` | 从当前代码学习 | - | `--context-file` = ``<br>`--context` = ``<br>`--focus, -f` = `[]`<br>`--force` = `false`<br>`--help, -h` = `false`<br>`--language, -l` = ``<br>`--profile` = `auto` |
| `skills-seed learn history` | 从 Git 历史学习 | - | `--batch-size, -b` = `10`<br>`--help, -h` = `false`<br>`--limit, -n` = `50`<br>`--since, -s` = `` |
| `skills-seed log` | 查看学习变更记录 | - | `--help, -h` = `false` |
| `skills-seed patterns` | 管理已学习的 patterns | `add --context <description>`, `compact`, `delete <pattern-id>`, `show [pattern-id]`, `stats`, `update <pattern-id> --context <description>` | `--help, -h` = `false` |
| `skills-seed patterns add --context <description>` | 用自然语言添加用户自定义模式 | - | `--category, -c` = ``<br>`--context` = ``<br>`--files, -f` = `[]`<br>`--help, -h` = `false` |
| `skills-seed patterns compact` | 整理相似 patterns | - | `--ai` = `false`<br>`--category, -c` = ``<br>`--dry-run` = `false`<br>`--help, -h` = `false` |
| `skills-seed patterns delete <pattern-id>` | 删除指定 pattern | - | `--help, -h` = `false` |
| `skills-seed patterns show [pattern-id]` | 查看已学习 pattern 的概览或完整详情 | - | `--format` = `table`<br>`--help, -h` = `false`<br>`--sort` = `updated` |
| `skills-seed patterns stats` | 查看 pattern 质量和 check 命中统计 | - | `--help, -h` = `false` |
| `skills-seed patterns update <pattern-id> --context <description>` | 修订指定 pattern | - | `--category, -c` = ``<br>`--context` = ``<br>`--files, -f` = `[]`<br>`--help, -h` = `false` |
| `skills-seed preview` | 预览分析输入 | `files` | `--help, -h` = `false` |
| `skills-seed preview files` | 预览将被分析的文件 | - | `--focus, -f` = `[]`<br>`--help, -h` = `false`<br>`--limit` = `200`<br>`--mode` = `full` |
| `skills-seed profile` | 查看或刷新项目画像 | `refresh`, `show` | `--help, -h` = `false` |
| `skills-seed profile refresh` | 重新分析项目并保存项目画像 | - | `--help, -h` = `false`<br>`--language, -l` = `` |
| `skills-seed profile show` | 显示当前项目画像摘要 | - | `--help, -h` = `false` |
| `skills-seed reset` | 备份并重置 skills-seed 初始化状态 | - | `--help, -h` = `false`<br>`--locale, -l` = ``<br>`--mode` = `project`<br>`--skills-locale` = ``<br>`--workspace` = `false` |
| `skills-seed review` | 导入评审评论并查看防漏统计 | `import`, `stats` | `--help, -h` = `false` |
| `skills-seed review import` | 从 JSON 文件导入评审评论 | - | `--from-file` = ``<br>`--help, -h` = `false` |
| `skills-seed review stats` | 查看评审评论防漏统计 | - | `--help, -h` = `false`<br>`--line-window` = `3` |
| `skills-seed sync` | 一键同步 skills | - | `--category, -c` = ``<br>`--context` = ``<br>`--files, -f` = `[]`<br>`--help, -h` = `false`<br>`--no-interactive` = `false`<br>`--pattern` = ``<br>`--restart` = `false`<br>`--resume` = `false` |
| `skills-seed workflow` | 添加或更新用户工作流 | - | `--child` = ``<br>`--context` = ``<br>`--help, -h` = `false`<br>`--name` = ``<br>`--overwrite` = `false` |
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

## 顶层命令

### `skills-seed init`

#### 命令概述

在 Git 仓库中初始化 `.skills-seed/`、默认配置、数据库和 prompt / skills 模板。支持单项目模式和 workspace 模式。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed init` | 初始化当前仓库 | `skills-seed init --mode project --agent codex --skills codex --locale zh-CN` | 必须在 Git 仓库根目录执行；已存在 `.skills-seed` 时不覆盖 |

#### `init` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--mode` | `project` | 初始化模式：`project` 单项目，`workspace` 多子项目根仓 |
| `--agent` | 空 | 初始化时写入的执行 Agent engine，例如 `claude` 或 `codex`；留空时使用内置默认值 |
| `--skills` | 空 | 初始化时写入的 skills 输出类型，例如 `claude` 或 `codex`；留空时使用内置默认值 |
| `--workspace` | `false` | `--mode workspace` 的快捷参数 |
| `--locale`, `-l` | 空 | 工具输出与配置模板语言：`zh-CN` 或 `en-US`；留空时使用内置默认值 `zh-CN` |
| `--skills-locale` | 空 | 生成 Skills 和 AI prompt 语言：`zh-CN` 或 `en-US`；留空时使用内置默认值 `en-US` |
| `--help`, `-h` | `false` | 查看 `init` 帮助 |

#### 常用示例

```bash
skills-seed init --mode project --locale zh-CN
skills-seed init --mode project --agent claude --skills codex --locale zh-CN
skills-seed init --mode workspace --locale zh-CN
skills-seed init --workspace
skills-seed init --workspace --agent codex --skills codex
```

#### 注意事项

1. `--agent` 会设置 `agent.engine`，并确保 `agent.commands` 中存在对应 engine。
2. `--skills` 会设置 `skills.target`，并确保 `skills.paths` 中存在对应 target 的默认输出目录。
3. `--workspace` 会初始化根仓，并同步初始化当前检测到的子仓。
4. 新初始化的子仓会继承根仓 `agent.engine`、`agent.commands` 和 `skills.target`、`skills.paths`。
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

备份并重置当前仓库的 `.skills-seed`。旧数据会移动到 `.skills-seed.backup/<timestamp>`，再按指定模式重新初始化。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed reset` | 重置当前仓库初始化状态 | `skills-seed reset --mode workspace` | 会备份旧 `.skills-seed`，但仍建议确认当前工作区状态 |

#### 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--mode` | `project` | 重置后的初始化模式：`project` 或 `workspace` |
| `--workspace` | `false` | `--mode workspace` 的快捷参数 |
| `--locale`, `-l` | 空 | 重置后工具输出与配置模板语言：`zh-CN` 或 `en-US`；留空时使用内置默认值 `zh-CN` |
| `--skills-locale` | 空 | 重置后生成 Skills 和 AI prompt 语言：`zh-CN` 或 `en-US`；留空时使用内置默认值 `en-US` |
| `--help`, `-h` | `false` | 查看 `reset` 帮助 |

#### 常用示例

```bash
skills-seed reset --mode project
skills-seed reset --mode workspace
skills-seed reset --workspace
```

#### 注意事项

1. `reset` 用于重新选择模式或恢复初始化状态。
2. `profile.mode` 在学习或生成后会锁定，不能直接在配置中切换模式。

### `skills-seed learn`

#### 命令概述

从当前代码或 Git 提交历史中学习编码模式、业务方法和最佳实践，并写入 `.skills-seed` 数据库。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed learn current` | 从当前代码库增量学习 | `skills-seed learn current --focus internal/service --profile skip` | 会比较文件 md5，只学习新增、修改或删除的文件；提示词或模板升级后可加 `--force` 重新学习当前扫描范围 |
| `skills-seed learn history` | 从 Git 提交历史学习 | `skills-seed learn history --limit 50 --batch-size 5` | 已学习过的 commit 会跳过 |

#### `learn` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--help`, `-h` | `false` | 查看 `learn` 帮助 |

#### `learn current` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--language`, `-l` | 配置或自动识别 | 项目主要语言 |
| `--focus`, `-f` | 空 | 只学习指定目录或文件；可重复使用，路径必须在项目根目录内 |
| `--profile` | `auto` | 项目画像刷新策略：`auto`、`skip`、`refresh` |
| `--context` | 空 | 本次学习的一次性补充说明，会传给 AI Agent，不写入 `.skills-seed/prompts/` |
| `--context-file` | 空 | 从文件读取本次学习的一次性补充说明，不写入 `.skills-seed/prompts/` |
| `--help`, `-h` | `false` | 查看 `learn current` 帮助 |

#### `learn history` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--limit`, `-n` | `learning.history.max_commits`，默认 `50` | 最多分析的提交数量 |
| `--since`, `-s` | 空 | 时间范围，例如 `7d`、`30d`、`6m`、`1y` |
| `--batch-size`, `-b` | `learning.history.batch_size`；未加载配置时为 `10` | 每批提交数量；每批调用一次 Agent 分析并在入库前策展候选模式 |
| `--help`, `-h` | `false` | 查看 `learn history` 帮助 |

#### `--profile` 取值

| 取值 | 说明 |
|---|---|
| `auto` | 项目画像不存在时自动生成；本次实际写入新模式/更新模式时自动刷新；否则跳过 |
| `skip` | 只学习 patterns，不更新画像 |
| `refresh` | 基于当前输入强制刷新画像 |

#### 常用示例

```bash
skills-seed learn current
skills-seed learn current --focus internal/service --profile skip
skills-seed learn current --force --profile refresh
skills-seed learn current -f internal/agent -f internal/service
skills-seed learn current --context "只关注兼容性边界"
skills-seed learn current --context-file .skills-seed/context.md
skills-seed learn history --limit 50
skills-seed learn history --since 30d
skills-seed learn history --limit 40 --batch-size 5
```

#### 注意事项

1. 首次成功后会记录已分析文件的 md5；没有可学习文件变化时，会跳过 patterns 学习和项目画像刷新。
2. 生成的 skills 目录默认排除，包括配置中的 `skills.paths`、`.claude/skills/**` 和 `.agents/skills/**`。
3. workspace 根仓只编排，不把子仓 patterns 写入根仓。
4. workspace 子项目按 `agent.parallelism` 真并发执行。
5. workspace 子项目完成后，根仓还会继续分析工作区画像、工作区规范并保存关系产物；终端会显示对应进度，避免长耗时 Agent 调用看起来像卡住。
6. workspace 根仓会对工作区关系事实输入记录 md5；当 `workspace.projects`、子项目画像和本次一次性说明未变化，且 workspace profile/spec 已存在时，会跳过根仓画像和规范分析。skills 产物由 `generate skills` 或 `sync` 强制全量重建。
7. 长期有效的提示词补充写入 `.skills-seed/prompts/instructions/<prompt-id>.md`；`--context` 和 `--context-file` 只影响本次命令。
8. `learn current` 会基于文件快照识别新增、修改、删除三类状态；分析完成后按当前作用范围覆盖快照，下一次学习会从新的干净快照计算 diff。
9. 有 focus、diff、sample 或入口文件等边界输入时，学习和项目画像分析会使用 `learning.current.structural` 的内嵌 tree-sitter 结构化预扫描；没有边界输入时不会因此全仓扫描。
10. Agent 遇到 429 / 529 / overloaded 等可重试错误时，会按 `agent.retry` 重试；当前进度行会显示 Agent 错误、本次调用耗时和退避等待，并在下一次调用开始时切换为“第 N 次尝试”。

### `skills-seed generate`

#### 命令概述

生成 AI Agent 相关产物。当前支持 `skills` 子命令。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed generate skills` | 从项目画像和 patterns 生成 skills | `skills-seed generate skills --output .agents/skills/my-project` | 默认输出到当前 `skills.target` 的 `skills.paths` |

#### `generate` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--help`, `-h` | `false` | 查看 `generate` 帮助 |

#### `generate skills` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--output`, `-o` | 当前 `skills.target` 的 `skills.paths` | 临时指定 skills 输出目录 |
| `--no-references` | `false` | 只生成入口 `SKILL.md`，不写入 `references/` 明细文件 |
| `--help`, `-h` | `false` | 查看 `generate skills` 帮助 |

#### 常用示例

```bash
skills-seed generate skills
skills-seed generate skills --output .agents/skills/my-project
```

一次性补充说明只在学习阶段使用，例如 `skills-seed learn current --context-file .skills-seed/context.md`。`generate skills` 只消费已沉淀的项目画像、workspace 画像/spec 和 patterns。

#### Prompt 合并说明

`.skills-seed/prompts/` 中的文件会与内置 prompt 合并，不会替换内置 prompt。常用持久补充位置：

- `.skills-seed/prompts/project/project-profile.md`：项目事实画像。
- `.skills-seed/prompts/project/common.md`：项目通用约束。
- `.skills-seed/prompts/project/<prompt-id>.md`：某个 prompt 的项目级补充。
- `.skills-seed/prompts/workspace/<prompt-id>.md`：workspace 级补充。
- `.skills-seed/prompts/instructions/<prompt-id>.md`：用户补充指令。

合并顺序为内置 prompt、项目画像、项目通用约束、项目级补充、workspace 补充、用户补充指令，最后追加内置最终输出契约。最终输出契约不可由用户文件覆盖，用于保护 JSON / Markdown 输出格式。

#### 生成内容

```text
SKILL.md
agents/
references/
  project-overview.md
  project-spec.md
  patterns/*.md
  examples/*.md
```

`SKILL.md` 会包含摘要阶段产出的关键洞察和改进建议（如果 Agent 返回了这些字段），用于补充入口 skill 中的项目判断依据。

#### 注意事项

1. workspace 模式会先用每个子项目自己的配置重新生成子项目 skill，再生成根仓 workspace skill。
2. 已有手写 `SKILL.md` 没有 `generated-by: skills-seed` 标记时默认不会被覆盖。
3. 生成排序会使用 `EffectiveScore*0.6 + normalized(HitCount)*0.3 + Confidence*0.1`；`review stats` 仍只作为观测数据，不直接影响生成。
4. `generate skills` 不做生成输入指纹校验；显式执行时会删除旧的 skills-seed 生成目录并按当前画像、patterns 和工作流完整重建。

### `skills-seed preview`

#### 命令概述

预览当前配置下 full 或 incremental 分析会选择的文件，不调用 AI Agent。适合排查 `exclude.paths`、`exclude.gitignore`、focus 路径和文件选择策略是否符合预期。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed preview files` | 预览将被分析的文件 | `skills-seed preview files --mode incremental --focus internal/service` | 只输出文件选择结果，不学习 patterns |

#### `preview` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--help`, `-h` | `false` | 查看 `preview` 帮助 |

#### `preview files` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--mode` | `full` | 预览模式：`full`/`first` 预览全量选择，`incremental`/`current` 预览当前快照 diff |
| `--focus`, `-f` | 空 | 只预览这些路径下的文件；可重复使用 |
| `--limit` | `200` | 最多输出的文件数量 |
| `--help`, `-h` | `false` | 查看 `preview files` 帮助 |

#### 常用示例

```bash
skills-seed preview files
skills-seed preview files --mode full
skills-seed preview files --mode incremental
skills-seed preview files --mode incremental --focus internal/service
skills-seed preview files --limit 500
```

#### 注意事项

1. `preview files` 和 `learn current` 共用文件选择策略，可用于确认哪些文件会进入学习分析。
2. `--mode incremental` 会基于当前文件快照展示新增、修改和删除候选；如果还没有快照，结果会接近首次学习范围。
3. 输出中的 skipped 计数可帮助判断文档、排除规则或 Git ignore 是否过滤了预期文件。

### `skills-seed patterns`

#### 命令概述

管理已学习的 patterns。支持添加用户自定义模式、整理语义相近的 patterns、查看 DB 字段、模式质量和 check 命中统计。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed patterns add --context <描述>` | 用自然语言定义模式，AI 生成结构化 pattern | `skills-seed patterns add --context "API 路由使用 RESTful 风格" --category api` | 会调用 AI Agent |
| `skills-seed patterns update <pattern-id> --context <说明>` | 修订指定 pattern，保留原 ID 和归属信息 | `skills-seed patterns update resp-extra-update-logging --context "补充审计日志要求"` | 会调用 AI Agent |
| `skills-seed patterns delete <pattern-id>` | 删除指定 pattern | `skills-seed patterns delete plugin-source-editing-rule` | workspace 根目录会同步删除已关联子项目模式 |
| `skills-seed patterns compact` | 默认使用本地规则整理相似 patterns，显式 `--ai` 时调用 Agent 语义合并 | `skills-seed patterns compact --category api --dry-run` | `--dry-run` 可先预览，不写数据库 |
| `skills-seed patterns stats` | 查看模式质量和 check 命中统计 | `skills-seed patterns stats` | 不调用 AI Agent，不修改数据库 |
| `skills-seed patterns show [pattern-id]` | 无参数查看概览，传入 ID 查看完整详情 | `skills-seed patterns show business-create-order --format json` | 不调用 AI Agent，不修改数据库 |

#### `patterns` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--help`, `-h` | `false` | 查看 `patterns` 帮助 |

#### `patterns add` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--category`, `-c` | 空 | 指定模式分类，如 `business`、`api`、`testing`；留空由 AI 自动推断 |
| `--context` | 空 | 用户输入的自然语言模式描述，必填 |
| `--files`, `-f` | 空 | 指定参考文件或目录路径；多个范围需重复传入该参数，AI 会读取内容辅助生成 |
| `--help`, `-h` | `false` | 查看 `patterns add` 帮助 |

workspace 根目录执行 `patterns add` 时，会先写入根模式库；如果描述中命中子项目 id 或 path，也会同步写入对应子项目模式库。skills 由 `sync` 或显式 `generate skills` 统一重新生成。

#### `patterns update` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--category`, `-c` | 空 | 指定修订后的模式分类；留空沿用现有分类 |
| `--context` | 空 | 用户输入的自然语言修订说明，必填 |
| `--files`, `-f` | 空 | 指定参考文件或目录路径；多个范围需重复传入该参数 |
| `--help`, `-h` | `false` | 查看 `patterns update` 帮助 |

#### `patterns delete` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--help`, `-h` | `false` | 查看 `patterns delete` 帮助 |

#### `patterns compact` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--ai` | `false` | 使用 AI 进行语义合并；默认使用本地确定性合并，不调用 Agent |
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
| `--sort` | `updated` | 概览排序：`updated`、`score`、`hits` 或 `category` |

#### 常用示例

```bash
skills-seed patterns add --context "所有 API 路由使用 RESTful 风格"
skills-seed patterns add --context "错误必须包装上下文" --category error
skills-seed patterns add --context "数据库操作使用事务，项目使用 GORM" --files internal/service
skills-seed patterns update resp-extra-update-logging --context "补充响应额外字段更新的审计日志要求"
skills-seed patterns delete plugin-source-editing-rule
skills-seed patterns compact
skills-seed patterns compact --category api
skills-seed patterns compact --category business --dry-run
skills-seed patterns compact --ai --dry-run
skills-seed patterns stats
skills-seed patterns show
skills-seed patterns show --sort score
skills-seed patterns show business-create-order
skills-seed patterns show business-create-order --format json
```

#### 注意事项

1. `patterns compact` 默认使用本地确定性合并，不调用 Agent；只有传入 `--ai` 时才会调用当前 `agent.engine` 对应的 CLI。
2. 不确定整理结果时先使用 `--dry-run`。
3. `patterns stats` 使用已记录的 check 命中数据，只有执行过带 `PatternID` 的检查后才会出现命中次数。
4. `patterns show` 无参数时显示模式概览列表，默认按更新时间倒序；可用 `--sort score` 看高价值规则，`--sort hits` 看高频命中规则，`--sort category` 使用分类分组视角。位置列优先使用业务/工具方法的 `code_location`，没有业务方法时回退到模式级 `evidence_locations` 的第一条证据位置。传入 `pattern-id` 时显示单条模式完整详情，包括正/反例、质量指标、workspace 归属、证据位置、业务方法字段、代码位置历史和语言无关符号快照。
5. `patterns stats` 和 `patterns show` 不调用 AI，也不修改数据，但仍需要打开 `.skills-seed/store/project.db`；如果数据库被其他 `skills-seed` 命令占用，CLI 会提示等待当前命令结束或检查残留进程。

### `skills-seed review`

#### 命令概述

导入本地代码评审评论，并与已记录的 pattern hits 做文件和行号窗口匹配，统计哪些评论可能已被现有模式提前发现。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed review import --from-file <file>` | 从 JSON 文件导入评审评论数组 | `skills-seed review import --from-file review-comments.json` | 按评论 `id` 覆盖保存，重复导入同一评论不会重复计数 |
| `skills-seed review stats` | 查看评审评论防漏统计 | `skills-seed review stats --line-window 3` | 不调用 AI Agent，不修改数据库 |

#### `review` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--help`, `-h` | `false` | 查看 `review` 帮助 |

#### `review import` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--from-file` | 必填 | 包含评审评论数组的 JSON 文件 |
| `--help`, `-h` | `false` | 查看 `review import` 帮助 |

#### `review stats` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--line-window` | `3` | 匹配已有 pattern hit 时允许的行号距离 |
| `--help`, `-h` | `false` | 查看 `review stats` 帮助 |

#### 导入 JSON 字段

| 字段 | 说明 |
|---|---|
| `id` | 评论唯一 ID |
| `provider` | 来源，如 `local`、`github` 或 `gitlab` |
| `review_id` | 所属评审 ID |
| `commit` | 对应提交 |
| `file` | 文件路径 |
| `line` | 评论行号 |
| `author` | 评论作者 |
| `body` | 评论正文 |
| `resolved` | 评论是否已解决 |
| `created_at` | RFC3339 时间，如 `2026-05-28T09:02:00Z` |

#### 常用示例

```bash
skills-seed review import --from-file review-comments.json
skills-seed review stats
skills-seed review stats --line-window 5
```

#### 注意事项

1. MVP 只支持本地 JSON 导入，不直接连接 GitHub 或 GitLab。
2. `review stats` 依赖已有 `check` 命中记录；没有 pattern hits 时导入评论都会计为遗漏。
3. 匹配规则是相同文件路径且行号距离不超过 `--line-window`。

### `skills-seed profile`

#### 命令概述

查看或刷新项目画像。项目画像位于 `.skills-seed/store/documents/project-profile.json`，用于生成 `references/project-overview.md`。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed profile show` | 显示当前项目画像摘要 | `skills-seed profile show` | 不调用 AI Agent，不修改数据库 |
| `skills-seed profile refresh` | 重新分析项目并覆盖项目画像 | `skills-seed profile refresh --language go` | 不学习 patterns，只刷新画像 |

#### `profile` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--help`, `-h` | `false` | 查看 `profile` 帮助 |

#### `profile refresh` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--language`, `-l` | 配置或自动识别 | 临时指定项目语言 |
| `--help`, `-h` | `false` | 查看 `profile refresh` 帮助 |

#### 常用示例

```bash
skills-seed profile show
skills-seed profile refresh
skills-seed profile refresh --language go
```

#### 注意事项

1. `profile show` 适合快速确认当前画像内容。
2. `profile refresh` 会覆盖现有项目画像，但不会执行 patterns 学习。

### `skills-seed sync`

#### 命令概述

一键同步：学习当前代码 → 生成 skills。`--context` 只作为本次学习背景传给分析提示词；需要用自然语言补充用户模式时使用 `--pattern`。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed sync` | learn current → generate skills | `skills-seed sync` | 优先恢复未完成的 sync 状态；有学习变化时生成 skills |
| `skills-seed sync --context <背景>` | learn current with context → generate skills | `skills-seed sync --context "私有化部署，不是 SaaS"` | 给本次分析提供一次性背景，不写入用户模式 |
| `skills-seed sync --pattern <描述>` | patterns add → generate skills | `skills-seed sync --pattern "API 路由使用 RESTful 风格"` | 适合补充 AI 未自动发现的模式 |

#### 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--category`, `-c` | 空 | `--pattern` 模式下指定模式分类 |
| `--files`, `-f` | 空 | `--pattern` 模式下指定参考文件或目录路径；多个范围需重复传入该参数 |
| `--context` | 空 | 本次学习的额外背景，只影响 learn current 提示词 |
| `--pattern` | 空 | 用户输入的自然语言模式描述；传入后执行 patterns add → generate |
| `--help`, `-h` | `false` | 查看 `sync` 帮助 |

#### 常用示例

```bash
skills-seed sync
skills-seed sync --context "私有化部署，不是 SaaS"
skills-seed sync --pattern "所有 API 路由使用 RESTful 风格"
skills-seed sync --pattern "错误必须包装上下文" --category error
skills-seed sync --pattern "数据库操作使用事务" --files internal/service
```

#### 注意事项

1. `sync` 默认会先执行 `learn current`；只有本轮学习写入新/更新模式或 workspace 关系产物变化时，才继续执行 `generate skills`。
2. `sync --context` 不会添加用户模式，只影响本次学习分析。
3. `sync --pattern` 跳过学习步骤，直接用自然语言定义模式，适合补充 AI 未自动发现的规则。
4. 中间步骤失败会中断后续步骤。

### `skills-seed check`

#### 命令概述

检查暂存区或所有 Git 跟踪文件是否符合已学习 patterns，并可交互式处理问题。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed check` | 检查暂存区或所有 Git 跟踪文件 | `skills-seed check --all --interactive=false` | 默认只检查暂存文件 |

#### 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--interactive`, `-i` | `true` | 启用交互式修复确认；hook 中通常使用 `false` |
| `--all`, `-a` | `false` | 检查所有 Git 跟踪文件；默认只检查暂存文件 |
| `--help`, `-h` | `false` | 查看 `check` 帮助 |

#### 常用示例

```bash
skills-seed check
skills-seed check --all
skills-seed check --interactive=false
```

#### 注意事项

1. 需要纯检查时可直接运行 `skills-seed check --interactive=false`。
2. 未指定 `--all` 时，只检查 Git 暂存区。
3. 交互式生成修复时，Agent 返回的修复摘要会输出到日志；无法安全完整重写的文件会通过人工审查警告展示，而不会强行写入不完整修复。

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

查看最近沉淀到项目技能中的变更记录。此命令读取 `.skills-seed/store/documents/change-log.json`，输出形式类似 `git log`，不打印详细诊断日志。

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

1. `sync`、`learn current`、`generate skills` 会写入学习变更记录。
2. 详细诊断日志仍保留在 `.skills-seed/runtime/logs/`，用于排障。

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
