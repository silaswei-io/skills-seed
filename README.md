# Skills Seed

<div align="center">

**智能代码模式学习与技能文档生成工具**

[![Go Version](https://img.shields.io/badge/Go-1.25.6+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

[简体中文](README.md) | [English](README.en.md)

</div>

---

## 📖 项目简介

Skills Seed 是一个智能的代码模式学习工具，通过分析 Git 提交历史自动学习项目的编码模式和最佳实践，并生成结构化的 Claude Code / Codex 兼容技能文档。它帮助团队沉淀编码规范，提升代码质量，并加速新成员的上手过程。

## ✨ 核心特性

- 🔍 **智能模式学习** - 从 Git 提交历史中自动提取编码模式和最佳实践
- 🤖 **AI 驱动分析** - 使用 AI 深度分析代码变更，识别命名规范、错误处理、架构模式等
- 📚 **自动文档生成** - 生成结构化的 Claude Code / Codex 兼容技能文档，包含示例和最佳实践
- ✅ **代码检查** - 基于学习的模式检查代码问题，提供修复建议
- 🔧 **自动修复** - 支持交互式和自动化的代码修复
- 🌐 **多语言支持** - 支持中文和英文，自动检测系统语言
- 💾 **本地存储** - 所有数据存储在本地，保护代码隐私

## 🚀 快速开始

### 安装

推荐直接通过 Go 安装 CLI：

```bash
go install github.com/silaswei-io/skills-seed/cmd/skills-seed@latest
skills-seed --help
```

如果命令不可用，请确认 `$GOPATH/bin` 或 `$GOBIN` 已加入 `PATH`。

也可以从源码构建：

```bash
# 克隆仓库
git clone https://github.com/silaswei-io/skills-seed.git
cd skills-seed

# 构建
go build -o skills-seed ./cmd/skills-seed

# 安装本地版本（可选）
go install ./cmd/skills-seed
```

### 初始化项目

```bash
# 在你的项目根目录执行
cd your-project
skills-seed init

# 指定语言（可选）
skills-seed init --locale zh-CN  # 中文
skills-seed init --locale en-US  # 英文
```

初始化会：

- 创建 `.skills-seed` 目录
- 生成配置、数据库和项目专属 prompts
- 记录项目根路径和默认输出配置

### 学习编码模式

```bash
# 分析当前代码库
skills-seed learn current

# 学习最近 50 次提交
skills-seed learn history --limit=50

# 学习最近 30 天的提交
skills-seed learn history --since=30d

# 只刷新项目画像（用于重新生成项目概览）
skills-seed profile refresh
```

`learn current` 只保存 patterns 和项目画像，不会直接生成 `SKILL.md` 或 `references/`。需要生成技能文档时请继续运行 `skills-seed generate-skills`。

### 检查代码

```bash
# 检查暂存区的代码
skills-seed check

# 交互式检查（支持自动修复）
skills-seed check --interactive
```

### 生成技能文档

```bash
# 生成技能文档
skills-seed generate-skills

# 需要先合并相似模式时，显式执行
skills-seed patterns merge
skills-seed generate-skills

# 指定 Claude 输出路径
skills-seed generate-skills --output ~/.claude/skills/my-project-skills

# 指定 Codex 输出路径
skills-seed generate-skills --output .agents/skills/my-project-skills
```

## 📁 项目结构

```text
your-project/
├── .skills-seed/              # Skills Seed 数据目录
│   ├── config.yaml            # 配置文件
│   ├── patterns.db            # 模式数据库（BoltDB）
│   ├── memory/                # 内存文件
│   └── logs/                  # 日志文件
├── .claude/
│   └── skills/
│       └── skills-seed-skills/  # Claude Code 技能文档
└── .agents/
    └── skills/
        └── skills-seed-skills/  # Codex 兼容技能文档
            ├── SKILL.md
            ├── agents/
            │   └── openai.yaml
            └── references/
                ├── patterns/
                └── examples/
```

## 🎯 使用场景

1. **团队协作** - 沉淀团队的编码规范和最佳实践
2. **代码审查** - 自动检查代码是否符合项目规范
3. **新人上手** - 快速了解项目的编码模式和架构风格
4. **持续改进** - 从高质量的提交中不断学习和改进
5. **AI 辅助开发** - 生成的技能文档可以帮助 Claude Code、Codex 等客户端更好地理解项目

## 🏗️ 架构设计

Skills Seed 采用领域驱动设计（DDD）和清晰的分层架构：

```text
internal/
├── domain/          # 领域层：核心业务模型和规则
├── service/         # 应用层：业务用例和流程编排
├── infra/           # 基础设施层：数据存储、Git 操作等
├── agent/           # AI 代理：与 Claude API 交互
├── command/         # 命令层：CLI 命令实现
├── container/       # 依赖注入容器
├── i18n/            # 国际化支持
├── templates/       # 模板引擎
└── utils/           # 工具函数
```

**核心领域模型**：

- **Pattern** - 代码模式（命名、错误处理、架构等）
- **Issue** - 代码问题
- **Rule** - 编码规则
- **CommitInfo** - Git 提交信息
- **FileInfo** - 文件信息

## ⚙️ 配置说明

配置文件位于 `.skills-seed/config.yaml`：

```yaml
project:
  name: "your-project"
  language: "go"
  locale: "zh-CN"  # 或 en-US

learning:
  max_commits: 50        # 每次学习的最大提交数
  batch_size: 10         # 批量处理大小
  confidence_threshold: 0.7  # 置信度阈值

output:
  skills_path: ".claude/skills/skills-seed-skills"  # 兼容旧字段
  skills_paths:
    claude: ".claude/skills/skills-seed-skills"
    codex: ".agents/skills/skills-seed-skills"

logging:
  level: "info"          # 日志级别：debug, info, warn, error
  logs_path: "logs"      # 日志目录

autofix:
  strategy: "patch"      # 修复策略：patch, direct, preview
  backup_path: "backups" # 备份目录
```

## 🔌 Git 钩子集成

Skills Seed 可以与 Git 钩子集成，在提交前自动检查代码：

```bash
# 安装 pre-commit 钩子
skills-seed hook install pre-commit

# 钩子会在 git commit 前自动运行：
# skills-seed check --interactive
```

## 🛠️ 开发指南

### 环境要求

- Go 1.25.6+
- Git

### 本地开发

```bash
# 克隆仓库
git clone https://github.com/silaswei-io/skills-seed.git
cd skills-seed

# 安装依赖
go mod download

# 运行测试
go test ./...

# 构建
go build -o skills-seed ./cmd/skills-seed

# 运行
./skills-seed --help
```

### 代码规范

- 遵循 [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- 使用 `gofmt` 格式化代码
- 使用 `golint` 和 `go vet` 检查代码
- 保持函数简洁，单一职责
- 编写单元测试，覆盖率 ≥ 80%

## 🤝 贡献指南

欢迎贡献！请查看 [CONTRIBUTING.md](CONTRIBUTING.md) 了解详情。

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'feat: add amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建 Pull Request

## 📝 更新日志

查看 [CHANGELOG.md](CHANGELOG.md) 了解版本更新历史。

## 📄 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件。

## 🙏 致谢

感谢以下开源项目：

- [Cobra](https://github.com/spf13/cobra) - 强大的 CLI 框架
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - 优雅的终端 UI
- [BoltDB](https://github.com/etcd-io/bbolt) - 高性能嵌入式数据库
- [go-i18n](https://github.com/nicksnyder/go-i18n) - 国际化支持

---

<div align="center">

**Made with ❤️ by [silaswei-io](https://github.com/silaswei-io)**

</div>
