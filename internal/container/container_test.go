package container

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	promptloader "github.com/silaswei-io/skills-seed/internal/prompts/loader"
	"github.com/stretchr/testify/require"
	bberrors "go.etcd.io/bbolt/errors"
)

func TestPatternRepositoryErrorAddsLockHintForBoltTimeout(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))

	err := patternRepositoryError(fmt.Errorf("failed to open db: %w", bberrors.ErrTimeout))

	require.Error(t, err)
	require.True(t, errors.Is(err, bberrors.ErrTimeout))
	require.Contains(t, err.Error(), "创建模式仓储失败")
	require.Contains(t, err.Error(), "数据库文件可能正在被其他 skills-seed 命令使用，请等待当前命令结束后重试")
}

func TestPatternRepositoryErrorKeepsGenericMessageForOtherErrors(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))

	err := patternRepositoryError(errors.New("permission denied"))

	require.Error(t, err)
	require.Contains(t, err.Error(), "创建模式仓储失败")
	require.NotContains(t, err.Error(), "数据库文件可能正在被其他 skills-seed 命令使用")
}

func TestNewContainerUsesSkillsLocaleForPromptLoader(t *testing.T) {
	projectRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, ".git"), 0755))
	seedPath := filepath.Join(projectRoot, ".skills-seed")

	configRepo, err := config.NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)
	cfg := configRepo.Get()
	cfg.Project.Name = "demo"
	cfg.Project.Mode = domain.ModeProject
	cfg.Project.Locale = "zh-CN"
	cfg.Project.RootPath = projectRoot
	cfg.Agent.Engine = "noop"
	cfg.Agent.Commands = map[string]string{"noop": "noop"}
	cfg.Skills.Locale = "en-US"
	require.NoError(t, configRepo.Update(cfg))

	var capturedLoader *promptloader.Loader
	restoreFactory := RegisterAgentFactoryForTest("noop", func(commandPath string, timeout time.Duration, loader *promptloader.Loader, allowUserPlugins bool, retryCfg config.RetryConfig) agent.Agent {
		capturedLoader = loader
		return nil
	})
	defer restoreFactory()

	cont, err := NewContainer(context.Background(), seedPath)
	require.NoError(t, err)
	defer cont.Close()

	require.Same(t, cont.PromptLoader, capturedLoader)
	prompt, err := cont.PromptLoader.Render("workflow-optimize", agent.OptimizeWorkflowRequest{
		ID:       "release",
		Context:  "整理发版流程",
		Language: "go",
	})
	require.NoError(t, err)
	require.Contains(t, prompt, "All user-facing natural-language fields must be written in English (en-US)")
	require.NotContains(t, prompt, "All user-facing natural-language fields must be written in Simplified Chinese (zh-CN)")
}

func TestNewContainerLocalBackendDoesNotCreateAgent(t *testing.T) {
	projectRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, ".git"), 0755))
	seedPath := filepath.Join(projectRoot, ".skills-seed")
	configRepo, err := config.NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)
	cfg := configRepo.Get()
	cfg.Project.Mode = domain.ModeProject
	cfg.Project.RootPath = projectRoot
	cfg.Learning.Backend = config.LearningBackendLocal
	cfg.Agent.Engine = "unregistered"
	cfg.Agent.Commands = map[string]string{"unregistered": "missing-command"}
	require.NoError(t, configRepo.Update(cfg))

	cont, err := NewContainer(context.Background(), seedPath)
	require.NoError(t, err)
	defer cont.Close()
	require.Nil(t, cont.Agent)
	require.NotNil(t, cont.AnalyzerSvc)
	require.NotNil(t, cont.CuratorSvc)
}
