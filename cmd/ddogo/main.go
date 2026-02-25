// Package main is the entry point for the ddogo CLI.
package main

import (
	"fmt"
	"os"

	"github.com/supersonik/ddogo/internal/cli"
	"github.com/supersonik/ddogo/internal/version"
)

func main() {
	app := cli.New(version.Version)
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
