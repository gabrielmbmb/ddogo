package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gabrielmbmb/ddogo/internal/datadog"
)

func TestRenderPrettyEscapesMultilineMessages(t *testing.T) {
	var b bytes.Buffer
	err := RenderLogs(&b, "pretty", []datadog.LogEntry{
		{Timestamp: "2026-02-25T08:00:00Z", Message: "line1\nline2\r\nline3\tend"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := b.String()
	if strings.Contains(out, "line1\nline2\nline3") {
		t.Fatalf("expected pretty output to keep one row per log, got: %q", out)
	}
	if !strings.Contains(out, `line1\nline2\nline3 end`) {
		t.Fatalf("expected escaped newlines and normalized tabs, got: %q", out)
	}
}

func TestFormatPrettyMessageTruncatesLongMessages(t *testing.T) {
	msg := strings.Repeat("a", prettyMessageMaxRunes+20)
	formatted := formatPrettyMessage(msg)

	if !strings.HasSuffix(formatted, "…") {
		t.Fatalf("expected ellipsis, got %q", formatted)
	}
	if got := len([]rune(formatted)); got != prettyMessageMaxRunes {
		t.Fatalf("expected %d runes, got %d", prettyMessageMaxRunes, got)
	}
}
