# Skills Seed 配置说明

## 本地扫描与按需语义补充

默认使用本地 CodeGraph 结构扫描。`learn current` 和 `sync` 使用随发行包内置的 CodeGraph 1.6.2 更新本地结构索引，结构扫描不调用 AI；程序只为重要语义缺口自动调用分析和独立审核，不再进行全仓 AI 议程规划。源码开发构建请先执行 `make bundle`；也可用 `SKILLS_SEED_CODEGRAPH_HOME` 指定官方平台包目录。发行二进制首次使用时解压组件到用户缓存，无需全局 Node/npm，也不在运行时下载组件。

```bash
skills-seed sync
skills-seed codegraph query "HTTP"
# 也可以主动指定需要更深入判断的范围：
skills-seed learn current --semantic --focus internal/order --context "确认取消订单的状态边界"
skills-seed generate skills
```

生成的 `references/code-navigation.md` 按目录链接到符号入口，并要求开发 Agent 先检索、核对签名与调用示例，再决定复用、扩展或新增。目录是结构导航，不是自动推断的业务域。`codegraph callers/callees/impact` 使用查询返回的符号 ID；`codegraph trace <from-id> <to-id>` 查询静态调用路径。动态调用与跨服务关系可能不完整，空结果不能证明没有现成能力。

索引保存到标准 `.codegraph/`，与其他 CodeGraph 工具复用。skills-seed 只在该目录写入自己的 `navigation.json` 和锁文件；扫描按入选路径更新索引，不因范围收缩而重建共享数据库。源码选择沿用排除规则，缓存与生成 Skill 不参与反向学习。未被 SDK 索引的文件会显式列出。解析失败会报错，不冒充扫描成功。

本地结构扫描本身不更新语义学习指纹；只有自动补充完成分析、审核和入库后，才提交该范围的语义指纹。未完成的检查点保留。生成时，源码证据指纹变化的旧 Pattern 暂不投影；原知识仍保留。项目导航由本地索引和已保存知识确定性生成；源码证据变化时只停止投影失效的旧业务 Pattern。Rule/Workflow 保持原有维护方式，权威文件应由消费 Skill 的 Agent 按实际路径直接阅读。

工作区默认从显式配置生成项目路由，不通过 AI 猜测跨项目业务关系。子项目按同样策略自动补充；主动深挖时进入子项目使用 `--semantic --focus`。`--context` 传入自动补充任务；`--focus` 只限制本地扫描范围。`--force` 重新确认当前索引并触发产物刷新。

自动补充优先检查已有知识的变更证据，其次从本地 CodeGraph 调用图选择语义入口。同一调用链的下游能力保留在本地导航，不重复发起逐文件 AI 问题；无法由图确认的独立入口仍待分析。多个完整证据批次才使用紧凑结构事实并行预筛选，失败或不确定时保留候选。源码已删除的旧知识由生成阶段停止投影。候选按重要性顺序和完整证据容量动态组批，已有知识的关联证据必须保留；证据包上限 64 KiB、完整提示词上限 96 KiB。超限不截断，异常批次保留待补充。正常 `sync` 持续处理全部候选，不设置项目级总时长；单次 Agent 调用仍受 `agent.timeout`、重试次数和输入大小保护。AI 不可用、失败、超时或明确返回证据不足时，保留对应批次并继续其他独立批次，原因及待补充路径写入 `.skills-seed/runtime/local-supplement.json`。`mode` 和 `structural` 仅控制显式语义补充；默认流程不再有 AI 议程配置。

[简体中文](CONFIGURATION.md) | [English](CONFIGURATION.EN.md)

配置文件位于 `.skills-seed/config.yaml`。`skills-seed init` 会按当前项目生成默认配置；大多数路径都相对项目根目录或 `.skills-seed` 目录，具体以字段说明为准。

## 0.8.x 配置结构

0.8.x 继续沿用 0.7.x 配置结构，不保留旧字段兼容：

