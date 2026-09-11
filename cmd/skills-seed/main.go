package main

import (
	"fmt"
	"io"
	"os"

	"github.com/silaswei-io/skills-seed/internal/bootstrap"
	"github.com/silaswei-io/skills-seed/internal/i18n"
)

func main() {
	if err := execute(bootstrap.Run, os.Stderr); err != nil {
		os.Exit(1)
	}
}

func execute(run func() error, stderr io.Writer) error {
	if err := run(); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", i18n.Get("CliFatalError"), err)
		return err
	}
	return nil
}
