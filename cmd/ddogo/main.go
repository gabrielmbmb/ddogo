// Package main is the entry point for the ddogo CLI.
package main

import (
	"fmt"
	"os"

	"github.com/gabrielmbmb/ddogo/internal/cli"
	"github.com/gabrielmbmb/ddogo/internal/cli/commands"
	"github.com/gabrielmbmb/ddogo/internal/version"
)

func main() {
	// Keep default interrupt handling so Ctrl-C also stops blocking input and
	// credential-store calls, which do not accept a context.
	app := cli.New(version.Version, commands.Dependencies{})
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