- 顶层 `project` 改名为 `profile`，表示当前配置文件所属项目或工作区本身，不表示 `project` 运行模式。
- `workspace` 下只保留 `projects`，不再提供 `shared`、`contracts`、`infra` 给用户手填。
- workspace 公共库、契约和基础设施路由由显式项目配置和本地 CodeGraph 事实确定，不通过额外 AI 画像调用猜测；生成阶段只消费已沉淀结果。
- workspace 根配置的 `profile.language` 默认留空，因为一个工作区可以包含多种语言子项目。
- `analysis.codegraph` 已移除，结构化上下文与符号校验统一配置在 `learning.current.structural`；默认 `provider: auto` 优先使用 CodeGraph，并在 CodeGraph 不可用时回退内嵌 tree-sitter。

## 配置示例

### 默认结构

```yaml
profile:
  name: "your-project"
  mode: "project"
  language: ""
  locale: "zh-CN"
  git_remote: ""
  root_path: ""
  initialized_at: ""

workspace:
  projects: []

agent:
  engine: "claude"
  commands:
    claude: "claude"
    codex: "codex"
  timeout: 1800
  max_turns: 80
  allow_user_plugins: false
  parallelism: 0
  model: ""
  retry:
    max_retries: 3
    initial_interval: 15
    max_interval: 120

learning:
  current:
    mode: "normal"
    pattern_admission:
      min_confidence: 0.75
      min_single_evidence_confidence: 0.85
    agenda:
    structural:
      enabled: true
      provider: "auto"
      max_symbols: 30
      max_file_size: 512
skills:
  target: "claude"
  locale: "en-US"
  paths:
    claude: ".claude/skills/<project-name>-dev"
    codex: ".agents/skills/<project-name>-dev"

logging:
  enabled: false
  level: "DEBUG"
  logs_path: "runtime/logs"
  max_log_files: 30

exclude:
  gitignore: true
  paths:
    - ".*"
    - "vendor/**"
    - "node_modules/**"
    - "dist/**"
    - "build/**"
    - "out/**"
    - "target/**"
    - "coverage/**"
    - ".cache/**"
    - "tmp/**"
    - "temp/**"
    - "*.log"
    - "*.tmp"
    - "*.bak"
    - "*.swp"
    - "*.zip"
    - "*.tar"
    - "*.tar.gz"
    - "*.tgz"
    - "*.rar"
    - "*.7z"
    - "*.png"
    - "*.jpg"
    - "*.jpeg"
    - "*.gif"
    - "*.webp"
    - "*.ico"
    - "*.pdf"
    - "*.mp4"
    - "*.mov"
```

## 配置项

### `profile`

`profile` 描述当前配置文件所属的项目或工作区本身，不表示 `project` 运行模式

#### 字段

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `name` | 当前目录名 | 项目名称，init 时自动填充 |
| `mode` | `project` | 初始化模式：`project` 单项目，`workspace` 多子项目工作区 |
| `language` | 自动识别或空 | 项目主要语言；init 识别不到时留空，可按项目设置 |
| `locale` | `zh-CN` | 工具输出、配置模板与 seed context 模板语言 |
| `git_remote` | 自动填充或空 | Git 远程仓库地址 |
| `root_path` | 当前项目绝对路径 | init 时写入，供运行时定位项目根目录 |
| `initialized_at` | init 时间 | 初始化时间 |

#### 说明

1. `mode` 在开始学习或生成 skills 后会被锁定，不能直接在 `project` 和 `workspace` 模式之间切换。
2. 需要重新选择模式时，使用 `skills-seed reset --mode project` 或 `skills-seed reset --mode workspace`。
3. `locale` 支持 `zh-CN` 和 `en-US`。

### `workspace`

#### 字段

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `projects` | `[]` | 子项目列表；workspace init 会尝试发现第一层目录中的项目 |

