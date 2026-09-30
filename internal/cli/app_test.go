package cli

import (
	"io"
	"testing"

	cli2 "github.com/urfave/cli/v2"

	"github.com/gabrielmbmb/ddogo/internal/cli/commands"
)

func TestUnknownCommandExitCode(t *testing.T) {
	app := New("test", commands.Dependencies{})
	app.Writer = io.Discard
	app.ErrWriter = io.Discard

	prevExiter := cli2.OsExiter
	prevErrWriter := cli2.ErrWriter
	defer func() {
		cli2.OsExiter = prevExiter
		cli2.ErrWriter = prevErrWriter
	}()

	exitCode := 0
	cli2.OsExiter = func(code int) {
		exitCode = code
	}
	cli2.ErrWriter = io.Discard

	_ = app.Run([]string{"ddogo", "does-not-exist"})
	if exitCode != 3 {
		t.Fatalf("expected unknown command exit code 3, got %d", exitCode)
	}
}

func TestAppIncludesErrorsCommand(t *testing.T) {
	app := New("test", commands.Dependencies{})
	for _, cmd := range app.Commands {
		if cmd.Name == "errors" {
			return
		}
	}
	t.Fatal("expected root app to include errors command")
}

func TestAppIncludesRUMCommand(t *testing.T) {
	app := New("test", commands.Dependencies{})
	for _, cmd := range app.Commands {
		if cmd.Name == "rum" {
			return
		}
	}
	t.Fatal("expected root app to include rum command")
}

func TestAppIncludesMonitorsCommand(t *testing.T) {
	app := New("test", commands.Dependencies{})
	for _, cmd := range app.Commands {
		if cmd.Name == "monitors" {
			return
		}
	}
	t.Fatal("expected root app to include monitors command")
}

func TestDomainCommandAliases(t *testing.T) {
	app := New("test", commands.Dependencies{})

	expectAlias := func(commandName, alias string) {
		t.Helper()
		for _, cmd := range app.Commands {
			if cmd.Name != commandName {
				continue
			}
			for _, a := range cmd.Aliases {
				if a == alias {
					return
				}
			}
			t.Fatalf("expected command %q to have alias %q", commandName, alias)
		}
		t.Fatalf("command %q not found", commandName)
	}

	expectAlias("logs", "log")
	expectAlias("spans", "trace")
	expectAlias("spans", "traces")
	expectAlias("errors", "error")
	expectAlias("monitors", "monitor")
}
