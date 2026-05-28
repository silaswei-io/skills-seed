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
| `skills-seed init` | 初始化当前仓库 | `skills-seed init --mode project --agent codex --locale zh-CN` | 必须在 Git 仓库根目录执行；已存在 `.skills-seed` 时不覆盖 |
| `skills-seed init children` | 按根仓 `workspace.projects` 初始化子项目 | `skills-seed init children --locale zh-CN` | 只能在 `project.mode: "workspace"` 的根仓执行 |

#### `init` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--mode` | `project` | 初始化模式：`project` 单项目，`workspace` 多子项目根仓 |
| `--agent` | `claude` | 初始化时写入的 Agent provider，例如 `claude` 或 `codex` |
| `--workspace` | `false` | `--mode workspace` 的快捷参数 |
| `--children` | `false` | 与 workspace 模式一起使用，初始化根仓后同步初始化缺失 `.skills-seed` 的子项目 |
| `--locale`, `-l` | 自动检测 | 配置文件语言：`zh-CN` 或 `en-US` |
| `--help`, `-h` | `false` | 查看 `init` 帮助 |

#### `init children` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--locale`, `-l` | 根仓配置语言 | 子项目配置文件语言：`zh-CN` 或 `en-US` |
| `--help`, `-h` | `false` | 查看 `init children` 帮助 |

#### 常用示例

```bash
skills-seed init --mode project --locale zh-CN
skills-seed init --mode project --agent codex --locale zh-CN
skills-seed init --mode workspace --locale zh-CN
skills-seed init --workspace
skills-seed init --workspace --children --agent codex
skills-seed init children
```

#### 注意事项

1. `--agent` 会设置 `agent.provider`，并确保 `agent.commands` 和 `output.skills_paths` 中存在对应 provider。
2. `--workspace --children` 会先初始化根仓，再按 `workspace.projects` 初始化缺失 `.skills-seed` 的子仓。
3. 新初始化的子仓会继承根仓 `agent.provider`、`agent.commands` 和 `output.skills_paths`。
4. 已初始化的子仓会跳过；如果子仓 agent 与根仓不同，只提示，不覆盖。
5. 初始化成功后会输出相对 `.skills-seed` 位置和当前版本 tag 对应的 README 文档地址。

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
| `--locale`, `-l` | 自动检测 | 重置后配置文件语言：`zh-CN` 或 `en-US` |
| `--help`, `-h` | `false` | 查看 `reset` 帮助 |

#### 常用示例

```bash
skills-seed reset --mode project
skills-seed reset --mode workspace
skills-seed reset --workspace
```

#### 注意事项

1. `reset` 用于重新选择模式或恢复初始化状态。
2. `project.mode` 在学习或生成后会锁定，不能直接在配置中切换模式。

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
| `--context` | 空 | 本次学习的补充说明，会传给 AI Agent |
| `--context-file` | 空 | 从文件读取本次学习的补充说明 |
| `--help`, `-h` | `false` | 查看 `learn current` 帮助 |

#### `learn history` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--limit`, `-n` | `learning.max_commits`，默认 `50` | 最多分析的提交数量 |
| `--since`, `-s` | 空 | 时间范围，例如 `7d`、`30d`、`6m`、`1y` |
| `--batch-size`, `-b` | `learning.batch_size`，默认 `5` | 每批提交数量；每批会合并后调用一次 Agent |
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
2. 生成的 skills 目录默认排除，包括配置中的 `output.skills_paths`、`.claude/skills/**` 和 `.agents/skills/**`。
3. workspace 根仓只编排，不把子仓 patterns 写入根仓。
4. `workspace.init_children: true` 时，缺失 `.skills-seed` 的子项目会先按根仓 agent 配置初始化。
5. workspace 子项目按 `agent.parallelism` 真并发执行。

### `skills-seed generate-skills`

#### 命令概述

从项目画像和 patterns 生成当前 provider 的 skills。生成阶段会调用 `agent.provider` 对应的 CLI 做摘要合并和润色。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed generate-skills` | 生成当前 provider 的 skills | `skills-seed generate-skills --output .agents/skills/my-project` | 默认输出到当前 provider 的 `output.skills_paths` |

#### 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--output`, `-o` | 当前 provider 的 `output.skills_paths` | 临时指定 skills 输出目录 |
| `--merge`, `-m` | `false` | 生成前合并相似 patterns；已不推荐，改用 `skills-seed patterns merge` |
| `--context` | 空 | 本次生成的补充说明，会传给 AI Agent |
| `--context-file` | 空 | 从文件读取本次生成的补充说明 |
| `--help`, `-h` | `false` | 查看 `generate-skills` 帮助 |

#### 常用示例

```bash
skills-seed generate-skills
skills-seed generate-skills --output .agents/skills/my-project
skills-seed generate-skills --context "重点保留 API 兼容性约束"
skills-seed generate-skills --context-file .skills-seed/context.md
```

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

#### 注意事项

1. workspace 模式会先用每个子项目自己的配置生成子项目 skill，再生成根仓 workspace skill。
2. 已有手写 `SKILL.md` 没有 `generated-by: skills-seed` 标记时默认不会被覆盖。
3. `--merge` 是兼容旧用法，推荐先单独执行 `skills-seed patterns merge`。

### `skills-seed patterns`

#### 命令概述

管理已学习的 patterns。目前主要用于合并语义相近的 patterns，减少重复规则。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed patterns merge` | 调用当前 Agent 合并相似 patterns | `skills-seed patterns merge --category api --dry-run` | `--dry-run` 可先预览，不写数据库 |

#### `patterns` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--help`, `-h` | `false` | 查看 `patterns` 帮助 |

#### `patterns merge` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--category`, `-c` | 空 | 只合并指定分类，如 `business`、`api`、`testing`；留空表示全部 |
| `--dry-run` | `false` | 只预览合并结果，不写入数据库 |
| `--help`, `-h` | `false` | 查看 `patterns merge` 帮助 |

#### 常用示例

```bash
skills-seed patterns merge
skills-seed patterns merge --category api
skills-seed patterns merge --category business --dry-run
```

#### 注意事项

1. 合并会调用当前 `agent.provider` 对应的 CLI。
2. 不确定合并结果时先使用 `--dry-run`。

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

### `skills-seed view`

#### 命令概述

查看 `.skills-seed` 中已学习或生成的内容。目前主要用于查看已学习 patterns。

#### 命令形式

| 命令形式 | 说明 | 常用示例 | 注意事项 |
|---|---|---|---|
| `skills-seed view patterns` | 按分类查看已学习 patterns | `skills-seed view patterns --category testing` | 不修改数据库 |

#### `view` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--help`, `-h` | `false` | 查看 `view` 帮助 |

#### `view patterns` 参数

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--category`, `-c` | 空 | 按分类过滤：`naming`、`error`、`structure`、`concurrency`、`testing`、`business`、`api`、`database`、`utils`、`middleware`、`config` |
| `--help`, `-h` | `false` | 查看 `view patterns` 帮助 |

#### 常用示例

```bash
skills-seed view patterns
skills-seed view patterns --category testing
```

#### 注意事项

1. 不指定 `--category` 时显示全部分类。
2. 该命令只读，不会触发学习、合并或生成。

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
