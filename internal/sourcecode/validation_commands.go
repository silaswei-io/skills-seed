package sourcecode

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

var dockerGoBuildPattern = regexp.MustCompile(`(?:[A-Za-z_][A-Za-z0-9_]*=\S+\s+)*go build\b[^&|;]*`)

// DiscoverValidationCommands 从仓库文件中提取可验证的命令事实。
func DiscoverValidationCommands(projectRoot string, tests GoTestInventory) []domain.ValidationCommand {
	projectRoot = strings.TrimSpace(projectRoot)
	if projectRoot == "" {
		return nil
	}
	commands := make([]domain.ValidationCommand, 0)
	commands = append(commands, goTestValidationCommands(tests)...)
	commands = append(commands, jzeroValidationCommands(projectRoot)...)
	commands = append(commands, dockerfileValidationCommands(projectRoot)...)
	return domain.CleanValidationCommands(commands)
}

func goTestValidationCommands(tests GoTestInventory) []domain.ValidationCommand {
	commands := make([]domain.ValidationCommand, 0, len(tests.Modules))
	for _, module := range tests.Modules {
		if len(module.TestFiles) == 0 {
			continue
		}
		commands = append(commands, domain.ValidationCommand{
			Command:  "go test ./...",
			When:     "开发完成后运行单元测试验证功能正确性",
			Source:   module.ModFile,
			Workdir:  module.Workdir,
			Evidence: []string{module.ModFile},
			Type:     "test",
		})
	}
	return commands
}

func jzeroValidationCommands(projectRoot string) []domain.ValidationCommand {
	if _, err := os.Stat(filepath.Join(projectRoot, ".jzero.yaml")); err != nil {
		return nil
	}
	return []domain.ValidationCommand{
		{
			Command:  "jzero gen",
			When:     "修改 API 描述或数据库表结构后重新生成代码",
			Source:   ".jzero.yaml",
			Evidence: []string{".jzero.yaml"},
			Type:     "generate",
		},
		{
			Command:    "jzero format",
			When:       "代码生成完成后格式化派生产物",
			Source:     ".jzero.yaml",
			ScopePaths: []string{"internal"},
			Evidence:   []string{".jzero.yaml"},
			Type:       "check",
		},
	}
}

func dockerfileValidationCommands(projectRoot string) []domain.ValidationCommand {
	command := dockerfileGoBuildCommand(filepath.Join(projectRoot, "Dockerfile"))
	if command == "" {
		return nil
	}
	return []domain.ValidationCommand{{
		Command:  command,
		When:     "验证容器构建使用的 Go 编译命令",
		Source:   "Dockerfile",
		Evidence: []string{"Dockerfile"},
		Type:     "build",
	}}
}

func dockerfileGoBuildCommand(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	var statement strings.Builder
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if statement.Len() > 0 {
			statement.WriteByte(' ')
		}
		statement.WriteString(strings.TrimSuffix(line, "\\"))
		if strings.HasSuffix(line, "\\") {
			continue
		}
		if command := goBuildCommandFromDockerStatement(statement.String()); command != "" {
			return command
		}
		statement.Reset()
	}
	return goBuildCommandFromDockerStatement(statement.String())
}

func goBuildCommandFromDockerStatement(statement string) string {
	statement = stripDockerRunOptions(statement)
	match := dockerGoBuildPattern.FindString(statement)
	return strings.TrimSpace(match)
}

func stripDockerRunOptions(statement string) string {
	fields := strings.Fields(statement)
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if field == "RUN" || strings.HasPrefix(field, "--mount=") {
			continue
		}
		out = append(out, field)
	}
	return strings.Join(out, " ")
}
