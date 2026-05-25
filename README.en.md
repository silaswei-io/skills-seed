# Skills Seed

<div align="center">

**Intelligent code pattern learning and skill documentation generation tool**

[![Go Version](https://img.shields.io/badge/Go-1.25.6+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

[简体中文](README.md) | [English](README.en.md)

</div>

---

## 📖 Introduction

Skills Seed is an intelligent code pattern learning tool that automatically learns coding patterns and best practices from Git commit history and generates structured Claude Code / Codex-compatible skill documentation. It helps teams document coding standards, improve code quality, and accelerate onboarding for new team members.

## ✨ Key Features

- 🔍 **Smart Pattern Learning** - Automatically extract coding patterns and best practices from Git commit history
- 🤖 **AI-Powered Analysis** - Deep analysis of code changes using AI to identify naming conventions, error handling, architectural patterns, etc.
- 📚 **Auto Documentation Generation** - Generate structured Claude Code / Codex-compatible skill documentation with examples and best practices
- ✅ **Code Checking** - Check code issues based on learned patterns with fix suggestions
- 🔧 **Auto Fix** - Support interactive and automated code fixes
- 🌐 **Multi-Language Support** - Support for Chinese and English with automatic system language detection
- 💾 **Local Storage** - All data stored locally to protect code privacy

## 🚀 Quick Start

### Installation

Install the CLI directly with Go:

```bash
go install github.com/silaswei-io/skills-seed/cmd/skills-seed@latest
skills-seed --help
```

If the command is not found, make sure `$GOPATH/bin` or `$GOBIN` is in your `PATH`.

You can also build from source:

```bash
# Clone the repository
git clone https://github.com/silaswei-io/skills-seed.git
cd skills-seed

# Build
go build -o skills-seed ./cmd/skills-seed

# Install the local version (optional)
go install ./cmd/skills-seed
```

### Initialize Project

```bash
# Run in your project root
cd your-project
skills-seed init

# Specify language (optional)
skills-seed init --locale en-US  # English
skills-seed init --locale zh-CN  # Chinese
```

Initialization will:

- Create `.skills-seed` directory
- Generate configuration, database, and project-specific prompts
- Record project root and default output settings

### Learn Coding Patterns

```bash
# Analyze current codebase
skills-seed learn current

# Learn from last 50 commits
skills-seed learn history --limit=50

# Learn from last 30 days
skills-seed learn history --since=30d

# Refresh only the project profile (used to regenerate project overview)
skills-seed profile refresh
```

`learn current` only saves patterns and the project profile. It does not generate `SKILL.md` or `references/`; run `skills-seed generate-skills` when you want documentation output.

### Check Code

```bash
# Check staged files
skills-seed check

# Interactive check (with auto-fix support)
skills-seed check --interactive
```

### Generate Skill Documentation

```bash
# Generate skill documentation
skills-seed generate-skills

# Merge similar patterns explicitly when needed
skills-seed patterns merge
skills-seed generate-skills

# Specify Claude output path
skills-seed generate-skills --output ~/.claude/skills/my-project-skills

# Specify Codex output path
skills-seed generate-skills --output .agents/skills/my-project-skills
```

## 📁 Project Structure

```text
your-project/
├── .skills-seed/              # Skills Seed data directory
│   ├── config.yaml            # Configuration file
│   ├── patterns.db            # Pattern database (BoltDB)
│   ├── memory/                # Memory files
│   └── logs/                  # Log files
├── .claude/
│   └── skills/
│       └── skills-seed-skills/  # Claude Code skill documentation
└── .agents/
    └── skills/
        └── skills-seed-skills/  # Codex-compatible skill documentation
            ├── SKILL.md
            ├── agents/
            │   └── openai.yaml
            └── references/
                ├── patterns/
                └── examples/
```

## 🎯 Use Cases

1. **Team Collaboration** - Document team coding standards and best practices
2. **Code Review** - Automatically check if code follows project conventions
3. **Onboarding** - Quickly understand project coding patterns and architectural styles
4. **Continuous Improvement** - Continuously learn and improve from high-quality commits
5. **AI-Assisted Development** - Generated skill documentation helps Claude Code, Codex, and compatible clients better understand the project

## 🏗️ Architecture

Skills Seed adopts Domain-Driven Design (DDD) and clean layered architecture:

```text
internal/
├── domain/          # Domain layer: core business models and rules
├── service/         # Application layer: business use cases and orchestration
├── infra/           # Infrastructure layer: data storage, Git operations, etc.
├── agent/           # AI agent: interacts with Claude API
├── command/         # Command layer: CLI command implementations
├── container/       # Dependency injection container
├── i18n/            # Internationalization support
├── templates/       # Template engine
└── utils/           # Utility functions
```

**Core Domain Models**:

- **Pattern** - Code pattern (naming, error handling, architecture, etc.)
- **Issue** - Code issue
- **Rule** - Coding rule
- **CommitInfo** - Git commit information
- **FileInfo** - File information

## ⚙️ Configuration

Configuration file located at `.skills-seed/config.yaml`:

```yaml
project:
  name: "your-project"
  language: "go"
  locale: "en-US"  # or zh-CN

learning:
  max_commits: 50        # Maximum commits to learn each time
  batch_size: 10         # Batch processing size
  confidence_threshold: 0.7  # Confidence threshold

output:
  skills_path: ".claude/skills/skills-seed-skills"  # Legacy fallback
  skills_paths:
    claude: ".claude/skills/skills-seed-skills"
    codex: ".agents/skills/skills-seed-skills"

logging:
  level: "info"          # Log level: debug, info, warn, error
  logs_path: "logs"      # Log directory

autofix:
  strategy: "patch"      # Fix strategy: patch, direct, preview
  backup_path: "backups" # Backup directory
```

## 🔌 Git Hook Integration

Skills Seed can integrate with Git hooks to automatically check code before commits:

```bash
# Install pre-commit hook
skills-seed hook install pre-commit

# The hook will automatically run before git commit:
# skills-seed check --interactive
```

## 🛠️ Development

### Requirements

- Go 1.25.6+
- Git

### Local Development

```bash
# Clone the repository
git clone https://github.com/silaswei-io/skills-seed.git
cd skills-seed

# Install dependencies
go mod download

# Run tests
go test ./...

# Build
go build -o skills-seed ./cmd/skills-seed

# Run
./skills-seed --help
```

### Code Standards

- Follow [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Format code with `gofmt`
- Check code with `golint` and `go vet`
- Keep functions concise with single responsibility
- Write unit tests with ≥ 80% coverage

## 🤝 Contributing

Contributions are welcome! Please check [CONTRIBUTING.en.md](CONTRIBUTING.en.md) for details.

1. Fork this repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Create a Pull Request

## 📝 Changelog

See [CHANGELOG.en.md](CHANGELOG.en.md) for version history.

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

Thanks to the following open source projects:

- [Cobra](https://github.com/spf13/cobra) - Powerful CLI framework
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - Elegant terminal UI
- [BoltDB](https://github.com/etcd-io/bbolt) - High-performance embedded database
- [go-i18n](https://github.com/nicksnyder/go-i18n) - Internationalization support

---

<div align="center">

**Made with ❤️ by [silaswei-io](https://github.com/silaswei-io)**

</div>
