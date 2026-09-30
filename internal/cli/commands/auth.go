// Package commands contains urfave/cli command definitions for ddogo.
package commands

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/urfave/cli/v2"
	"golang.org/x/term"

	"github.com/gabrielmbmb/ddogo/internal/auth"
	"github.com/gabrielmbmb/ddogo/internal/config"
)

// Auth returns credential management commands.
func (d Dependencies) Auth() *cli.Command {
	return &cli.Command{
		Name:  "auth",
		Usage: "Manage persisted Datadog credentials",
		Subcommands: []*cli.Command{
			d.authLogin(),
			d.authStatus(),
			d.authLogout(),
		},
	}
}

func (d Dependencies) authLogin() *cli.Command {
	return &cli.Command{
		Name:        "login",
		Usage:       "Persist Datadog credentials in the OS keychain",
		Description: "Examples:\n  ddogo auth login\n  ddogo auth login --non-interactive --api-key $DD_API_KEY --app-key $DD_APP_KEY --site datadoghq.eu\n  ddogo auth login --profile work",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "api-key",
				Usage: "Datadog API key (for non-interactive login)",
			},
			&cli.StringFlag{
				Name:  "app-key",
				Usage: "Datadog application key (for non-interactive login)",
			},
			&cli.BoolFlag{
				Name:  "non-interactive",
				Usage: "Do not prompt; require keys via flags or env/global flags",
			},
		},
		Action: func(c *cli.Context) error {
			if err := c.Err(); err != nil {
				return err
			}
			profile := auth.NormalizeProfile(c.String("profile"))
			store := d.Store

			existing := auth.Credentials{}
			stored, err := store.Load(profile)
			if err == nil {
				existing = stored
			} else if !errors.Is(err, auth.ErrNotFound) && !errors.Is(err, auth.ErrUnavailable) {
				return err
			}

			apiKey := firstNonEmpty(c.String("api-key"), c.String("dd-api-key"))
			appKey := firstNonEmpty(c.String("app-key"), c.String("dd-app-key"))
			siteProvided := strings.TrimSpace(c.String("site")) != ""
			site := firstNonEmpty(c.String("site"), existing.Site)

			if c.Bool("non-interactive") {
				if strings.TrimSpace(apiKey) == "" || strings.TrimSpace(appKey) == "" {
					return fmt.Errorf("--non-interactive requires API and APP keys via --api-key/--app-key or --dd-api-key/--dd-app-key (or DD_API_KEY/DD_APP_KEY)")
				}
				if strings.TrimSpace(site) == "" {
					site = auth.DefaultSite
				}
			} else {
				if strings.TrimSpace(apiKey) == "" {
					apiKey, err = promptSecret(c.App.Reader, c.App.ErrWriter, "Datadog API key: ")
					if err != nil {
						return err
					}
				}
				if strings.TrimSpace(appKey) == "" {
					appKey, err = promptSecret(c.App.Reader, c.App.ErrWriter, "Datadog APP key: ")
					if err != nil {
						return err
					}
				}

				if !siteProvided {
					siteDefault := site
					if strings.TrimSpace(siteDefault) == "" {
						siteDefault = auth.DefaultSite
					}
					site, err = promptLine(c.App.Reader, c.App.ErrWriter, "Datadog site", siteDefault)
					if err != nil {
						return err
					}
				}
			}

			creds := auth.Credentials{APIKey: apiKey, AppKey: appKey, Site: site}.Normalize()
			if creds.APIKey == "" || creds.AppKey == "" {
				return fmt.Errorf("both Datadog API and APP keys are required")
			}

			if err := c.Err(); err != nil {
				return err
			}
			if err := store.Save(profile, creds); err != nil {
				if errors.Is(err, auth.ErrUnavailable) {
					return fmt.Errorf("unable to access secure credential store: %w", err)
				}
				return err
			}

			if err := writef(c.App.Writer, "saved Datadog credentials for profile %q (site: %s)\n", profile, creds.Site); err != nil {
				return err
			}
			return nil
		},
	}
}

