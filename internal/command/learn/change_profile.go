package learn

import "github.com/silaswei-io/skills-seed/internal/service/fileanalysis"

type currentChangeProfile string

const (
	currentChangeProfileInitial  currentChangeProfile = "initial"
	currentChangeProfileMicro    currentChangeProfile = "micro"
	currentChangeProfileMinor    currentChangeProfile = "minor"
	currentChangeProfileNormal   currentChangeProfile = "normal"
	currentChangeProfileRefactor currentChangeProfile = "refactor"
)

func classifyCurrentChangeProfile(changes *fileanalysis.FileChanges) currentChangeProfile {
	if changes == nil {
		return currentChangeProfileNormal
	}
	changed := len(changes.AddedOrModified)
	deleted := len(changes.Deleted)
	if changed > 0 && deleted == 0 && len(changes.Unchanged) == 0 {
		return currentChangeProfileInitial
	}
	if deleted > 0 {
		return currentChangeProfileRefactor
	}
	switch {
	case changed <= 2:
		return currentChangeProfileMicro
	case changed <= 15:
		return currentChangeProfileMinor
	default:
		return currentChangeProfileNormal
	}
}