#### `projects` 项目字段

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `id` | 目录名规范化 | 子项目唯一标识 |
| `path` | 发现到的相对路径 | 子项目路径，相对 workspace 根目录 |
| `type` | 自动识别 | 子项目类型，例如应用、库、共享组件、基础设施或契约项目 |
| `language` | 自动识别 | 子项目主要语言 |

#### 行为

1. `skills-seed init --workspace` 会初始化根仓，并同步初始化当时检测到的子项目。
2. 后续新增或拷入 workspace 的子项目使用 `skills-seed workspace add .` 自动检测添加，或使用 `skills-seed workspace add <子项目>` 指定添加。
3. 子项目已有 `.skills-seed/config.yaml` 时不覆盖；如果子项目 agent 和根仓不同，只提示并保留子项目配置。
4. 子项目已有 `.skills-seed` 目录但缺少 `config.yaml` 时会报错，避免覆盖半初始化状态。
5. 只有 workspace 根目录第一层且拥有独立 `.git` 的目录会被识别为子项目。
6. `go.mod`、`package.json`、安装脚本、Helm/Terraform 等标记只用于识别 `type` 和 `language`，不再决定目录是否是项目。

### `learning.current`

`learning.current` 控制 `learn current` 的文件范围和结构化上下文。

#### 字段

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `mode` | `normal` | 学习策略均以高精准率准入：`fast` 缩小证据探索范围；`normal` 为默认；`deep` 扩大证据探索范围，但不降低模式准入标准 |
| `structural.enabled` | `true` | 是否启用结构化上下文；即使开启，也只会在存在焦点、源码样本或入口文件时运行 |
| `structural.provider` | `auto` | 结构化上下文与符号校验来源：`auto` 优先 CodeGraph 并在不可用时回退 tree-sitter；`codegraph` 强制使用 CodeGraph；`treesitter` 显式选择内嵌 parser |
| `structural.max_symbols` | `30` | 输出到结构化上下文的最大符号数 |
| `structural.max_file_size` | `512` | 单个源码文件大小上限，单位 KB；超过时跳过该文件 |

#### `structural`

结构化 provider 同时用于给 Agent 提供符号、导入、入口点和模块线索，以及校验 AI 返回的源码符号。默认 `provider: auto` 优先使用 CodeGraph；如果目标项目未初始化会自动执行初始化，索引同步或状态检查异常时会尝试自动修复；当 CodeGraph 命令或索引不可用时，会回退到内嵌 tree-sitter。`provider: codegraph` 表示强制使用 CodeGraph，失败会直接暴露错误；`provider: treesitter` 表示只使用内嵌 parser。

0.7.1 起，结构化预扫描、`learn current` 和 `preview` 共用同一套文件过滤策略：默认只纳入源码、构建配置和依赖配置；文档、生成产物、全局 `exclude` 命中的路径以及已生成 Skills 输出目录会被跳过。

0.9.0 起，项目结构摘要、样例文件收集和结构化预扫描都统一使用同一套配置化文件过滤策略。除 `.git`、`.skills-seed` 和已配置的 skills 输出目录等内置安全边界外，不再在 analyzer 内额外维护目录名关键字；需要排除依赖、构建产物或项目自定义目录时，应写入 `exclude`。

当前版本中，`learn current` 在本地文件过滤后会保留全部范围内候选，不再用路径词表或模型猜测删减源码文件。候选路径、焦点文件和结构化上下文等大块输入会写入 runtime 输入文件，prompt 通过路径引用它们。

文件过滤策略默认叠加 Git ignore 规则；如需分析被 `.gitignore` 忽略的文件，可将 `exclude.gitignore` 设为 `false`。增量状态由持久化文件指纹判定，不保存源码快照；AI 对照当前源码证据与已保存知识判断语义变化。打开已有项目时会移除旧版本留下的 `.skills-seed/cache/snapshots/`。

#### 建议

