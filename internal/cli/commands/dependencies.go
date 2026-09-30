package commands

import (
	"time"

	"github.com/urfave/cli/v2"

	"github.com/gabrielmbmb/ddogo/internal/auth"
	"github.com/gabrielmbmb/ddogo/internal/config"
	"github.com/gabrielmbmb/ddogo/internal/datadog"
)

// Dependencies supplies the external resources used by command actions.
// The client factory is called only by commands that make API requests.
type Dependencies struct {
	Store     auth.Store
	Now       func() time.Time
	NewClient func(config.Global) (*datadog.Client, error)
}

func (d Dependencies) loadGlobal(c *cli.Context) (config.Global, error) {
	return config.Load(globalFlags(c), d.Store)
}

func globalFlags(c *cli.Context) config.Global {
	return config.Global{
		Output:   c.String("output"),
		DDAPIKey: c.String("dd-api-key"),
		DDAppKey: c.String("dd-app-key"),
		Site:     c.String("site"),
		Profile:  c.String("profile"),
	}
}
