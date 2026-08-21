package runjournal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/silaswei-io/skills-seed/internal/infra/storage/fileio"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/layout"
)

// ScopeKind 表示一次运行的作用域类型。
type ScopeKind string

const (
	// ScopeWorkspace 表示工作区根运行。
	ScopeWorkspace ScopeKind = "workspace"
	// ScopeProject 表示独立项目运行。
	ScopeProject ScopeKind = "project"
	// ScopeChild 表示工作区中的子项目运行快照。
	ScopeChild ScopeKind = "child"
)

// Scope 描述一次运行所属的项目边界。
type Scope struct {
	Kind        ScopeKind `json:"kind"`
	Name        string    `json:"name,omitempty"`
	ProjectID   string    `json:"project_id,omitempty"`
	ProjectPath string    `json:"project_path,omitempty"`
}

// Entry 描述一次运行记录。
type Entry struct {
	ID         string    `json:"id"`
	ParentID   string    `json:"parent_id,omitempty"`
	Command    string    `json:"command"`
	Scope      Scope     `json:"scope"`
	Summary    string    `json:"summary"`
	Details    []string  `json:"details,omitempty"`
	LogPath    string    `json:"log_path,omitempty"`
	Children   []Entry   `json:"children,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
}

var entrySeq atomic.Uint64

// Path 返回运行记录目录。
func Path(seedPath string) string {
	return layout.New(seedPath).RuntimeJournal()
}

// Append 追加一条运行记录。
func Append(seedPath string, entry Entry) error {
	if strings.TrimSpace(seedPath) == "" {
		return nil
	}
	if strings.TrimSpace(entry.ID) == "" {
		entry.ID = newID(entry.Command)
	}
	if entry.StartedAt.IsZero() {
		entry.StartedAt = time.Now()
	}
	if entry.FinishedAt.IsZero() {
		entry.FinishedAt = time.Now()
	}
	if strings.TrimSpace(entry.Summary) == "" {
		entry.Summary = entry.Command
	}
	if err := os.MkdirAll(Path(seedPath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return fileio.WriteFileAtomic(entryPath(seedPath, entry.ID), data, 0644)
}

// Recent 返回最近的运行记录，按完成时间倒序。
func Recent(seedPath string, limit int) ([]Entry, error) {
	dir := Path(seedPath)
	entries := make([]Entry, 0)
	matches, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var entry Entry
		if err := json.Unmarshal(data, &entry); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		left := entries[i].FinishedAt
		if left.IsZero() {
			left = entries[i].StartedAt
		}
		right := entries[j].FinishedAt
		if right.IsZero() {
			right = entries[j].StartedAt
		}
		if left.Equal(right) {
			return entries[i].ID > entries[j].ID
		}
		return left.After(right)
	})
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	return entries, nil
}

func entryPath(seedPath, id string) string {
	return filepath.Join(Path(seedPath), id+".json")
}

func newID(command string) string {
	now := time.Now().Format("20060102-150405.000000000")
	seq := entrySeq.Add(1)
	return fmt.Sprintf("%s-%s-%06d", now, safeCommandName(command), seq)
}

func safeCommandName(command string) string {
	name := strings.TrimSpace(command)
	if name == "" {
		return "run"
	}
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, name)
	return strings.Trim(name, "-")
}
