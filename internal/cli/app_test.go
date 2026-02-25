package cli

import (
	"io"
	"testing"

	cli2 "github.com/urfave/cli/v2"
)

func TestUnknownCommandReturnsNonZero(t *testing.T) {
	app := New("test")
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
	if exitCode == 0 {
		t.Fatal("expected unknown command to exit with non-zero code")
	}
}
