package commands

import (
	"flag"
	"testing"

	"github.com/urfave/cli/v2"
)

func TestParseCSVFlag(t *testing.T) {
	t.Parallel()

	got := parseCSVFlag("issue, issue.assignee,,issue.case")
	if len(got) != 3 {
		t.Fatalf("expected 3 values, got %d (%#v)", len(got), got)
	}
	if got[0] != "issue" || got[1] != "issue.assignee" || got[2] != "issue.case" {
		t.Fatalf("unexpected parsed values: %#v", got)
	}
}

func TestIssueIDArg(t *testing.T) {
	t.Parallel()

	set := flag.NewFlagSet("test", 0)
	_ = set.Parse([]string{"issue-1"})
	ctx := cli.NewContext(nil, set, nil)
	ctx.Command = &cli.Command{Name: "get"}

	issueID, err := issueIDArg(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issueID != "issue-1" {
		t.Fatalf("expected issue-1, got %q", issueID)
	}

	set2 := flag.NewFlagSet("test-empty", 0)
	_ = set2.Parse(nil)
	ctx2 := cli.NewContext(nil, set2, nil)
	ctx2.Command = &cli.Command{Name: "get"}

	if _, err := issueIDArg(ctx2); err == nil {
		t.Fatal("expected missing issue id to return error")
	}
}

func TestErrorsCommandNoLongerContainsRUMEventsSubcommand(t *testing.T) {
	t.Parallel()

	cmd := Errors()
	for _, sub := range cmd.Subcommands {
		if sub.Name == "rum-events" {
			t.Fatal("did not expect errors command to expose rum-events")
		}
	}
}
