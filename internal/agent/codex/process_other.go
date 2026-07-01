//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package codex

import "os/exec"

func configureCommandProcessGroup(_ *exec.Cmd) {}

func terminateCommandProcessGroup(_ *exec.Cmd) {}
