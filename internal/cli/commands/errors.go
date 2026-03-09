package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/gabrielmbmb/ddogo/internal/config"
	"github.com/gabrielmbmb/ddogo/internal/datadog"
	"github.com/gabrielmbmb/ddogo/internal/output"
)

// Errors returns the top-level "errors" command with its subcommands.
func Errors() *cli.Command {
	return &cli.Command{
		Name:    "errors",
		Aliases: []string{"error"},
		Usage:   "Search and manage Datadog Error Tracking issues",
		Subcommands: []*cli.Command{
			errorSearch(),
			errorGet(),
			errorSetState(),
			errorAssign(),
			errorUnassign(),
		},
	}
}

func errorSearch() *cli.Command {
	return &cli.Command{
		Name:        "search",
		Usage:       "Search error tracking issues in a time window",
		Description: "Examples:\n  ddogo errors search --query 'service:api @language:go' --track trace --from -1h\n  ddogo errors search --query 'service:web' --persona browser --order-by TOTAL_COUNT --limit 50 --output json",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "query",
				Aliases:  []string{"q"},
				Usage:    "Datadog error tracking query",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "from",
				Usage: "Start time (RFC3339) or relative duration like -15m",
				Value: "-24h",
			},
			&cli.StringFlag{
				Name:  "to",
				Usage: "End time (RFC3339) or relative duration like now / -5m",
				Value: "now",
			},
			&cli.IntFlag{
				Name:  "limit",
				Usage: "Maximum number of issues to return (Datadog max is 100)",
				Value: datadog.MaxIssuesSearchLimit,
			},
			&cli.StringFlag{
				Name:  "track",
				Usage: "Event track: trace|logs|rum",
			},
			&cli.StringFlag{
				Name:  "persona",
				Usage: "Persona: ALL|BROWSER|MOBILE|BACKEND (defaults to ALL when neither --track nor --persona is set)",
			},
			&cli.StringFlag{
				Name:  "order-by",
				Usage: "Sort order: TOTAL_COUNT|FIRST_SEEN|IMPACTED_SESSIONS|PRIORITY",
			},
			&cli.StringFlag{
				Name:  "include",
				Usage: "Comma-separated relationships to include: issue,issue.assignee,issue.case,issue.team_owners",
				Value: "issue",
			},
		},
		Action: func(c *cli.Context) error {
			cfg, err := config.LoadGlobal(c)
			if err != nil {
				return err
			}

			if c.Int("limit") <= 0 {
				return fmt.Errorf("--limit must be > 0")
			}
			if c.Int("limit") > datadog.MaxIssuesSearchLimit {
				return fmt.Errorf("--limit must be <= %d", datadog.MaxIssuesSearchLimit)
			}

			now := time.Now().UTC()
			from, to, err := parseWindow(now, c.String("from"), c.String("to"), "from", "to")
			if err != nil {
				return err
			}

			ddClient, err := newDatadogClient(cfg)
			if err != nil {
				return err
			}

			result, err := ddClient.ErrorTracking().Search(c.Context, datadog.SearchIssuesRequest{
				Query:   c.String("query"),
				From:    from.Format(time.RFC3339),
				To:      to.Format(time.RFC3339),
				Limit:   c.Int("limit"),
				Track:   c.String("track"),
				Persona: c.String("persona"),
				OrderBy: c.String("order-by"),
				Include: parseCSVFlag(c.String("include")),
			})
			if err != nil {
				return err
			}

			return output.RenderIssueSearchResults(os.Stdout, cfg.Output, result.Issues)
		},
	}
}

func errorGet() *cli.Command {
	return &cli.Command{
		Name:        "get",
		Usage:       "Get details of an error tracking issue",
		ArgsUsage:   "<issue-id>",
		Description: "Example:\n  ddogo errors get c1726a66-1f64-11ee-b338-da7ad0900002 --include assignee,team_owners",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "include",
				Usage: "Comma-separated relationships to include: assignee,case,team_owners",
			},
		},
		Action: func(c *cli.Context) error {
			cfg, err := config.LoadGlobal(c)
			if err != nil {
				return err
			}

			issueID, err := issueIDArg(c)
			if err != nil {
				return err
			}

			ddClient, err := newDatadogClient(cfg)
			if err != nil {
				return err
			}

			issue, err := ddClient.ErrorTracking().GetIssue(c.Context, issueID, parseCSVFlag(c.String("include")))
			if err != nil {
				return err
			}

			return output.RenderIssue(os.Stdout, cfg.Output, issue)
		},
	}
}