1. 大多数项目保持默认值即可；没有边界输入时不会运行结构化上下文。
2. 明确不需要结构化上下文时，把 `structural.enabled` 设为 `false`。
3. 大型仓库显式使用 tree-sitter 时可降低 `structural.max_file_size`，避免解析生成文件、bundle 或异常大文件。
4. 结构化上下文只消费已有边界输入，不在没有 seed 时全仓扫描。

### Prompt 运行时调试

项目背景层从 `.skills-seed/context/` 读取，渲染时会过滤默认元数据、空脚手架和未填写占位内容，只保留用户实际写入的背景信息。它不能创建或覆盖 Rule、命令权限、Workflow 步骤或源码证据。

渲染后的 prompt 默认保存在 `.skills-seed/runtime/rendered-prompts/`，并生成同名 `.manifest.json`。manifest 会记录内置模板、context 片段和输出契约等片段是否参与合并、原始长度和最终长度，方便排查 Agent 实际收到的上下文。候选文件、焦点文件和结构化上下文等大块输入会优先保存到 `.skills-seed/runtime/` 下的 prompt input 目录，渲染后的 prompt 通过路径引用它们。最终输出契约由独立的 append 模板追加，并对 JSON 型 prompt 强制要求最终响应只能是单个可解析 JSON 对象，同时要求相同输入下保持语义稳定和确定性排序。

0.10.5 起，`learn current` 焦点分析不会再把已有模式库写入每个焦点 prompt；如果需要查看已有模式，请读取本地模式库或使用 `patterns show` / `patterns stats`。Claude 和 Codex 调用统一使用 DTO 生成的 Schema 约束原生结构化输出；全量与增量焦点分析会把本批焦点 ID 传入实际运行时 Schema，同时约束焦点枚举与结果数量。解析层使用 `jsonrepair-go` 修复 JSON 语法，修复结果仍必须通过严格 DTO 解码，不会放行未知字段或错误结构。

0.11.0 起，`learning.current.mode` 可设置为 `fast`、`normal` 或 `deep`，用于在学习速度和模式覆盖质量之间选择策略；该配置会进入续跑状态指纹。生成 skills 时会输出相关参考路由、重要性分层和分组入口索引，并在渲染前校验证据路径是否存在。测试、部署和验收流程不从源码学习，其步骤、命令和顺序由用户维护的 Workflow 提供，并随最终 Skills 一起生成。

学习焦点没有可配置分类。规划阶段根据源码责任、能力、资源动作、状态转换、契约、集成和横切机制，自动形成验证未来开发决策所需的最小证据集。焦点只服务于本次取证，不会成为 Workflow、工作步骤或生成 Skill 的固定分类。模型输出解析会额外修复证据行号范围表达式，将 `"line": 29-43` 这类非法 JSON 归一为单个行号。

当前版本按证据容量动态组批，相关焦点可以共用一次分析调用；独立批次并发受 `agent.parallelism` 控制。分析结果逐项保存后，相应焦点的独立知识审查即可与其他批次的分析或审查并行执行；同一运行的 checkpoint 由主流程逐项保存。增量分析提出的候选若未通过本地证据准入，该焦点在没有其他合格决策时会降级为唯一的 `no_change` 回执，区分“已分析但无可入库模式”和“Agent 遗漏焦点”。生成 skills 时，低频或局部证据不会进入强约束层，避免把偶发现象误写成必须遵守的项目标准。

独立审查中的能力入口以完整 `business_method` 表示设置或替换，省略即移除，不再使用与对象重复表达状态的判定字段。审查 Agent 必须打开候选证据直接调用或引用的精确定义，记录完整声明、项目相对位置、前置条件和结果/错误语义；本地层仅校验结构化契约和路径安全，不以调用方与定义文件必须相同来替代语义审查。

候选准备只做范围、排除规则和路径规范化，不根据文件名、目录名、语言或框架词汇判断知识价值。所有范围内候选都进入议程规划，规划结果必须用焦点或带理由的跳过回执完整覆盖输入路径。后续源码分析与独立知识审查负责语义判断；本地代码只校验证据、范围、结构化契约和覆盖完整性。

