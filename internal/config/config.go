// Package config resolves and validates ddogo runtime configuration.
package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gabrielmbmb/ddogo/internal/auth"
)

// Global holds configuration shared across commands. Flag and environment
// precedence is handled by the caller before loading stored credentials.
type Global struct {
	Output   string
	DDAPIKey string
	DDAppKey string
	Site     string
	Profile  string
}

// Sources describes where the effective authentication settings came from.
// It contains no credential values.
type Sources struct {
	APIKeyFrom string
	AppKeyFrom string
	SiteFrom   string
}

// Load validates supplied settings and fills missing values from the store.
// Missing or unavailable stores are acceptable for environment-only usage.
func Load(cfg Global, store auth.Store) (Global, error) {
	output, err := ParseOutput(cfg.Output)
	if err != nil {
		return Global{}, err
	}
	cfg.Output = output
	cfg.Profile = auth.NormalizeProfile(cfg.Profile)

	var stored auth.Credentials
	if store != nil && (strings.TrimSpace(cfg.DDAPIKey) == "" || strings.TrimSpace(cfg.DDAppKey) == "" || strings.TrimSpace(cfg.Site) == "") {
		stored, err = store.Load(cfg.Profile)
		if err != nil {
			if !errors.Is(err, auth.ErrNotFound) && !errors.Is(err, auth.ErrUnavailable) {
				return Global{}, err
			}
			stored = auth.Credentials{}
		}
	}
	cfg, _ = Resolve(cfg, stored)
	return cfg, nil
}

// Resolve applies supplied > stored > default precedence without I/O.
// Supplied values may already combine flags and environment variables.
func Resolve(cfg Global, stored auth.Credentials) (Global, Sources) {
	cfg.Profile = auth.NormalizeProfile(cfg.Profile)
	var sources Sources
	cfg.DDAPIKey, sources.APIKeyFrom = resolveValue(cfg.DDAPIKey, stored.APIKey, "", "missing")
	cfg.DDAppKey, sources.AppKeyFrom = resolveValue(cfg.DDAppKey, stored.AppKey, "", "missing")
	cfg.Site, sources.SiteFrom = resolveValue(cfg.Site, stored.Site, auth.DefaultSite, "default")
	return cfg, sources
}

func resolveValue(provided, stored, fallback, fallbackSource string) (string, string) {
	if value := strings.TrimSpace(provided); value != "" {
		return value, "flag_or_env"
	}
	if value := strings.TrimSpace(stored); value != "" {
		return value, "store"
	}
	return fallback, fallbackSource
}

// ParseOutput validates and normalizes an output format.
func ParseOutput(value string) (string, error) {
	output := strings.ToLower(strings.TrimSpace(value))
	switch output {
	case "pretty", "json":
		return output, nil
	default:
		return "", fmt.Errorf("invalid --output: %q (expected pretty|json)", output)
	}
}
