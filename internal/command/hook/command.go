package hook

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/spf13/cobra"
)

// Cmd 返回 hook 命令
func Cmd() *cobra.Command {
	var install bool
	var uninstall bool

	hookCmd := &cobra.Command{
		Use:     "hook",
		Short:   i18n.Get("HookShort"),
		Long:    i18n.Get("HookLongDesc"),
		Example: i18n.Get("HookExample"),
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if install && uninstall {
				fmt.Println(i18n.Get("HookBothFlagsError"))
				os.Exit(1)
			}

			if install {
				if err := installHook(); err != nil {
					fmt.Println(i18n.GetWithParams("HookInstallFailed", map[string]interface{}{"Error": err.Error()}))
					os.Exit(1)
				}
				fmt.Println(i18n.Get("HookInstallSuccess"))
			} else if uninstall {
				if err := uninstallHook(); err != nil {
					fmt.Println(i18n.GetWithParams("HookUninstallFailed", map[string]interface{}{"Error": err.Error()}))
					os.Exit(1)
				}
				fmt.Println(i18n.Get("HookUninstallSuccess"))
			} else {
				// 默认执行 pre-commit hook
				if err := runPreCommitHook(); err != nil {
					fmt.Println(i18n.GetWithParams("HookRunFailed", map[string]interface{}{"Error": err.Error()}))
					os.Exit(1)
				}
			}
		},
	}

	hookCmd.Flags().BoolVarP(&install, "install", "i", false, i18n.Get("HookFlagInstall"))
	hookCmd.Flags().BoolVarP(&uninstall, "uninstall", "u", false, i18n.Get("HookFlagUninstall"))
	hookCmd.AddCommand(hookActionCmd("install", i18n.Get("HookInstallShort"), i18n.Get("HookInstallLongDesc"), i18n.Get("HookInstallExample"), installHook, i18n.Get("HookInstallSuccess")))
	hookCmd.AddCommand(hookActionCmd("uninstall", i18n.Get("HookUninstallShort"), i18n.Get("HookUninstallLongDesc"), i18n.Get("HookUninstallExample"), uninstallHook, i18n.Get("HookUninstallSuccess")))
	hookCmd.AddCommand(hookActionCmd("run", i18n.Get("HookRunShort"), i18n.Get("HookRunLongDesc"), i18n.Get("HookRunExample"), runPreCommitHook, ""))

	return hookCmd
}

func hookActionCmd(use, short, long, example string, action func() error, successMessage string) *cobra.Command {
	return &cobra.Command{
		Use:     use,
		Short:   short,
		Long:    long,
		Example: example,
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if err := action(); err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
			}
			if successMessage != "" {
				fmt.Println(successMessage)
			}
		},
	}
}

func installHook() error {
	// 获取项目根目录
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("InitGetCurrentDirFailed"), err)
	}

	// 检查是否是 Git 仓库
	gitDir := filepath.Join(projectRoot, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf("%s", i18n.Get("ErrHookNotGitRepo"))
	}

	// 检查 .skills-seed 是否已初始化
	seedPath := filepath.Join(projectRoot, ".skills-seed")
	if _, err := os.Stat(seedPath); os.IsNotExist(err) {
		return fmt.Errorf("%s", i18n.Get("ErrHookNotInitialized"))
	}

	// 创建 hook 脚本
	hookPath := filepath.Join(gitDir, "hooks", "pre-commit")
	hookContent := `#!/bin/bash
# skills-seed pre-commit hook

# 获取暂存的文件
STAGED_FILES=$(git diff --cached --name-only --diff-filter=ACM | grep '\.go$')

if [ -z "$STAGED_FILES" ]; then
    exit 0
fi

echo "Running skills-seed check..."

# 运行 skills-seed check。Git hook 通常没有交互式 TTY，必须关闭交互模式。
if ! skills-seed check --interactive=false; then
    echo "skills-seed check found issues. Please fix them before committing."
    exit 1
fi

exit 0
`

	// 确保 hooks 目录存在
	hooksDir := filepath.Join(gitDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("HookCreateDirFailed"), err)
	}

	// 写入 hook 脚本
	if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("HookWriteFileFailed"), err)
	}

	return nil
}

func uninstallHook() error {
	// 获取项目根目录
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("InitGetCurrentDirFailed"), err)
	}

	// 检查 hook 文件是否存在
	hookPath := filepath.Join(projectRoot, ".git", "hooks", "pre-commit")
	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		return fmt.Errorf("%s", i18n.Get("ErrHookNotFound"))
	}

	// 删除 hook 文件
	if err := os.Remove(hookPath); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("HookRemoveFileFailed"), err)
	}

	return nil
}

func runPreCommitHook() error {
	// 获取暂存的 Go 文件
	cmd := exec.Command("git", "diff", "--cached", "--name-only", "--diff-filter=ACM")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("HookGetStagedFilesFailed"), err)
	}

	files := strings.Split(string(output), "\n")
	goFiles := []string{}
	for _, file := range files {
		if strings.HasSuffix(file, ".go") && file != "" {
			goFiles = append(goFiles, file)
		}
	}

	if len(goFiles) == 0 {
		fmt.Println(i18n.Get("HookNoStagedFiles"))
		return nil
	}

	fmt.Println(i18n.GetWithParams("HookCheckingFiles", map[string]interface{}{"Count": len(goFiles)}))

	// 运行 skills-seed check
	checkCmd := exec.Command("skills-seed", "check", "--interactive=false")
	checkCmd.Stdout = os.Stdout
	checkCmd.Stderr = os.Stderr

	if err := checkCmd.Run(); err != nil {
		return fmt.Errorf("%s", i18n.GetWithParams("ErrHookCheckFailed", map[string]interface{}{"Error": err.Error()}))
	}

	fmt.Println(i18n.Get("HookCheckPassed"))
	return nil
}
