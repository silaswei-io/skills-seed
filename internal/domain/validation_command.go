package domain

import (
	"strings"
	"unicode"
)

type ValidationCommandKind string

const (
	ValidationCommandOther       ValidationCommandKind = "other"
	ValidationCommandTest        ValidationCommandKind = "test"
	ValidationCommandStaticCheck ValidationCommandKind = "static_check"
	ValidationCommandBuild       ValidationCommandKind = "build"
	ValidationCommandGenerate    ValidationCommandKind = "generate"
	ValidationCommandContract    ValidationCommandKind = "contract"
)

func ClassifyValidationCommand(command ValidationCommand) ValidationCommandKind {
	declared := declaredValidationCommandKind(command.Type)
	text := strings.ToLower(strings.TrimSpace(command.Command))
	if text == "" {
		return ValidationCommandOther
	}
	tokens := validationCommandTokens(text)
	if kind := inferredValidationCommandKind(tokens, text); kind != ValidationCommandOther {
		return kind
	}
	if containsValidationCommandToken(tokens, "server", "serve", "start", "init", "install", "deploy", "version", "watch", "dev", "debug", "run") {
		return ValidationCommandOther
	}
	return declared
}

func CanonicalValidationCommandType(kind ValidationCommandKind, declared string) string {
	switch kind {
	case ValidationCommandTest:
		return "test"
	case ValidationCommandStaticCheck:
		if strings.EqualFold(strings.TrimSpace(declared), "lint") {
			return "lint"
		}
		return "check"
	case ValidationCommandBuild:
		return "build"
	case ValidationCommandGenerate:
		return "generate"
	case ValidationCommandContract:
		return "contract"
	default:
		return ""
	}
}

func declaredValidationCommandKind(value string) ValidationCommandKind {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "test":
		return ValidationCommandTest
	case "check", "lint", "static", "static_check":
		return ValidationCommandStaticCheck
	case "build":
		return ValidationCommandBuild
	case "generate":
		return ValidationCommandGenerate
	case "contract":
		return ValidationCommandContract
	default:
		return ValidationCommandOther
	}
}

func inferredValidationCommandKind(tokens []string, text string) ValidationCommandKind {
	switch {
	case containsValidationCommandToken(tokens, "test", "tests", "pytest", "jest", "vitest", "verify") || strings.Contains(text, "测试") || strings.Contains(text, "验证"):
		return ValidationCommandTest
	case containsValidationCommandToken(tokens, "check", "checks", "lint", "vet", "staticcheck", "typecheck", "format", "fmt") || strings.Contains(text, "检查") || strings.Contains(text, "格式化"):
		return ValidationCommandStaticCheck
	case containsValidationCommandToken(tokens, "build", "compile") || strings.Contains(text, "构建") || strings.Contains(text, "编译"):
		return ValidationCommandBuild
	case containsValidationCommandToken(tokens, "generate", "gen", "codegen") || strings.Contains(text, "生成"):
		return ValidationCommandGenerate
	case containsValidationCommandToken(tokens, "contract", "contracts", "swagger", "openapi", "proto") || strings.Contains(text, "契约"):
		return ValidationCommandContract
	default:
		return ValidationCommandOther
	}
}

func validationCommandTokens(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

func containsValidationCommandToken(tokens []string, expected ...string) bool {
	allowed := make(map[string]struct{}, len(expected))
	for _, value := range expected {
		allowed[value] = struct{}{}
	}
	for _, token := range tokens {
		if _, ok := allowed[token]; ok {
			return true
		}
	}
	return false
}
