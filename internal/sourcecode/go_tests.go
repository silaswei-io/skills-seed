package sourcecode

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// GoTestModule 表示一个 go.mod 及其直接拥有的测试文件。
type GoTestModule struct {
	Workdir   string
	ModFile   string
	TestFiles []string
}

// GoTestInventory 是从当前源码树确定性提取的 Go 测试事实。
type GoTestInventory struct {
	Modules          []GoTestModule
	UnownedTestFiles []string
}

func (inventory GoTestInventory) HasModules() bool {
	return len(inventory.Modules) > 0
}

func (inventory GoTestInventory) HasTests() bool {
	for _, module := range inventory.Modules {
		if len(module.TestFiles) > 0 {
			return true
		}
	}
	return len(inventory.UnownedTestFiles) > 0
}

// DiscoverGoTests 将每个 _test.go 归属到最近的上级 go.mod。
func DiscoverGoTests(projectRoot string) (GoTestInventory, error) {
	if strings.TrimSpace(projectRoot) == "" {
		return GoTestInventory{}, nil
	}

	var moduleDirs []string
	var testFiles []string
	err := filepath.WalkDir(projectRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && path != projectRoot && ignoredSourceDir(entry.Name()) {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		switch {
		case entry.Name() == "go.mod":
			moduleDirs = append(moduleDirs, filepath.Dir(path))
		case strings.HasSuffix(entry.Name(), "_test.go"):
			testFiles = append(testFiles, path)
		}
		return nil
	})
	if err != nil {
		return GoTestInventory{}, err
	}

	sort.Slice(moduleDirs, func(i, j int) bool {
		return pathDepth(moduleDirs[i]) > pathDepth(moduleDirs[j])
	})
	modules := make(map[string]*GoTestModule, len(moduleDirs))
	for _, dir := range moduleDirs {
		workdir := relativeSourcePath(projectRoot, dir)
		modules[dir] = &GoTestModule{
			Workdir: workdir,
			ModFile: relativeSourcePath(projectRoot, filepath.Join(dir, "go.mod")),
		}
	}

	var unowned []string
	for _, testFile := range testFiles {
		owner := nearestModule(moduleDirs, testFile)
		relative := relativeSourcePath(projectRoot, testFile)
		if owner == "" {
			unowned = append(unowned, relative)
			continue
		}
		modules[owner].TestFiles = append(modules[owner].TestFiles, relative)
	}

	inventory := GoTestInventory{UnownedTestFiles: unowned}
	for _, module := range modules {
		sort.Strings(module.TestFiles)
		inventory.Modules = append(inventory.Modules, *module)
	}
	sort.Slice(inventory.Modules, func(i, j int) bool {
		return inventory.Modules[i].Workdir < inventory.Modules[j].Workdir
	})
	sort.Strings(inventory.UnownedTestFiles)
	return inventory, nil
}

func ignoredSourceDir(name string) bool {
	switch name {
	case ".git", ".skills-seed", ".agents", ".claude", "node_modules", "vendor":
		return true
	default:
		return false
	}
}

func nearestModule(moduleDirs []string, path string) string {
	for _, dir := range moduleDirs {
		rel, err := filepath.Rel(dir, path)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return dir
		}
	}
	return ""
}

func relativeSourcePath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	if rel == "." {
		return "."
	}
	return filepath.ToSlash(rel)
}

func pathDepth(path string) int {
	return len(strings.Split(filepath.Clean(path), string(filepath.Separator)))
}