初始化交互中的“Agent 总并发数”会写入 `agent.parallelism`。workspace 根配置用它控制子项目并发和共享 Agent 调用额度；普通 project 配置用它控制独立证据批次的并发。当前代码学习本地规划证据批次，只在关键语义缺口处调用 Agent。`learning.current.pattern_admission` 控制候选进入合并与入库链路所需的置信度。

0.8.0 起，Agent 输出默认单独保存在 `.skills-seed/runtime/agent-outputs/`，包含最终内容、原始 CLI 输出、stderr 和 manifest。运行日志只记录长度和归档路径，不再输出模型回复预览或 stdout/stderr 明文。0.10.3 起，最终内容如果是合法 JSON，会在 `.md` 归档中格式化为可读的 `json` 代码块。

可重试的 Agent 调用会在终端和运行日志中以 `ERROR` 记录本地化失败原因、调用耗时和下次重试等待时间；原始 stdout 与 stderr 仍通过归档路径提供，避免把大段模型输出写入日志。

0.9.6 起，`.skills-seed/runtime` 下的调试记录使用 `YYYYMMDD-HHMMSS[-NNN]-<kind>-<name>` 文件名前缀；同一秒内生成多个 runtime ID 时追加递增序号避免覆盖。`rendered-prompts/` 与对应的 `agent-outputs/` 共享同一个日期时间 ID 和语义名，Agent 输出文件只额外包含 Agent 名称，方便把同一次调用中的 prompt 和输出一一对应。0.10.3 起，合法 JSON 输出会在 `.md` 归档中格式化为可读的 `json` 代码块。

模式库入库前会执行候选规范化。当前 `learn current` 的候选模式来自证据包分析结果，并先经过独立知识审查；本地服务再按配置的证据与置信度门槛准入，AI 只对新旧模式提出语义合并建议。本地服务负责校验来源 ID、从来源传播受控知识标志、确定性处理重复归属、补回遗漏的合格候选并一次性写入。学习摘要中的“发现候选模式”表示本轮审查后的候选数，“规范化写入”表示包含相关既有 Pattern 在内的最终 upsert 数，因此后者可能大于前者。本轮源码候选统一以自己的 Pattern ID 建立输入身份，既有 `MergedFrom` 只表示历史输出谱系，不参与本轮来源归属；同 ID 当前候选表示该 Pattern 的当前有效版本，会覆盖已有记录后再参与校验和水合，不同 ID 才作为独立来源判断是否允许合并。重复校验已水合结果时，无歧义的历史谱系只回指其当前有效 Pattern 并按主 ID 去重；同一谱系存在多个所有者时仍拒绝入库。`generate skills` 只读取已保存数据，不执行模式合并或 Agent 调用。

当前版本不再维护 skills dirty state。`sync` 完成学习后仅在本轮有学习变化时生成 skills。workspace `sync` 会分别展示各子项目的学习变化、生成跳过或缺失 Skills 补全状态，并在学习阶段末汇总所有子项目的文件变化、候选模式、规范化写入和移除数量；工作区关系输入未变化不代表子项目没有学习产出。显式执行 `skills-seed generate skills` 会删除旧的 skills-seed 生成目录并完整重建；手动添加用户模式后应显式运行该命令刷新产物。

### 生成标记

Skills 模板中的 skills-seed 生成说明现在受内部默认值控制，默认不写入最终文件，减少生成内容对后续学习的干扰。需要确认产物来源时，可通过文件头部的 `generated-by` 元数据或运行时日志排查。

### `agent`

