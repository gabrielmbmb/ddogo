package commands

import (
	"io"
	"strings"
	"testing"
)

func TestPromptsRejectNonTerminalInput(t *testing.T) {
	t.Parallel()
	in := strings.NewReader("do not read secrets from a pipe")
	if _, err := promptSecret(in, io.Discard, "Key: "); err == nil {
		t.Fatal("expected secret prompt to require a terminal")
	}
	if _, err := promptLine(in, io.Discard, "Site", "datadoghq.com"); err == nil {
		t.Fatal("expected line prompt to require a terminal")
	}
}
