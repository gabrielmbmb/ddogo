package commands

import "testing"

func TestMetricsCommandIncludesSubcommands(t *testing.T) {
	t.Parallel()

	cmd := Metrics()
	expected := []string{"query", "list", "metadata", "tags"}

	for _, name := range expected {
		found := false
		for _, sub := range cmd.Subcommands {
			if sub.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected metrics command to include %q subcommand", name)
		}
	}
}

func TestMetricsCommandAliases(t *testing.T) {
	t.Parallel()

	cmd := Metrics()
	if cmd.Name != "metrics" {
		t.Fatalf("expected command name 'metrics', got %q", cmd.Name)
	}

	found := false
	for _, alias := range cmd.Aliases {
		if alias == "metric" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected 'metric' alias")
	}
}
