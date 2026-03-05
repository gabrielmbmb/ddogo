package commands

import (
	"fmt"
	"time"

	"github.com/gabrielmbmb/ddogo/internal/config"
	"github.com/gabrielmbmb/ddogo/internal/datadog"
)

func newDatadogClient(cfg config.Global) (*datadog.Client, error) {
	return datadog.NewClient(datadog.ClientConfig{
		APIKey: cfg.DDAPIKey,
		AppKey: cfg.DDAppKey,
		Site:   cfg.Site,
	})
}

func parseWindow(now time.Time, fromValue, toValue, fromFlag, toFlag string) (time.Time, time.Time, error) {
	from, err := parseTimeInput(fromValue, now)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid --%s: %w", fromFlag, err)
	}
	to, err := parseTimeInput(toValue, now)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid --%s: %w", toFlag, err)
	}
	if to.Before(from) {
		return time.Time{}, time.Time{}, fmt.Errorf("--%s must be >= --%s", toFlag, fromFlag)
	}
	return from, to, nil
}

func parseTimeInput(v string, now time.Time) (time.Time, error) {
	if v == "now" {
		return now, nil
	}
	if len(v) > 0 && v[0] == '-' {
		d, err := time.ParseDuration(v)
		if err != nil {
			return time.Time{}, err
		}
		return now.Add(d), nil
	}
	return time.Parse(time.RFC3339, v)
}
