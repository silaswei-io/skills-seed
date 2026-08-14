package fileanalysis

import (
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/utils/pathx"
)

// PathsToFileInfos 把规范化后的相对路径转换为源码文件信息。
func PathsToFileInfos(paths []string) []domain.FileInfo {
	paths = normalizeCandidatePaths(paths)
	files := make([]domain.FileInfo, 0, len(paths))
	for _, path := range paths {
		files = append(files, domain.NewFileInfo(path, ""))
	}
	return files
}

func normalizeCandidatePaths(paths []string) []string {
	return pathx.CleanRelativeList(paths)
}
