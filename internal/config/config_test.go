package config

import (
	"flag"
	"testing"

	"github.com/urfave/cli/v2"
)

func TestLoadGlobalValidatesOutput(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	_ = set.String("output", "yaml", "")
	ctx := cli.NewContext(nil, set, nil)

	_, err := LoadGlobal(ctx)
	if err == nil {
		t.Fatal("expected error for invalid output")
	}
}