func (d Dependencies) authStatus() *cli.Command {
	return &cli.Command{
		Name:  "status",
		Usage: "Show auth configuration status without exposing secrets",
		Action: func(c *cli.Context) error {
			output, err := config.ParseOutput(c.String("output"))
			if err != nil {
				return err
			}

			profile := auth.NormalizeProfile(c.String("profile"))
			store := d.Store

			var stored auth.Credentials
			storeAvailable := true
			storeHasCreds := false

			stored, err = store.Load(profile)
			switch {
			case err == nil:
				storeHasCreds = true
			case errors.Is(err, auth.ErrNotFound):
				storeHasCreds = false
			case errors.Is(err, auth.ErrUnavailable):
				storeAvailable = false
			default:
				return err
			}

			if !storeHasCreds {
				stored = auth.Credentials{}
			}
			cfg, sources := config.Resolve(globalFlags(c), stored)

			status := authStatusOutput{
				Profile: profile,
				Environment: envStatus{
					APIKeySet: strings.TrimSpace(os.Getenv("DD_API_KEY")) != "",
					AppKeySet: strings.TrimSpace(os.Getenv("DD_APP_KEY")) != "",
					SiteSet:   strings.TrimSpace(os.Getenv("DD_SITE")) != "",
				},
				Store: storeStatus{
					Available:      storeAvailable,
					HasCredentials: storeHasCreds,
					Site:           strings.TrimSpace(stored.Site),
				},
				Effective: effectiveStatus{
					APIKeySet:  cfg.DDAPIKey != "",
					AppKeySet:  cfg.DDAppKey != "",
					Site:       cfg.Site,
					APIKeyFrom: sources.APIKeyFrom,
					AppKeyFrom: sources.AppKeyFrom,
					SiteFrom:   sources.SiteFrom,
				},
			}

			if output == "json" {
				enc := json.NewEncoder(c.App.Writer)
				enc.SetIndent("", "  ")
				return enc.Encode(status)
			}

			if err := writef(c.App.Writer, "Profile: %s\n", status.Profile); err != nil {
				return err
			}
			if err := writef(c.App.Writer, "Environment: DD_API_KEY=%s DD_APP_KEY=%s DD_SITE=%s\n",
				setString(status.Environment.APIKeySet),
				setString(status.Environment.AppKeySet),
				setString(status.Environment.SiteSet),
			); err != nil {
				return err
			}
			if err := writef(c.App.Writer, "Secure store: available=%s credentials=%s", yesNo(status.Store.Available), yesNo(status.Store.HasCredentials)); err != nil {
				return err
			}
			if status.Store.Site != "" {
				if err := writef(c.App.Writer, " site=%s", status.Store.Site); err != nil {
					return err
				}
			}
			if err := writeln(c.App.Writer); err != nil {
				return err
			}
			if err := writef(c.App.Writer, "Effective: api_key=%s (%s), app_key=%s (%s), site=%s (%s)\n",
				setString(status.Effective.APIKeySet), status.Effective.APIKeyFrom,
				setString(status.Effective.AppKeySet), status.Effective.AppKeyFrom,
				status.Effective.Site, status.Effective.SiteFrom,
			); err != nil {
				return err
			}
			return nil
		},
	}
}

func (d Dependencies) authLogout() *cli.Command {
	return &cli.Command{
		Name:  "logout",
		Usage: "Delete persisted Datadog credentials from the OS keychain",
		Action: func(c *cli.Context) error {
			if err := c.Err(); err != nil {
				return err
			}
			profile := auth.NormalizeProfile(c.String("profile"))
			store := d.Store

			err := store.Delete(profile)
			if errors.Is(err, auth.ErrNotFound) {
				if err := writef(c.App.Writer, "no stored credentials found for profile %q\n", profile); err != nil {
					return err
				}
				return nil
			}
			if err != nil {
				if errors.Is(err, auth.ErrUnavailable) {
					return fmt.Errorf("unable to access secure credential store: %w", err)
				}
				return err
			}

			if err := writef(c.App.Writer, "deleted stored credentials for profile %q\n", profile); err != nil {
				return err
			}
			return nil
		},
	}
}

type authStatusOutput struct {
	Profile     string          `json:"profile"`
	Environment envStatus       `json:"environment"`
	Store       storeStatus     `json:"store"`
	Effective   effectiveStatus `json:"effective"`
}

type envStatus struct {
	APIKeySet bool `json:"dd_api_key_set"`
	AppKeySet bool `json:"dd_app_key_set"`
	SiteSet   bool `json:"dd_site_set"`
}

type storeStatus struct {
	Available      bool   `json:"available"`
	HasCredentials bool   `json:"has_credentials"`
	Site           string `json:"site,omitempty"`
}

type effectiveStatus struct {
	APIKeySet  bool   `json:"api_key_set"`
	AppKeySet  bool   `json:"app_key_set"`
	Site       string `json:"site"`
	APIKeyFrom string `json:"api_key_from"`
	AppKeyFrom string `json:"app_key_from"`
	SiteFrom   string `json:"site_from"`
}

func writef(w io.Writer, format string, args ...any) error {
	_, err := fmt.Fprintf(w, format, args...)
	return err
}

func writeln(w io.Writer) error {
	_, err := fmt.Fprintln(w)
	return err
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func stdinFD(in io.Reader) (int, error) {
	file, ok := in.(*os.File)
	if !ok {
		return 0, fmt.Errorf("interactive prompt requires a terminal; use --non-interactive with key flags")
	}
	const maxInt = int(^uint(0) >> 1)
	fd := file.Fd()
	if fd > uintptr(maxInt) {
		return 0, fmt.Errorf("stdin file descriptor value out of range")
	}
	return int(fd), nil
}

func promptSecret(in io.Reader, out io.Writer, prompt string) (string, error) {
	fd, err := stdinFD(in)
	if err != nil {
		return "", err
	}
	if !term.IsTerminal(fd) {
		return "", fmt.Errorf("interactive prompt requires a terminal; use --non-interactive with key flags")
	}

	if _, err := fmt.Fprint(out, prompt); err != nil {
		return "", err
	}
	secret, err := term.ReadPassword(fd)
	if _, writeErr := fmt.Fprintln(out); writeErr != nil {
		return "", writeErr
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(secret)), nil
}

func promptLine(in io.Reader, out io.Writer, label, defaultValue string) (string, error) {
	fd, err := stdinFD(in)
	if err != nil {
		return "", err
	}
	if !term.IsTerminal(fd) {
		return "", fmt.Errorf("interactive prompt requires a terminal")
	}

	if strings.TrimSpace(defaultValue) != "" {
		_, err = fmt.Fprintf(out, "%s [%s]: ", label, strings.TrimSpace(defaultValue))
	} else {
		_, err = fmt.Fprintf(out, "%s: ", label)
	}
	if err != nil {
		return "", err
	}

	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return strings.TrimSpace(defaultValue), nil
	}
	return line, nil
}

func setString(v bool) string {
	if v {
		return "set"
	}
	return "unset"
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
