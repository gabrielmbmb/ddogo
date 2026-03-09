// Package auth manages Datadog credential persistence and retrieval.
package auth

import (
	"errors"
	"strings"
)

const (
	// DefaultProfile is the profile used when no explicit profile is provided.
	DefaultProfile = "default"
	// DefaultSite is the Datadog site used when no explicit site is configured.
	DefaultSite = "datadoghq.com"
)

var (
	// ErrNotFound indicates credentials are not present in the configured store.
	ErrNotFound = errors.New("credentials not found")
	// ErrUnavailable indicates the credential store is not available on this machine/session.
	ErrUnavailable = errors.New("credential store unavailable")
)

// Credentials represents persisted Datadog auth material.
type Credentials struct {
	APIKey string `json:"api_key"` //nolint:gosec // Contains credential material by design.
	AppKey string `json:"app_key"` //nolint:gosec // Contains credential material by design.
	Site   string `json:"site,omitempty"`
}

// Normalize trims credentials and ensures the site fallback.
func (c Credentials) Normalize() Credentials {
	out := Credentials{
		APIKey: strings.TrimSpace(c.APIKey),
		AppKey: strings.TrimSpace(c.AppKey),
		Site:   strings.TrimSpace(c.Site),
	}
	if out.Site == "" {
		out.Site = DefaultSite
	}
	return out
}

// Store abstracts secure credential persistence.
type Store interface {
	Save(profile string, creds Credentials) error
	Load(profile string) (Credentials, error)
	Delete(profile string) error
}

// NormalizeProfile resolves an empty profile to DefaultProfile.
func NormalizeProfile(profile string) string {
	p := strings.TrimSpace(profile)
	if p == "" {
		return DefaultProfile
	}
	return p
}
