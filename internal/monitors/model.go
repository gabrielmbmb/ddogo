package monitors

// Alert is a monitor alert group with optional investigation context.
type Alert struct {
	MonitorID       int64          `json:"monitor_id,omitempty"`
	MonitorName     string         `json:"monitor_name,omitempty"`
	MonitorType     string         `json:"monitor_type,omitempty"`
	Query           string         `json:"query,omitempty"`
	Message         string         `json:"message,omitempty"`
	Tags            []string       `json:"tags,omitempty"`
	Priority        *int64         `json:"priority,omitempty"`
	OverallState    string         `json:"overall_state,omitempty"`
	Group           string         `json:"group,omitempty"`
	Status          string         `json:"status,omitempty"`
	RaisedAtTS      int64          `json:"raised_at_ts,omitempty"`
	LastTriggeredTS int64          `json:"last_triggered_ts,omitempty"`
	LastNoDataTS    int64          `json:"last_nodata_ts,omitempty"`
	LastNotifiedTS  int64          `json:"last_notified_ts,omitempty"`
	LastResolvedTS  int64          `json:"last_resolved_ts,omitempty"`
	Investigation   *Investigation `json:"investigation,omitempty"`
}

// Investigation is best-effort search context for an alert.
type Investigation struct {
	Source string `json:"source,omitempty"`
	Query  string `json:"query,omitempty"`
	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
}
