package changelog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/infra/storage/fileio"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/layout"
)

const fileName = "change-log.json"

// Entry 表示一次学习或生成对项目沉淀产生的变更。
type Entry struct {
	ID        string    `json:"id"`
	Command   string    `json:"command"`
	Summary   string    `json:"summary"`
	Details   []string  `json:"details,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Builder 收集一条变更记录。
type Builder struct {
	seedPath string
	command  string
	details  []string
}

// Start 创建变更记录构造器。
func Start(seedPath, command string) *Builder {
	return &Builder{seedPath: seedPath, command: command}
}

// Detail 追加一条变更详情。
func (b *Builder) Detail(summary string) {
	if b == nil {
		return
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return
	}
	b.details = append(b.details, summary)
}

// Save 保存一条变更记录。
func (b *Builder) Save(summary string) error {
	if b == nil || b.seedPath == "" {
		return nil
	}
	summary = strings.TrimSpace(summary)
	if summary == "" && len(b.details) > 0 {
		summary = b.details[0]
	}
	return Append(b.seedPath, Entry{
		ID:        newID(b.command),
		Command:   b.command,
		Summary:   summary,
		Details:   append([]string(nil), b.details...),
		CreatedAt: time.Now(),
	})
}

// Append 追加一条变更记录。
func Append(seedPath string, entry Entry) error {
	if seedPath == "" {
		return nil
	}
	if entry.ID == "" {
		entry.ID = newID(entry.Command)
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	logFile, err := readFile(seedPath)
	if err != nil {
		return err
	}
	logFile.Entries = append(logFile.Entries, entry)
	sortEntries(logFile.Entries)

	path := Path(seedPath)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(logFile, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return fileio.WriteFileAtomic(path, data, 0644)
}

// Recent 返回最近的变更记录，按时间倒序。
func Recent(seedPath string, limit int) ([]Entry, error) {
	logFile, err := readFile(seedPath)
	if err != nil {
		return nil, err
	}
	entries := append([]Entry(nil), logFile.Entries...)
	sortEntries(entries)
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	return entries, nil
}

// Path 返回变更记录文件路径。
func Path(seedPath string) string {
	return layout.New(seedPath).ChangeLog()
}

type file struct {
	Entries []Entry `json:"entries"`
}

func readFile(seedPath string) (file, error) {
	path := Path(seedPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return file{}, nil
		}
		return file{}, err
	}
	var logFile file
	if err := json.Unmarshal(data, &logFile); err != nil {
		return file{}, err
	}
	return logFile, nil
}

func sortEntries(entries []Entry) {
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].CreatedAt.After(entries[j].CreatedAt)
	})
}

func newID(command string) string {
	now := time.Now()
	return fmt.Sprintf("%s-%s", now.Format("20060102-150405-000000000"), safeCommandName(command))
}

func safeCommandName(command string) string {
	name := strings.TrimSpace(command)
	if name == "" {
		return "change"
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
