# 贡献指南

[简体中文](CONTRIBUTING.md) | [English](CONTRIBUTING.en.md)

感谢你关注 Skills Seed。提交贡献前，请先确认本地质量检查通过。

## 开发流程

1. Fork 仓库并创建特性分支。
2. 修改代码并补充必要的单元测试。
3. 提交前运行本地检查。
4. 创建 Pull Request，并说明变更动机、影响范围和验证方式。

## 本地检查

```bash
gofmt -w .
go mod tidy
go vet ./...
go test ./...
```

## 提交建议

- 保持提交聚焦，避免混入无关格式化或临时文件。
- 单元测试文件 `*_test.go` 应随相关功能一起提交。
- 不要提交本地生成的 `.skills-seed/`、`.claude/`、`.agents/`、`dist/` 或根目录二进制。
