# Skills Seed 命令说明

[简体中文](COMMANDS.md) | [English](COMMANDS.EN.md)

本文是完整命令参考。所有命令都支持 `--help`。需要读取 `.skills-seed/config.yaml` 的命令必须先执行 `skills-seed init`。

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
| `skills-seed learn current` | 从当前代码库增量学习 | `skills-seed learn current --focus internal/service --profile skip` | 会比较文件 md5，只学习新增、修改或删除的文件 |
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
| `--limit`, `-n` | `learning.max_commits`，默认 `50` | 最多分析的提交数量 |
| `--since`, `-s` | 空 | 时间范围，例如 `7d`、`30d`、`6m`、`1y` |
| `--batch-size`, `-b` | `learning.batch_size`；未加载配置时为 `10` | 每批提交数量；每批调用一次 Agent 分析并在入库前策展候选模式 |
| `--help`, `-h` | `false` | 查看 `learn history` 帮助 |

#### `--profile` 取值

| 取值 | 说明 |
|---|---|
| `auto` | 首次或全量学习会刷新画像；窄范围改动会尽量跳过 |
| `skip` | 只学习 patterns，不更新画像 |
| `refresh` | 基于当前输入强制刷新画像 |

#### 常用示例

```bash
skills-seed learn current
skills-seed learn current --focus internal/service --profile skip
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
6. workspace 根仓会对工作区关系分析输入记录 md5；当 `workspace.projects`、子项目画像、prompt 模板和本次一次性说明未变化，且 workspace profile/spec 已存在时，会跳过根仓画像和规范分析。
7. 长期有效的提示词补充写入 `.skills-seed/prompts/instructions/<prompt-id>.md`；`--context` 和 `--context-file` 只影响本次命令。
8. `learn current` 会基于文件快照识别新增、修改、删除三类状态；分析完成后按当前作用范围覆盖快照，下一次学习会从新的干净快照计算 diff。
9. 有 focus、diff、sample 或入口文件等边界输入时，学习和项目画像分析会使用 `analysis.structural` 的内嵌 tree-sitter 结构化预扫描；没有边界输入时不会因此全仓扫描。
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

1. workspace 模式会先用每个子项目自己的配置生成子项目 skill，再生成根仓 workspace skill。
2. 已有手写 `SKILL.md` 没有 `generated-by: skills-seed` 标记时默认不会被覆盖。
3. 生成排序会使用 `EffectiveScore*0.6 + normalized(HitCount)*0.3 + Confidence*0.1`；`review stats` 仍只作为观测数据，不直接影响生成。
4. `generate skills` 会对生成输入记录 md5；当项目画像、patterns、命中统计、配置、prompt/skills 模板和输出路径未变化，且输出产物完整时，会跳过 Agent 摘要和文件重写。workspace 根 skill 也会用同样机制跳过未变化的根产物。

### `skills-seed patterns`

#### 命令概述

管理已学习的 patterns。支持添加用户自定义模式、整理语义相近的 patterns、查看 DB 字段、模式质量和 check 命中统计。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed patterns add <描述>` | 用自然语言定义模式，AI 生成结构化 pattern | `skills-seed patterns add "API 路由使用 RESTful 风格" --category api` | 会调用 AI Agent |
| `skills-seed patterns compact` | 调用当前 Agent 策展整理相似 patterns | `skills-seed patterns compact --category api --dry-run` | `--dry-run` 可先预览，不写数据库 |
| `skills-seed patterns stats` | 查看模式质量和 check 命中统计 | `skills-seed patterns stats` | 不调用 AI Agent，不修改数据库 |
| `skills-seed patterns show [pattern-id]` | 查看 pattern 的 DB 字段、时间和代码位置元数据 | `skills-seed patterns show business-create-order --format json` | 不调用 AI Agent，不修改数据库 |

#### `patterns` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--help`, `-h` | `false` | 查看 `patterns` 帮助 |

#### `patterns add` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--category`, `-c` | 空 | 指定模式分类，如 `business`、`api`、`testing`；留空由 AI 自动推断 |
| `--files`, `-f` | 空 | 指定参考文件路径；多个文件需重复传入该参数，AI 会读取文件内容辅助生成 |
| `--context` | 空 | 补充上下文说明，帮助 AI 更准确理解模式 |
| `--help`, `-h` | `false` | 查看 `patterns add` 帮助 |

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

#### 常用示例

```bash
skills-seed patterns add "所有 API 路由使用 RESTful 风格"
skills-seed patterns add "错误必须包装上下文" --category error
skills-seed patterns add "数据库操作使用事务" --files internal/service/user.go --context "项目使用 GORM"
skills-seed patterns compact
skills-seed patterns compact --category api
skills-seed patterns compact --category business --dry-run
skills-seed patterns stats
skills-seed patterns show
skills-seed patterns show business-create-order --format json
```

#### 注意事项

1. `patterns compact` 会调用当前 `agent.engine` 对应的 CLI。
2. 不确定整理结果时先使用 `--dry-run`。
3. `patterns stats` 使用已记录的 check 命中数据，只有执行过带 `PatternID` 的检查后才会出现命中次数。
4. `patterns show` 读取 DB 中已保存字段，可用于排查 `created_at/updated_at`、代码位置状态和语言无关符号快照。
5. `patterns stats` 和 `patterns show` 不调用 AI，也不修改数据，但仍需要打开 `.skills-seed/memory/project.db`；如果数据库被其他 `skills-seed` 命令占用，CLI 会提示等待当前命令结束或检查残留进程。

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

