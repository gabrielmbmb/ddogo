package config

import (
	"errors"
	"testing"

	"github.com/gabrielmbmb/ddogo/internal/auth"
)

type fakeStore struct {
	creds auth.Credentials
	err   error
}

func (f fakeStore) Save(_ string, _ auth.Credentials) error { return nil }
func (f fakeStore) Load(_ string) (auth.Credentials, error) { return f.creds, f.err }
func (f fakeStore) Delete(_ string) error                   { return nil }

func TestLoad(t *testing.T) {
	t.Parallel()
	stored := auth.Credentials{APIKey: "stored-api", AppKey: "stored-app", Site: "datadoghq.eu"}
	corrupt := errors.New("invalid stored payload")
	for _, tt := range []struct {
		name    string
		input   Global
		store   fakeStore
		want    Global
		wantErr error
	}{
		{
			name:  "store fallback",
			input: Global{Output: "pretty"}, store: fakeStore{creds: stored},
			want: Global{Output: "pretty", DDAPIKey: "stored-api", DDAppKey: "stored-app", Site: "datadoghq.eu", Profile: "default"},
		},
		{
			name:  "supplied values override store",
			input: Global{Output: " JSON ", DDAPIKey: " api ", DDAppKey: " app ", Site: " us3.datadoghq.com ", Profile: " work "},
			store: fakeStore{err: corrupt},
			want:  Global{Output: "json", DDAPIKey: "api", DDAppKey: "app", Site: "us3.datadoghq.com", Profile: "work"},
		},
		{
			name:  "site still falls back with supplied keys",
			input: Global{Output: "json", DDAPIKey: "api", DDAppKey: "app"}, store: fakeStore{creds: stored},
			want: Global{Output: "json", DDAPIKey: "api", DDAppKey: "app", Site: "datadoghq.eu", Profile: "default"},
		},
		{
			name:  "partial credentials",
			input: Global{Output: "pretty", DDAPIKey: "api"}, store: fakeStore{creds: stored},
			want: Global{Output: "pretty", DDAPIKey: "api", DDAppKey: "stored-app", Site: "datadoghq.eu", Profile: "default"},
		},
		{
			name:  "missing store",
			input: Global{Output: "pretty"}, store: fakeStore{err: auth.ErrNotFound},
			want: Global{Output: "pretty", Site: auth.DefaultSite, Profile: "default"},
		},
		{
			name:  "unavailable store",
			input: Global{Output: "pretty"}, store: fakeStore{err: auth.ErrUnavailable},
			want: Global{Output: "pretty", Site: auth.DefaultSite, Profile: "default"},
		},
		{
			name:  "corrupt store",
			input: Global{Output: "pretty"}, store: fakeStore{err: corrupt}, wantErr: corrupt,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Load(tt.input, tt.store)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatal("resolved configuration did not match expected values")
			}
		})
	}
}

func TestResolveSources(t *testing.T) {
	t.Parallel()
	cfg, sources := Resolve(Global{DDAPIKey: "provided", Profile: " work "}, auth.Credentials{AppKey: "stored"})
	if cfg.Profile != "work" || cfg.Site != auth.DefaultSite {
		t.Fatal("incorrect credential resolution")
	}
	if sources.APIKeyFrom != "flag_or_env" || sources.AppKeyFrom != "store" || sources.SiteFrom != "default" {
		t.Fatal("incorrect credential sources")
	}
	_, sources = Resolve(Global{}, auth.Credentials{})
	if sources.APIKeyFrom != "missing" || sources.AppKeyFrom != "missing" {
		t.Fatal("expected missing credentials")
	}
	_, sources = Resolve(Global{}, auth.Credentials{Site: "datadoghq.eu"})
	if sources.SiteFrom != "store" {
		t.Fatal("expected stored site")
	}
}

func TestParseOutput(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"pretty", "json", " PRETTY "} {
		if _, err := ParseOutput(value); err != nil {
			t.Fatalf("expected %q to be valid: %v", value, err)
		}
	}
	if _, err := Load(Global{Output: "yaml"}, nil); err == nil {
		t.Fatal("expected invalid output to fail")
	}
	if _, err := Load(Global{Output: "pretty"}, nil); err != nil {
		t.Fatalf("expected nil store to use defaults: %v", err)
	}
}
