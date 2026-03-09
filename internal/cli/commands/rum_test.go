package commands

import "testing"

func TestRUMCommandIncludesSearchSubcommand(t *testing.T) {
	t.Parallel()

	cmd := RUM()
	for _, sub := range cmd.Subcommands {
		if sub.Name == "search" {
			return
		}
	}

	t.Fatal("expected rum command to include search subcommand")
}