func errorSetState() *cli.Command {
	return &cli.Command{
		Name:        "set-state",
		Usage:       "Update the state of an error tracking issue",
		ArgsUsage:   "<issue-id>",
		Description: "Examples:\n  ddogo errors set-state c1726a66-1f64-11ee-b338-da7ad0900002 --state RESOLVED\n  ddogo errors set-state <issue-id> --state ACKNOWLEDGED --output json",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "state",
				Usage:    "Target state: OPEN|ACKNOWLEDGED|RESOLVED|IGNORED|EXCLUDED",
				Required: true,
			},
		},
		Action: func(c *cli.Context) error {
			cfg, err := config.LoadGlobal(c)
			if err != nil {
				return err
			}

			issueID, err := issueIDArg(c)
			if err != nil {
				return err
			}

			ddClient, err := newDatadogClient(cfg)
			if err != nil {
				return err
			}

			issue, err := ddClient.ErrorTracking().UpdateIssueState(c.Context, issueID, c.String("state"))
			if err != nil {
				return err
			}

			return output.RenderIssue(os.Stdout, cfg.Output, issue)
		},
	}
}

func errorAssign() *cli.Command {
	return &cli.Command{
		Name:        "assign",
		Usage:       "Assign an issue to a user",
		ArgsUsage:   "<issue-id>",
		Description: "Example:\n  ddogo errors assign c1726a66-1f64-11ee-b338-da7ad0900002 --assignee-id 87cb11a0-278c-440a-99fe-701223c80296",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "assignee-id",
				Aliases:  []string{"user-id"},
				Usage:    "User identifier to assign the issue to",
				Required: true,
			},
		},
		Action: func(c *cli.Context) error {
			cfg, err := config.LoadGlobal(c)
			if err != nil {
				return err
			}

			issueID, err := issueIDArg(c)
			if err != nil {
				return err
			}

			ddClient, err := newDatadogClient(cfg)
			if err != nil {
				return err
			}

			issue, err := ddClient.ErrorTracking().UpdateIssueAssignee(c.Context, issueID, c.String("assignee-id"))
			if err != nil {
				return err
			}

			return output.RenderIssue(os.Stdout, cfg.Output, issue)
		},
	}
}

func errorUnassign() *cli.Command {
	return &cli.Command{
		Name:        "unassign",
		Usage:       "Remove the assignee from an issue",
		ArgsUsage:   "<issue-id>",
		Description: "Example:\n  ddogo errors unassign c1726a66-1f64-11ee-b338-da7ad0900002",
		Action: func(c *cli.Context) error {
			cfg, err := config.LoadGlobal(c)
			if err != nil {
				return err
			}

			issueID, err := issueIDArg(c)
			if err != nil {
				return err
			}

			ddClient, err := newDatadogClient(cfg)
			if err != nil {
				return err
			}

			if err := ddClient.ErrorTracking().DeleteIssueAssignee(c.Context, issueID); err != nil {
				return err
			}

			if cfg.Output == "json" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{"issue_id": issueID, "assignee_removed": true})
			}
			_, err = fmt.Fprintf(os.Stdout, "removed assignee from issue %s\n", issueID)
			return err
		},
	}
}

func issueIDArg(c *cli.Context) (string, error) {
	issueID := strings.TrimSpace(c.Args().First())
	if issueID != "" {
		return issueID, nil
	}

	commandName := "<subcommand>"
	if c.Command != nil && strings.TrimSpace(c.Command.Name) != "" {
		commandName = c.Command.Name
	}
	return "", fmt.Errorf("issue_id is required (usage: ddogo errors %s <issue-id>)", commandName)
}

func parseCSVFlag(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}
