package learn

import (
	"strings"

	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
)

type currentChangeProfile string

const (
	// currentChangeProfileInitial 表示尚未建立基线的首次学习。
	currentChangeProfileInitial currentChangeProfile = "initial"
	// currentChangeProfileIncremental 表示对既有知识的增量演进。
	currentChangeProfileIncremental currentChangeProfile = "incremental"
	// currentChangeProfileReconcile 表示变更包含删除，需要同时核对已有知识是否应退休。
	currentChangeProfileReconcile currentChangeProfile = "reconcile"
)

func classifyCurrentChangeProfile(changes *fileanalysis.FileChanges) currentChangeProfile {
	if changes == nil {
		return currentChangeProfileIncremental
	}
	if len(changes.AddedOrModified) > 0 && len(changes.Deleted) == 0 && changes.PreviousAnalyzedCount == 0 {
		return currentChangeProfileInitial
	}
	if len(changes.Deleted) > 0 {
		return currentChangeProfileReconcile
	}
	return currentChangeProfileIncremental
}

func normalizeCurrentChangeProfile(value string) currentChangeProfile {
	switch currentChangeProfile(strings.TrimSpace(value)) {
	case currentChangeProfileInitial:
		return currentChangeProfileInitial
	case currentChangeProfileReconcile, "refactor":
		return currentChangeProfileReconcile
	default:
		// 旧版本的 micro、minor、normal 以及未知值都不再影响学习策略。
		return currentChangeProfileIncremental
	}
}
