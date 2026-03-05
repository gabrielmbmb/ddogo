package datadog

import (
	"fmt"
	"strings"
)

// FormatSearchWarnings converts Datadog search metadata warnings into user-facing
// warning messages suitable for stderr output.
func FormatSearchWarnings(domain, status string, warnings []APIWarning) []string {
	out := make([]string, 0)
	if strings.EqualFold(strings.TrimSpace(status), "timeout") {
		if strings.TrimSpace(domain) == "" {
			out = append(out, "Datadog query timed out; partial results may be returned")
		} else {
			out = append(out, fmt.Sprintf("Datadog %s query timed out; partial results may be returned", strings.TrimSpace(domain)))
		}
	}

	for _, w := range warnings {
		parts := make([]string, 0, 3)
		if strings.TrimSpace(w.Title) != "" {
			parts = append(parts, strings.TrimSpace(w.Title))
		}
		if strings.TrimSpace(w.Detail) != "" {
			parts = append(parts, strings.TrimSpace(w.Detail))
		}
		if strings.TrimSpace(w.Code) != "" {
			parts = append(parts, "code="+strings.TrimSpace(w.Code))
		}
		if len(parts) == 0 {
			continue
		}
		out = append(out, strings.Join(parts, " | "))
	}

	return out
}
