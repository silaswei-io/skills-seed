package hook

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/interactive"
	"github.com/spf13/cobra"
)

// Cmd 返回 hook 命令
func Cmd() *cobra.Command {
	hookCmd := &cobra.Command{
		Use:     "hook",
		Short:   i18n.Get("HookShort"),
		Long:    i18n.Get("HookLongDesc"),
		Example: i18n.Get("HookExample"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	hookCmd.AddCommand(hookActionCmd("install", i18n.Get("HookInstallShort"), i18n.Get("HookInstallLongDesc"), i18n.Get("HookInstallExample"), func(cmd *cobra.Command) error {
		return installHook()
	}, i18n.Get("HookInstallSuccess")))
	hookCmd.AddCommand(hookActionCmd("uninstall", i18n.Get("HookUninstallShort"), i18n.Get("HookUninstallLongDesc"), i18n.Get("HookUninstallExample"), func(cmd *cobra.Command) error {
		return uninstallHook()
	}, i18n.Get("HookUninstallSuccess")))
	hookCmd.AddCommand(hookActionCmd("run", i18n.Get("HookRunShort"), i18n.Get("HookRunLongDesc"), i18n.Get("HookRunExample"), func(cmd *cobra.Command) error {
		return runPreCommitHook(cmd.OutOrStdout(), cmd.ErrOrStderr())
	}, ""))

	return hookCmd
}

func hookActionCmd(use, short, long, example string, action func(*cobra.Command) error, successMessage string) *cobra.Command {
	return &cobra.Command{
		Use:     use,
		Short:   short,
		Long:    long,
		Example: example,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := action(cmd); err != nil {
				return err
			}
			if successMessage != "" {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), successMessage); err != nil {
					return err
				}
			}
			return nil
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
	hookContent := preCommitHookContent()

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

func preCommitHookContent() string {
	return `#!/bin/bash
# skills-seed pre-commit hook

skills-seed hook run
`
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

func runPreCommitHook(stdout, stderr io.Writer) error {
	if !interactive.IsTerminal() {
		fmt.Fprintln(stdout, i18n.Get("HookNonInteractiveSkip"))
		return nil
	}

	action, err := selectHookAction()
	if err != nil {
		return err
	}
	if action == hookActionSkip {
		fmt.Fprintln(stdout, i18n.Get("HookSkipped"))
		return nil
	}
	return runHookAction(action, stdout, stderr)
}

type hookAction string

const (
	hookActionSync  hookAction = "sync"
	hookActionLearn hookAction = "learn"
	hookActionSkip  hookAction = "skip"
)

func selectHookAction() (hookAction, error) {
	action, err := interactive.Select(i18n.Get("HookPromptTitle"), []interactive.Option[hookAction]{
		{Value: hookActionSync, Title: i18n.Get("HookChoiceSync"), Description: i18n.Get("HookChoiceSyncDesc")},
		{Value: hookActionLearn, Title: i18n.Get("HookChoiceLearn"), Description: i18n.Get("HookChoiceLearnDesc")},
		{Value: hookActionSkip, Title: i18n.Get("HookChoiceSkip"), Description: i18n.Get("HookChoiceSkipDesc")},
	}, hookActionSkip)
	if err != nil {
		return hookActionSkip, err
	}
	return action, nil
}

func runHookAction(action hookAction, stdout, stderr io.Writer) error {
	args := hookActionArgs(action)
	if len(args) == 0 {
		return nil
	}
	cmd := exec.Command("skills-seed", args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s", i18n.GetWithParams("HookActionFailed", map[string]interface{}{"Action": strings.Join(args, " "), "Error": err.Error()}))
	}
	return nil
}

func hookActionArgs(action hookAction) []string {
	switch action {
	case hookActionSync:
		return []string{"sync"}
	case hookActionLearn:
		return []string{"learn", "current"}
	default:
		return nil
	}
}
