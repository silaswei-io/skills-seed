package patternnorm

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/sourcecode"
	"github.com/silaswei-io/skills-seed/internal/utils/pathx"
)

// gateEvidenceForPersistence 在入库前做确定性证据硬闸（A2）。
// 丢弃无有效证据路径、逃逸路径、流程类源码或无法在项目根解析到文件的候选。
// 不依赖 AI；调用方应在 mutation 之前执行。
func gateEvidenceForPersistence(projectRoot string, patterns []domain.Pattern) (kept []domain.Pattern, dropped []Drop) {
	kept = make([]domain.Pattern, 0, len(patterns))
	for _, pattern := range patterns {
		gated, ok := gatePatternEvidence(projectRoot, pattern)
		if !ok {
			dropped = append(dropped, Drop{
				ID:         pattern.ID,
				ReasonCode: DropUnsupportedEvidence,
				Reason:     "evidence locations failed persistence gate",
			})
			continue
		}
		kept = append(kept, gated)
	}
	return kept, dropped
}

func gatePatternEvidence(projectRoot string, pattern domain.Pattern) (domain.Pattern, bool) {
	locations := make([]domain.PatternEvidenceLocation, 0, len(pattern.EvidenceLocations))
	seen := make(map[string]struct{}, len(pattern.EvidenceLocations))
	for _, location := range pattern.EvidenceLocations {
		path, ok := gateEvidencePath(projectRoot, location.Path)
		if !ok {
			continue
		}
		location.Path = path
		key := path + "\x00" + strings.TrimSpace(location.Symbol) + "\x00" + strconv.Itoa(location.Line)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		locations = append(locations, location)
	}
	if len(locations) == 0 {
		return domain.Pattern{}, false
	}
	pattern.EvidenceLocations = locations

	if pattern.BusinessMethod != nil {
		if !gateBusinessMethodLocation(projectRoot, pattern.BusinessMethod) {
			pattern.BusinessMethod = nil
		}
	}
	return pattern, true
}

func gateEvidencePath(projectRoot, raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.Contains(raw, "..") {
		return "", false
	}
	if sourcecode.IsProcedureSource(raw) {
		return "", false
	}
	path := pathx.CleanRelative(filepath.ToSlash(raw))
	if path == "" || filepath.IsAbs(path) {
		return "", false
	}
	if strings.TrimSpace(projectRoot) == "" {
		// 无项目根时仅做结构闸，不碰文件系统。
		return path, true
	}
	abs := filepath.Join(projectRoot, filepath.FromSlash(path))
	info, err := os.Stat(abs)
	if err != nil || info.IsDir() {
		return "", false
	}
	return path, true
}

func gateBusinessMethodLocation(projectRoot string, method *domain.BusinessMethod) bool {
	if method == nil {
		return false
	}
	loc := strings.TrimSpace(method.CodeLocation.CurrentLocation)
	if loc == "" {
		loc = strings.TrimSpace(method.CodeLocation.HistoricalLocation)
	}
	if loc == "" {
		return false
	}
	// location 形如 path:line 或 path
	path := loc
	if idx := strings.LastIndex(loc, ":"); idx > 0 {
		path = loc[:idx]
	}
	_, ok := gateEvidencePath(projectRoot, path)
	return ok
}