#### 字段

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `engine` | `claude` | 执行分析、学习和生成摘要的 Agent 引擎，对应 `commands` 的 key |
| `commands` | `claude: claude`、`codex: codex` | engine 到 CLI 命令的映射 |
| `timeout` | `1800` | 单个 Agent 阶段的总预算，单位秒；包含排队、执行、退避和结构修复，重试不重置截止时间 |
| `max_turns` | `80` | 单次 Agent 调用允许的最大探索轮数；达到上限后结束调用，避免上下文无限增长；当前由 Claude CLI 使用 |
| `allow_user_plugins` | `false` | 是否允许 Agent 加载用户插件；默认关闭，避免批处理被用户插件影响 |
| `parallelism` | `0` | Agent 并发上限；workspace 根配置同时限制子项目数和共享 Agent 调用数，子项目配置不能放大共享额度。分析与独立审查共享额度；项目模式下 `0` 按本机 CPU 自动分配，最多 6 个并行调用 |
| `model` | 空 | skills-seed 调用 Agent CLI 时使用的模型名；空值不传模型参数，继承本机 Agent CLI 默认配置 |
| `retry.max_retries` | `3` | 可重试错误的最大重试次数；配置为 `0` 时使用默认值 `3` |
| `retry.initial_interval` | `15` | 首次重试等待秒数；配置为 `0` 时使用默认值 `15` |
| `retry.max_interval` | `120` | 指数退避最大等待秒数；配置为 `0` 时使用默认值 `120` |

`max_turns` 由 Claude CLI 用于限制单次调用的 Agent 探索轮数。学习调用默认使用严格 MCP 配置，只允许提示词声明的只读工具；未声明或被拒绝的工具不会被反复尝试。Codex 不接收该 Claude 专用参数。

#### `parallelism` 说明

1. `project` 模式下，`agent.parallelism > 1` 时，`learn current` 会并发分析独立证据焦点批次，已保存分析结果的审查可与其他批次交错执行；每个结果回到主流程后逐项 checkpoint，最终知识按议程顺序排列。
2. `workspace` 模式下，自动值为子项目数，上限 `6`。
3. 设置为大于 `0` 的数字时，使用该数字作为并发上限。
4. workspace 子项目任务通过 goroutine worker 池执行；子项目配置控制其焦点调度，但所有真实 Agent 调用仍受根配置的共享额度约束。独立分析与审查可以交错运行，同一运行状态的检查点仍串行提交。

#### `retry` 说明

1. 当前会对 429 / 529 / overloaded 等可重试 Agent CLI 错误进行指数退避重试。
2. 等待时间从 `initial_interval` 开始，每次翻倍，并受 `max_interval` 限制。
   结构校验失败不使用服务退避：最多执行一次附带前次结果与错误的结构修复，仍计入 `max_retries`。所有尝试共享阶段 `timeout`，超时后不发起新尝试。
3. `learn current` 等长耗时步骤会在进度行实时显示 Agent 错误、本次调用耗时和退避等待；同时终端会输出一条稳定的重试提示，包含等待时间和提取到的 API 原因。
4. 等待结束并进入下一次调用时，进度行会切换为“第 N 次尝试”；只有重试耗尽或不可重试错误才会显示最终 CLI 调用失败。

#### 切换 Agent

```yaml
agent:
  engine: "claude"
  model: ""
  commands:
    claude: "claude"
    codex: "codex"

skills:
  target: "codex"
  locale: "en-US"
  paths:
    claude: ".claude/skills/<project-name>-dev"
    codex: ".agents/skills/<project-name>-dev"
```

也可以在初始化时直接指定：

```bash
skills-seed init --mode project --agent codex
skills-seed init --mode project --agent codex --agent-model gpt-5-mini
skills-seed init --workspace --agent codex
```

### 工作流资源

用户工作流不写入 `config.yaml`，也不属于 `profile.mode`。用户直接提供完整 Markdown 内容，正文保存到 `.skills-seed/workflows/<id>/WORKFLOW.md`，其中 `<id>` 由 `--name` 生成：

```bash
skills-seed workflow --name release --content "# 发布流程\n\n- 发布前检查环境变量和构建产物"
```

