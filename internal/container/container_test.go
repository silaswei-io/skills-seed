package container

import (
	"errors"
	"fmt"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/i18n"
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
