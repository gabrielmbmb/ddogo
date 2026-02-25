package commands

import "testing"

func TestParseOutputFlag(t *testing.T) {
	t.Parallel()

	for _, in := range []string{"pretty", "json", " PRETTY "} {
		if _, err := parseOutputFlag(in); err != nil {
			t.Fatalf("expected %q to be valid, got error: %v", in, err)
		}
	}

	if _, err := parseOutputFlag("yaml"); err == nil {
		t.Fatal("expected invalid output to fail")
	}
}

func TestResolvedSite(t *testing.T) {
	t.Parallel()

	site, source := resolvedSite("datadoghq.eu", "", false)
	if site != "datadoghq.eu" || source != "flag_or_env" {
		t.Fatalf("unexpected resolved site from cli: %q (%s)", site, source)
	}

	site, source = resolvedSite("", "us3.datadoghq.com", true)
	if site != "us3.datadoghq.com" || source != "store" {
		t.Fatalf("unexpected resolved site from store: %q (%s)", site, source)
	}

	site, source = resolvedSite("", "", false)
	if site != "datadoghq.com" || source != "default" {
		t.Fatalf("unexpected resolved default site: %q (%s)", site, source)
	}
}