`--name` 与 `--content` 均为必填。工作流正文不强制固定章节，用户可按任务需要维护步骤、检查项和执行约束。同名工作流不会自动合并；需要更新时使用 `--overwrite` 完全替换。

生成 skills 时，工作流会写入输出目录的 `workflows/`，对应脚本目录会复制到 `scripts/workflows/<id>/`。

### `.skills-seed` 目录结构

`.skills-seed/store/` 是持久化数据目录，不应删除；`.skills-seed/cache/` 保存可恢复命令状态；`.skills-seed/runtime/` 保存按需开启的文件日志、渲染 prompt、Agent 输出和临时输入等运行时产物，可以在不需要排障时删除。初始化不会预建 `runtime/`；提示词输入目录仅在确实需要写入输入文件时创建，直接内联的输入不会留下空目录。关闭文件日志只影响 `runtime/logs/`，其他运行产物仍按需写入。

| 路径 | 作用 |
|---|---|
| `.skills-seed/store/project.db` | patterns、质量指标、文件指纹等索引数据 |
| `.skills-seed/store/documents/` | 画像、规范、状态和变更记录等可读 JSON 文档 |
| `.skills-seed/cache/commands/<command>/state.json` | 未完成命令的可恢复状态，包括尚未入库的焦点分析结果；全部持久化后自动清除，可删除并重新检测和规划 |
| `.skills-seed/runtime/logs/` | 显式设置 `logging.enabled: true` 后写入的文件日志 |
| `.skills-seed/runtime/rendered-prompts/` | 渲染后的 prompt 和 manifest |
| `.skills-seed/runtime/agent-outputs/` | Agent 输出归档 |
| `.skills-seed/runtime/` | 可重建运行时总目录；`skills-seed clear runtime` 会删除它，`runtime.cleanup_before_reanalysis` 控制重新分析前是否自动清理 |

### `.skills-seed/context/`

`.skills-seed/context/` 不是 `config.yaml` 字段，但由 `skills-seed init` 创建，属于项目级非权威背景层。它仅用于项目背景、术语和 workspace 背景；必须遵守的长期规则由 `.skills-seed/rules/` 独立维护，任务步骤由 `.skills-seed/workflows/` 独立维护。

常见路径：

| 路径 | 作用 |
|---|---|
| `.skills-seed/context/background.md` | 代码看不到的业务背景、外部系统和线上事实 |
| `.skills-seed/context/terminology.md` | 术语、别名、状态名和业务词到代码词的对应关系 |
| `.skills-seed/context/workspace.md` | workspace 级上下文，仅 workspace 模式生成 |

这些文件会与内置 prompt 合并，不会替换内置 prompt。合并后还会追加一个内置最终输出契约，保护 AI 返回的 JSON / Markdown 格式，避免用户上下文破坏解析。

`--context` 和 `--context-path` 是学习阶段的一次性背景说明，只影响当前 `learn current` 运行，不会写入 `.skills-seed/context/`，也不会传给 `generate skills`。长期规则使用 `skills-seed rule` 维护；任务流程使用 `skills-seed workflow` 维护；临时背景使用 `learn current --context` 或 `learn current --context-path`。

### `skills`

#### 字段

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `target` | `agent.engine` | 生成的 Skills 目标类型；可与 `agent.engine` 不同 |
| `name` | 空 | 显式 Skill 名称；为空时使用项目默认名，workspace 根使用工作区默认名 |
| `locale` | `en-US` | AI 学习输出、沉淀内容和生成 Skills 使用的语言 |
| `paths.claude` | `.claude/skills/<project-name>-dev` | Claude Code skills 输出目录；workspace 根默认为 `<workspace-name>-workspace-dev` |
| `paths.codex` | `.agents/skills/<project-name>-dev` | Codex skills 输出目录；workspace 根默认为 `<workspace-name>-workspace-dev` |

#### 说明

