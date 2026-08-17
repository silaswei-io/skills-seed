package skilloutput

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/infra/storage/fileio"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/layout"
	"github.com/silaswei-io/skills-seed/internal/runtimefiles"
)

const manifestFileName = "manifest.json"

// Manifest 描述一次 Skills 输出对应的已核验知识和生成文件。
type Manifest struct {
	SchemaVersion         int            `json:"schema_version"`
	KnowledgeSnapshotHash string         `json:"knowledge_snapshot_hash"`
	ProgramVersion        string         `json:"program_version"`
	TargetAgent           string         `json:"target_agent"`
	OutputPath            string         `json:"output_path"`
	TemplatesHash         string         `json:"templates_hash"`
	OutputFiles           []ManifestFile `json:"output_files"`
}

// ManifestFile 是单个生成文件的内容摘要。
type ManifestFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// RuntimeManifestPath 返回指定 Skills 目标的当前审计清单路径。
func RuntimeManifestPath(seedPath, targetAgent string) string {
	target := runtimefiles.SafePart(targetAgent, "agent")
	return layout.New(seedPath).Runtime("generated-skills", target, manifestFileName)
}

// WriteManifest 根据 Skills 输出写入运行时审计清单。
func WriteManifest(outputPath, manifestPath string, manifest Manifest) error {
	files, err := outputFiles(outputPath)
	if err != nil {
		return err
	}
	manifest.SchemaVersion = 2
	manifest.OutputFiles = files
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return fileio.WriteFileAtomic(manifestPath, append(data, '\n'), 0o644)
}

func outputFiles(root string) ([]ManifestFile, error) {
	files := make([]ManifestFile, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(content)
		files = append(files, ManifestFile{
			Path:   filepath.ToSlash(rel),
			SHA256: hex.EncodeToString(sum[:]),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool {
		return strings.Compare(files[i].Path, files[j].Path) < 0
	})
	return files, nil
}
