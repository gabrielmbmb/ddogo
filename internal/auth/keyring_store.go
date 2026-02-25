package auth

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

const defaultKeyringService = "ddogo"

type keyringPayload struct {
	APIKey string `json:"api_key"`
	AppKey string `json:"app_key"`
	Site   string `json:"site,omitempty"`
}

// KeyringStore persists credentials using the operating system keychain.
type KeyringStore struct {
	service string
}

// NewKeyringStore constructs a keyring-backed credential store.
func NewKeyringStore() *KeyringStore {
	return &KeyringStore{service: defaultKeyringService}
}

func keyringUser(profile string) string {
	return "profile:" + NormalizeProfile(profile)
}

// Save stores credentials for the given profile.
func (s *KeyringStore) Save(profile string, creds Credentials) error {
	normalized := creds.Normalize()
	payload, err := json.Marshal(keyringPayload{
		APIKey: normalized.APIKey,
		AppKey: normalized.AppKey,
		Site:   normalized.Site,
	})
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}

	if err := keyring.Set(s.service, keyringUser(profile), string(payload)); err != nil {
		return errors.Join(ErrUnavailable, fmt.Errorf("keyring set failed: %w", err))
	}
	return nil
}

// Load returns credentials for the given profile.
func (s *KeyringStore) Load(profile string) (Credentials, error) {
	value, err := keyring.Get(s.service, keyringUser(profile))
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return Credentials{}, ErrNotFound
		}
		return Credentials{}, errors.Join(ErrUnavailable, fmt.Errorf("keyring get failed: %w", err))
	}

	var payload keyringPayload
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return Credentials{}, fmt.Errorf("invalid stored credentials for profile %q: %w", NormalizeProfile(profile), err)
	}

	return Credentials{
		APIKey: payload.APIKey,
		AppKey: payload.AppKey,
		Site:   payload.Site,
	}.Normalize(), nil
}

// Delete removes credentials for the given profile.
func (s *KeyringStore) Delete(profile string) error {
	err := keyring.Delete(s.service, keyringUser(profile))
	if err == nil {
		return nil
	}
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNotFound
	}
	return errors.Join(ErrUnavailable, fmt.Errorf("keyring delete failed: %w", err))
}