1. `generate skills` 默认使用 `skills.target` 对应的 `skills.paths`。
2. 可通过 `skills-seed generate skills --output <path>` 临时指定输出目录。
3. `skills.locale` 支持 `zh-CN` 和 `en-US`，默认英文；它统一控制运行时 AI 自然语言输出、沉淀内容以及 `generate skills` 产物语言。
4. 新增自定义 engine 或 target 时，应分别添加 `agent.commands.<engine>` 和 `skills.paths.<target>`。

初始化时用 `skills-seed init --skills-name team-guide` 同时设置 `skills.name` 和各目标输出路径。显式名称原样使用，不追加 `-dev`，限 1–64 个小写字母、数字或连字符，不能以连字符开头、结尾或包含连续连字符。名称不会改变 `profile.name` 或项目归属，也不会由工作区根传给子项目。初始化后手动改名时，应同时调整 `skills.paths` 的目录名；修改配置不会迁移已有产物。

运行时 AI prompt 模板统一维护为英文单源模板，最终输出契约跟随 `skills.locale`；`profile.locale` 只影响工具输出、配置模板和 seed context 模板语言。

### `runtime`

#### 字段

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `cleanup_before_reanalysis` | `true` | 在重新分析前清理 `.skills-seed/runtime/`；`resume` 流程不会触发 |

### `logging`

#### 字段

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `enabled` | `false` | 是否写入 runtime 文件日志；终端输出、运行记录与检查点仍正常工作 |
| `level` | `DEBUG` | 日志级别：`DEBUG`、`INFO`、`WARN`、`ERROR` |
| `logs_path` | `runtime/logs` | 日志目录，相对 `.skills-seed` |
| `max_log_files` | `30` | 最多保留的日志文件数量，超过后自动清理旧日志 |

### `exclude`

`exclude` 控制学习、预览、项目结构摘要、样例文件收集和结构化预扫描共享的全局文件边界。

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `gitignore` | `true` | 是否排除 Git ignore 命中的文件，包括 `.gitignore`、`.git/info/exclude` 和全局 Git ignore |
| `paths` | 见下表 | 需要排除的相对路径或 glob |

关闭 `gitignore` 后，文件过滤仍会应用内置安全边界、已生成 Skills 输出目录和 `exclude.paths`，但不会再跳过被 Git ignore 规则忽略的源码文件。

#### 默认值

| Pattern | 说明 |
|---|---|
| `.*` | 点号开头的文件和目录，如 `.github`、`.cursor`、`.env` |
| `vendor/**` | 常见依赖目录 |
| `node_modules/**` | 常见依赖目录 |
| `dist/**` | 常见构建产物目录 |
| `build/**` | 常见构建产物目录 |
| `out/**` | 常见输出目录 |
| `target/**` | 常见构建产物目录 |
| `coverage/**` | 覆盖率报告目录 |
| `.cache/**` | 缓存目录 |
| `tmp/**` | 临时目录 |
| `temp/**` | 临时目录 |
| `*.log` | 日志文件 |
| `*.tmp` | 临时文件 |
| `*.bak` | 备份文件 |
| `*.swp` | 编辑器交换文件 |
| `*.zip` / `*.tar` / `*.tar.gz` / `*.tgz` / `*.rar` / `*.7z` | 压缩包 |
| `*.png` / `*.jpg` / `*.jpeg` / `*.gif` / `*.webp` / `*.ico` | 图片资源 |
| `*.pdf` | 文档产物 |
| `*.mp4` / `*.mov` | 视频资源 |

#### 说明

1. `exclude.paths` 使用 glob 风格匹配，不是正则。不含 `/` 的模式（如 `*.log`）会同时对文件基名和完整路径匹配。
2. 排除规则会影响学习、预览、项目结构摘要、样例文件收集和结构化预扫描；默认还会叠加 `exclude.gitignore`。
3. 生成的 skills 目录默认也会排除，包括配置中的 `skills.paths`、`.claude/skills/**` 和 `.agents/skills/**`。