查看或刷新项目画像。项目画像位于 `.skills-seed/memory/project-profile.json`，用于生成 `references/project-overview.md`。

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

一键同步：学习当前代码 → 生成 skills。如果传入 `--add` 参数，则跳过学习步骤，改为用自然语言定义模式后直接生成。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed sync` | 学习当前代码 → generate skills | `skills-seed sync` | 等效于依次执行 `learn current`、`generate skills` |
| `skills-seed sync --add <描述>` | patterns add → generate skills | `skills-seed sync --add "API 路由使用 RESTful 风格"` | 跳过学习，适合补充 AI 未自动发现的模式 |

#### 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--add` | 空 | 用自然语言定义模式描述，传入后执行 patterns add → generate |
| `--category`, `-c` | 空 | `--add` 模式下指定模式分类 |
| `--files`, `-f` | 空 | `--add` 模式下指定参考文件路径；多个文件需重复传入该参数 |
| `--context` | 空 | 补充上下文；普通 `sync` 会传给 `learn current`，`sync --add` 会传给用户模式生成 |
| `--help`, `-h` | `false` | 查看 `sync` 帮助 |

#### 常用示例

```bash
skills-seed sync
skills-seed sync --add "所有 API 路由使用 RESTful 风格"
skills-seed sync --add "错误必须包装上下文" --category error
skills-seed sync --add "数据库操作使用事务" --files internal/service/user.go
skills-seed sync --context "本次只关注兼容性边界"
```

#### 注意事项

1. `sync` 不带 `--add` 时，会先执行 `learn current`，再 `generate skills`；模式策展在学习入库阶段完成。
2. `sync --add` 跳过学习步骤，直接用自然语言定义模式，适合补充 AI 未自动发现的规则。
3. 中间步骤失败会中断后续步骤。

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

1. pre-commit hook 通常使用 `skills-seed check --interactive=false`。
2. 未指定 `--all` 时，只检查 Git 暂存区。
3. 交互式生成修复时，Agent 返回的修复摘要会输出到日志；无法安全完整重写的文件会通过人工审查警告展示，而不会强行写入不完整修复。

### `skills-seed hook`

#### 命令概述

管理 Git pre-commit hook。推荐使用子命令，`--install` 和 `--uninstall` 作为兼容旧用法保留。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed hook install` | 写入 `.git/hooks/pre-commit` | `skills-seed hook install` | 提交前执行 `skills-seed check --interactive=false` |
| `skills-seed hook uninstall` | 删除 `.git/hooks/pre-commit` | `skills-seed hook uninstall` | 不删除 `.skills-seed` 数据 |
| `skills-seed hook run` | 手动按 hook 逻辑检查暂存区 | `skills-seed hook run` | 适合提交前本地验证 |

#### `hook` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--install`, `-i` | `false` | 安装 Git pre-commit hook；推荐改用 `hook install` |
| `--uninstall`, `-u` | `false` | 卸载 Git pre-commit hook；推荐改用 `hook uninstall` |
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
skills-seed hook --install
skills-seed hook --uninstall
```

#### 注意事项

1. 兼容旧用法的 `--install` / `--uninstall` 仍可用，但推荐使用子命令。
2. `hook uninstall` 只移除 hook 文件，不清理学习数据。

### `skills-seed completion`

#### 命令概述

为指定 shell 生成自动补全脚本。该命令由 Cobra 提供，适合安装到本机 shell 配置中。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed completion bash` | 生成 Bash 补全脚本 | `source <(skills-seed completion bash)` | 依赖 bash-completion |
| `skills-seed completion zsh` | 生成 Zsh 补全脚本 | `source <(skills-seed completion zsh)` | 如未启用 completion，需要先执行 `autoload -U compinit; compinit` |
| `skills-seed completion fish` | 生成 Fish 补全脚本 | `skills-seed completion fish \| source` | 可写入 `~/.config/fish/completions/skills-seed.fish` |
| `skills-seed completion powershell` | 生成 PowerShell 补全脚本 | `skills-seed completion powershell \| Out-String \| Invoke-Expression` | 可写入 PowerShell profile |

#### `completion` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--help`, `-h` | `false` | 查看 `completion` 帮助 |

#### 子命令参数

| 子命令 | 参数 | 默认值 | 说明 |
|---|---|---:|---|
| `completion bash` | `--no-descriptions` | `false` | 禁用补全描述 |
| `completion bash` | `--help`, `-h` | `false` | 查看 bash 补全帮助 |
| `completion zsh` | `--no-descriptions` | `false` | 禁用补全描述 |
| `completion zsh` | `--help`, `-h` | `false` | 查看 zsh 补全帮助 |
| `completion fish` | `--no-descriptions` | `false` | 禁用补全描述 |
| `completion fish` | `--help`, `-h` | `false` | 查看 fish 补全帮助 |
| `completion powershell` | `--no-descriptions` | `false` | 禁用补全描述 |
| `completion powershell` | `--help`, `-h` | `false` | 查看 powershell 补全帮助 |

#### 常用示例

```bash
source <(skills-seed completion bash)
source <(skills-seed completion zsh)
skills-seed completion fish | source
skills-seed completion powershell | Out-String | Invoke-Expression
```

#### 注意事项

1. 永久安装补全脚本时，请参考对应 shell 的 `skills-seed completion <shell> --help`。
2. macOS/Linux 的补全脚本安装路径由 shell 和包管理器决定。

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
skills-seed help completion zsh
```

#### 注意事项

1. `skills-seed help <command>` 适合查看多级子命令。
2. `skills-seed <command> --help` 与它输出的帮助内容一致。
