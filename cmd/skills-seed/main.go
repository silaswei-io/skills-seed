package main

import (
	"fmt"
	"os"

	"github.com/silaswei-io/skills-seed/internal/bootstrap"
	"github.com/silaswei-io/skills-seed/internal/i18n"
)

func main() {
	if err := bootstrap.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", i18n.Get("CliFatalError"), err)
		os.Exit(1)
	}
}
