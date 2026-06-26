// Package layout defines the on-disk storage layout under .skills-seed.
package layout

import "path/filepath"

// Layout exposes semantic storage roots for one .skills-seed directory.
type Layout struct {
	seedPath string
}

// New creates a storage layout rooted at seedPath.
func New(seedPath string) Layout {
	return Layout{seedPath: seedPath}
}

// Store returns the persistent store root.
func (l Layout) Store(parts ...string) string {
	return filepath.Join(append([]string{l.seedPath, "store"}, parts...)...)
}

// StoreDocuments returns the persistent human-readable document root.
func (l Layout) StoreDocuments(parts ...string) string {
	return l.Store(append([]string{"documents"}, parts...)...)
}

// Cache returns the rebuildable cache root.
func (l Layout) Cache(parts ...string) string {
	return filepath.Join(append([]string{l.seedPath, "cache"}, parts...)...)
}

// Runtime returns the disposable runtime root.
func (l Layout) Runtime(parts ...string) string {
	return filepath.Join(append([]string{l.seedPath, "runtime"}, parts...)...)
}

// ProjectDB returns the BoltDB path for project pattern data.
func (l Layout) ProjectDB() string {
	return l.Store("project.db")
}

// ProjectProfile returns the project profile document path.
func (l Layout) ProjectProfile() string {
	return l.StoreDocuments("project-profile.json")
}

// ProjectSpec returns the project spec document path.
func (l Layout) ProjectSpec() string {
	return l.StoreDocuments("project-spec.json")
}

// ProjectDocument returns a workspace child project document path.
func (l Layout) ProjectDocument(projectID, name string) string {
	return l.StoreDocuments("projects", projectID, name)
}

// WorkspaceProfile returns the workspace profile document path.
func (l Layout) WorkspaceProfile() string {
	return l.StoreDocuments("workspace-profile.json")
}

// WorkspaceSpec returns the workspace spec document path.
func (l Layout) WorkspaceSpec() string {
	return l.StoreDocuments("workspace-spec.json")
}

// State returns the persistent state document path.
func (l Layout) State() string {
	return l.StoreDocuments("state.json")
}

// ChangeLog returns the persistent changelog document path.
func (l Layout) ChangeLog() string {
	return l.StoreDocuments("change-log.json")
}

// Snapshots returns the file snapshot cache root.
func (l Layout) Snapshots() string {
	return l.Cache("snapshots")
}

// CurrentAnalysisPlan returns the current learn-current analysis plan cache path.
func (l Layout) CurrentAnalysisPlan() string {
	return l.Cache("analysis", "current", "plan.json")
}

// RuntimeLogs returns the default runtime log directory.
func (l Layout) RuntimeLogs() string {
	return l.Runtime("logs")
}
