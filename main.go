package main

import (
	"fmt"
	"os"

	"github.com/salaboy/skills-oci/cmd"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	if err := cmd.NewRootCmd(version).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
