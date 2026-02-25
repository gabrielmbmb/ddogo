package config

import (
	"errors"
	"flag"
	"testing"

	"github.com/urfave/cli/v2"

	"github.com/gabrielmbmb/ddogo/internal/auth"
)

type fakeStore struct {
	creds auth.Credentials
	err   error
}

func (f fakeStore) Save(_ string, _ auth.Credentials) error { return nil }
func (f fakeStore) Load(_ string) (auth.Credentials, error) { return f.creds, f.err }
func (f fakeStore) Delete(_ string) error                   { return nil }

func TestLoadGlobalValidatesOutput(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	_ = set.String("output", "yaml", "")
	_ = set.String("dd-api-key", "", "")
	_ = set.String("dd-app-key", "", "")
	_ = set.String("site", "", "")
	_ = set.String("profile", "", "")
	ctx := cli.NewContext(nil, set, nil)

	_, err := loadGlobalWithStore(ctx, fakeStore{})
	if err == nil {
		t.Fatal("expected error for invalid output")
	}
}

func TestLoadGlobalFallsBackToStore(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	_ = set.String("output", "pretty", "")
	_ = set.String("dd-api-key", "", "")
	_ = set.String("dd-app-key", "", "")
	_ = set.String("site", "", "")
	_ = set.String("profile", "", "")
	ctx := cli.NewContext(nil, set, nil)

	cfg, err := loadGlobalWithStore(ctx, fakeStore{creds: auth.Credentials{
		APIKey: "api",
		AppKey: "app",
		Site:   "datadoghq.eu",
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DDAPIKey != "api" || cfg.DDAppKey != "app" || cfg.Site != "datadoghq.eu" {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}

func TestLoadGlobalUsesDefaultSiteWhenStoreMissing(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	_ = set.String("output", "pretty", "")
	_ = set.String("dd-api-key", "", "")
	_ = set.String("dd-app-key", "", "")
	_ = set.String("site", "", "")
	_ = set.String("profile", "", "")
	ctx := cli.NewContext(nil, set, nil)

	for _, errVal := range []error{auth.ErrNotFound, auth.ErrUnavailable} {
		cfg, err := loadGlobalWithStore(ctx, fakeStore{err: errVal})
		if err != nil {
			t.Fatalf("unexpected error for %v: %v", errVal, err)
		}
		if cfg.Site != auth.DefaultSite {
			t.Fatalf("expected default site %q, got %q", auth.DefaultSite, cfg.Site)
		}
	}
}

func TestLoadGlobalReturnsStoreDataCorruptionErrors(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	_ = set.String("output", "pretty", "")
	_ = set.String("dd-api-key", "", "")
	_ = set.String("dd-app-key", "", "")
	_ = set.String("site", "", "")
	_ = set.String("profile", "", "")
	ctx := cli.NewContext(nil, set, nil)

	storeErr := errors.New("bad payload")
	_, err := loadGlobalWithStore(ctx, fakeStore{err: storeErr})
	if !errors.Is(err, storeErr) {
		t.Fatalf("expected %v, got %v", storeErr, err)
	}
}
