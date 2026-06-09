.PHONY: build test clean format lint install run-init run-learn run-check run-generate

# 项目名称
PROJECT_NAME := skills-seed
BINARY := ./$(PROJECT_NAME)

# Go 相关
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod

# 构建目录
BUILD_DIR := build
CMD_DIR := cmd/skills-seed

# 默认目标
.DEFAULT_GOAL := build

# 构建
build:
	@echo "构建 $(PROJECT_NAME)..."
	$(GOBUILD) -o $(BINARY) ./$(CMD_DIR)
	@echo "✓ 构建完成: $(BINARY)"

# 安装
install:
	@echo "安装 $(PROJECT_NAME)..."
	$(GOCMD) install ./$(CMD_DIR)
	@echo "✓ 安装完成"

# 测试
test:
	@echo "运行测试..."
	$(GOTEST) -v ./...
	@echo "✓ 测试完成"

# 清理
clean:
	@echo "清理..."
	$(GOCLEAN)
	rm -f $(BINARY)
	rm -rf $(BUILD_DIR)
	@echo "✓ 清理完成"

# 格式化
format:
	@echo "格式化代码..."
	gofmt -w .

# Lint 检查
lint:
	@echo "运行 lint 检查..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --timeout=5m; \
	else \
		echo "golangci-lint 未安装"; \
		echo "安装: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b \$$(go env GOPATH)/bin v1.59.1"; \
	fi

# 运行命令（用于测试）
run-init:
	@echo "运行 init 命令..."
	$(BINARY) init

run-learn:
	@echo "运行 learn 命令..."
	$(BINARY) learn history --limit=5

run-check:
	@echo "运行 check 命令..."
	$(BINARY) check

run-generate:
	@echo "运行 generate 命令..."
	$(BINARY) generate skills

# 依赖管理
deps:
	@echo "下载依赖..."
	$(GOMOD) download
	@echo "✓ 依赖下载完成"

deps-update:
	@echo "更新依赖..."
	$(GOMOD) tidy
	@echo "✓ 依赖更新完成"

# 帮助
help:
	@echo "可用命令:"
	@echo "  make build          - 构建项目"
	@echo "  make install        - 安装到 GOPATH/bin"
	@echo "  make test           - 运行测试"
	@echo "  make clean          - 清理构建文件"
	@echo "  make format         - 格式化代码"
	@echo "  make lint           - 运行 lint 检查"
	@echo "  make run-init       - 运行 init 命令"
	@echo "  make run-learn      - 运行 learn 命令"
	@echo "  make run-check      - 运行 check 命令"
	@echo "  make run-generate   - 运行 generate 命令"
	@echo "  make deps           - 下载依赖"
	@echo "  make deps-update    - 更新依赖"
