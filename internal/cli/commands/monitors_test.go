package commands

import "testing"

func TestMonitorsCommandIncludesSubcommands(t *testing.T) {
	t.Parallel()

	cmd := (Dependencies{}).Monitors()
	expected := []string{"list", "alerts", "create"}

	for _, name := range expected {
		found := false
		for _, sub := range cmd.Subcommands {
			if sub.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected monitors command to include %q subcommand", name)
		}
	}
}

func TestMonitorsCommandAliases(t *testing.T) {
	t.Parallel()

	cmd := (Dependencies{}).Monitors()
	if cmd.Name != "monitors" {
		t.Fatalf("expected command name 'monitors', got %q", cmd.Name)
	}

	found := false
	for _, alias := range cmd.Aliases {
		if alias == "monitor" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected 'monitor' alias")
	}
}

func TestParseCSVValues(t *testing.T) {
	t.Parallel()

	values := parseCSVValues(" env:prod,service:api, ,team:core ")
	if len(values) != 3 {
		t.Fatalf("expected 3 values, got %d", len(values))
	}
	if values[0] != "env:prod" || values[1] != "service:api" || values[2] != "team:core" {
		t.Fatalf("unexpected values: %#v", values)
	}
}

func TestNormalizeDraftStatus(t *testing.T) {
	t.Parallel()

	value, err := normalizeDraftStatus("PUBLISHED")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != "published" {
		t.Fatalf("expected published, got %q", value)
	}

	if _, err := normalizeDraftStatus("bad"); err == nil {
		t.Fatal("expected error for invalid draft status")
	}
}

func TestNormalizeOnMissingData(t *testing.T) {
	t.Parallel()

	value, err := normalizeOnMissingData("SHOW_NO_DATA")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != "show_no_data" {
		t.Fatalf("expected show_no_data, got %q", value)
	}

	if _, err := normalizeOnMissingData("bad"); err == nil {
		t.Fatal("expected error for invalid on_missing_data")
	}
}
