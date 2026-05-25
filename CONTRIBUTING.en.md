# Contributing

[简体中文](CONTRIBUTING.md) | [English](CONTRIBUTING.en.md)

Thanks for your interest in Skills Seed. Before submitting a contribution, make sure the local quality checks pass.

## Workflow

1. Fork the repository and create a feature branch.
2. Make your changes and add focused unit tests when needed.
3. Run local checks before committing.
4. Open a pull request with the motivation, impact, and verification steps.

## Local Checks

```bash
gofmt -w .
go mod tidy
go vet ./...
go test ./...
```

## Commit Guidance

- Keep commits focused and avoid unrelated formatting or temporary files.
- Keep `*_test.go` unit tests with the related feature or fix.
- Do not commit generated `.skills-seed/`, `.claude/`, `.agents/`, `dist/`, or root-level binary files.
