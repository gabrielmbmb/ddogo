package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/gabrielmbmb/ddogo/internal/datadog"
)

const prettyIssueMessageMaxRunes = 120

// RenderIssueSearchResults writes issue search results in pretty or json format.
func RenderIssueSearchResults(w io.Writer, format string, issues []datadog.IssueSearchResult) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(issues)
	case "pretty":
		return renderPrettyIssueSearchResults(w, issues)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// RenderIssue writes a single issue in pretty or json format.
func RenderIssue(w io.Writer, format string, issue datadog.ErrorTrackingIssue) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(issue)
	case "pretty":
		return renderPrettyIssue(w, issue)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func renderPrettyIssueSearchResults(w io.Writer, issues []datadog.IssueSearchResult) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "LAST_SEEN\tSTATE\tSERVICE\tTOTAL\tUSERS\tSESSIONS\tISSUE_ID\tERROR"); err != nil {
		return err
	}

	for _, item := range issues {
		lastSeen := "-"
		state := "-"
		service := "-"
		errorSummary := "-"
		if item.Issue != nil {
			lastSeen = formatMillis(item.Issue.LastSeen)
			state = prettyValue(item.Issue.State, 24)
			service = prettyValue(item.Issue.Service, 36)
			errorSummary = prettyValue(issueSummary(item.Issue), prettyIssueMessageMaxRunes)
		}
		if _, err := fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%d\t%d\t%d\t%s\t%s\n",
			prettyValue(lastSeen, 40),
			state,
			service,
			item.TotalCount,
			item.ImpactedUsers,
			item.ImpactedSessions,
			prettyValue(item.ID, 64),
			errorSummary,
		); err != nil {
			return err
		}
	}

	return tw.Flush()
}

func renderPrettyIssue(w io.Writer, issue datadog.ErrorTrackingIssue) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "FIELD\tVALUE"); err != nil {
		return err
	}
	rows := [][2]string{
		{"id", issue.ID},
		{"state", issue.State},
		{"service", issue.Service},
		{"platform", issue.Platform},
		{"error_type", issue.ErrorType},
		{"error_message", issue.ErrorMessage},
		{"file_path", issue.FilePath},
		{"function_name", issue.FunctionName},
		{"first_seen", formatMillis(issue.FirstSeen)},
		{"last_seen", formatMillis(issue.LastSeen)},
		{"first_seen_version", issue.FirstSeenVersion},
		{"last_seen_version", issue.LastSeenVersion},
		{"is_crash", formatBool(issue.IsCrash)},
		{"languages", strings.Join(issue.Languages, ",")},
		{"assignee_id", issue.AssigneeID},
		{"case_id", issue.CaseID},
		{"team_owner_ids", strings.Join(issue.TeamOwnerIDs, ",")},
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(tw, "%s\t%s\n", row[0], prettyValue(row[1], prettyIssueMessageMaxRunes)); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func issueSummary(issue *datadog.ErrorTrackingIssue) string {
	if issue == nil {
		return ""
	}
	typePart := strings.TrimSpace(issue.ErrorType)
	msgPart := strings.TrimSpace(issue.ErrorMessage)
	switch {
	case typePart != "" && msgPart != "":
		return typePart + ": " + msgPart
	case typePart != "":
		return typePart
	default:
		return msgPart
	}
}

func formatMillis(ms int64) string {
	if ms <= 0 {
		return "-"
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}

func formatBool(v *bool) string {
	if v == nil {
		return "-"
	}
	if *v {
		return "true"
	}
	return "false"
}
