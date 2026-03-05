package commands

import (
	"flag"
	"testing"

	"github.com/urfave/cli/v2"

	spansvc "github.com/gabrielmbmb/ddogo/internal/spans"
)

func TestHasLogsFlagsWithoutWithLogs(t *testing.T) {
	t.Parallel()

	newCtx := func(args ...string) *cli.Context {
		set := flag.NewFlagSet("test", 0)
		_ = set.String("logs-query", "", "")
		_ = set.String("logs-from", "", "")
		_ = set.String("logs-to", "", "")
		_ = set.Int("logs-limit", defaultLogsLimit, "")
		_ = set.String("logs-rate-limit-mode", spansvc.DefaultLogsRateLimitMode, "")
		_ = set.Duration("logs-rate-limit-wait", spansvc.DefaultLogsRateLimitWait, "")
		_ = set.Int("logs-rate-limit-max-waits", spansvc.DefaultLogsRateLimitMaxWaits, "")
		_ = set.Parse(args)
		return cli.NewContext(nil, set, nil)
	}

	if hasLogsFlagsWithoutWithLogs(newCtx()) {
		t.Fatal("expected false when no logs flags are provided")
	}
	if !hasLogsFlagsWithoutWithLogs(newCtx("--logs-query", "status:error")) {
		t.Fatal("expected true when --logs-query is provided")
	}
	if !hasLogsFlagsWithoutWithLogs(newCtx("--logs-from", "-1h")) {
		t.Fatal("expected true when --logs-from is provided")
	}
	if !hasLogsFlagsWithoutWithLogs(newCtx("--logs-to", "now")) {
		t.Fatal("expected true when --logs-to is provided")
	}
	if !hasLogsFlagsWithoutWithLogs(newCtx("--logs-limit", "50")) {
		t.Fatal("expected true when --logs-limit is explicitly set")
	}
	if !hasLogsFlagsWithoutWithLogs(newCtx("--logs-rate-limit-mode", "wait")) {
		t.Fatal("expected true when --logs-rate-limit-mode is explicitly set")
	}
	if !hasLogsFlagsWithoutWithLogs(newCtx("--logs-rate-limit-wait", "10s")) {
		t.Fatal("expected true when --logs-rate-limit-wait is explicitly set")
	}
	if !hasLogsFlagsWithoutWithLogs(newCtx("--logs-rate-limit-max-waits", "5")) {
		t.Fatal("expected true when --logs-rate-limit-max-waits is explicitly set")
	}
}
