package auth

import (
	"errors"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestNormalizeProfile(t *testing.T) {
	t.Parallel()

	if got := NormalizeProfile(""); got != DefaultProfile {
		t.Fatalf("expected %q, got %q", DefaultProfile, got)
	}
	if got := NormalizeProfile(" work "); got != "work" {
		t.Fatalf("expected %q, got %q", "work", got)
	}
}

func TestCredentialsNormalize(t *testing.T) {
	t.Parallel()

	got := (Credentials{APIKey: " a ", AppKey: " b ", Site: ""}).Normalize()
	if got.APIKey != "a" || got.AppKey != "b" || got.Site != DefaultSite {
		t.Fatalf("unexpected normalized credentials: %+v", got)
	}
}

func TestKeyringStoreSaveLoadDelete(t *testing.T) {
	keyring.MockInit()
	store := NewKeyringStore()
	profile := "test"

	creds := Credentials{APIKey: "api", AppKey: "app", Site: "datadoghq.eu"}
	if err := store.Save(profile, creds); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	got, err := store.Load(profile)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if got != creds {
		t.Fatalf("expected %+v, got %+v", creds, got)
	}

	if err := store.Delete(profile); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	_, err = store.Load(profile)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}
