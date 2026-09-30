package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	cli2 "github.com/urfave/cli/v2"

	"github.com/gabrielmbmb/ddogo/internal/auth"
	"github.com/gabrielmbmb/ddogo/internal/cli/commands"
	"github.com/gabrielmbmb/ddogo/internal/config"
	"github.com/gabrielmbmb/ddogo/internal/datadog"
)

type memoryStore struct {
	creds   auth.Credentials
	profile string
	loadErr error
}

func (s *memoryStore) Load(profile string) (auth.Credentials, error) {
	s.profile = profile
	return s.creds, s.loadErr
}

func (s *memoryStore) Save(profile string, creds auth.Credentials) error {
	s.profile, s.creds, s.loadErr = profile, creds, nil
	return nil
}

func (s *memoryStore) Delete(profile string) error {
	s.profile, s.creds, s.loadErr = profile, auth.Credentials{}, auth.ErrNotFound
	return nil
}

func integrationApp(t *testing.T, handler http.HandlerFunc) (*cli2.App, *bytes.Buffer, *bytes.Buffer, *memoryStore) {
	t.Helper()
	for _, name := range []string{"DD_API_KEY", "DD_APP_KEY", "DD_SITE", "DDOGO_PROFILE"} {
		t.Setenv(name, "")
	}
	store := &memoryStore{creds: auth.Credentials{APIKey: "test-api", AppKey: "test-app", Site: "datadoghq.eu"}}
	var server *httptest.Server
	if handler != nil {
		server = httptest.NewServer(handler)
		t.Cleanup(server.Close)
	}
	app := New("test", commands.Dependencies{
		Store: store,
		Now:   func() time.Time { return time.Date(2026, 2, 25, 8, 0, 0, 0, time.UTC) },
		NewClient: func(cfg config.Global) (*datadog.Client, error) {
			if server == nil {
				t.Error("unexpected API client construction")
				return nil, errors.New("unexpected API client construction")
			}
			return datadog.NewClient(datadog.ClientConfig{
				APIKey: cfg.DDAPIKey, AppKey: cfg.DDAppKey, Site: cfg.Site,
				APIBaseURL: server.URL, HTTPClient: server.Client(), MaxRetries: -1,
			})
		},
	})
	out, diagnostics := new(bytes.Buffer), new(bytes.Buffer)
	app.Reader, app.Writer, app.ErrWriter = strings.NewReader(""), out, diagnostics
	return app, out, diagnostics, store
}

func TestCommandJSONContracts(t *testing.T) {
	issueResponse := `{"data":{"id":"issue-1","attributes":{"state":"OPEN","error_message":"boom"}}}`
	issueJSON := `{"id":"issue-1","error_message":"boom","state":"OPEN"}`
	for _, tt := range []struct {
		name, method, path, response, want string
		args                               []string
	}{
		{"logs", "POST", "/api/v2/logs/events/search", `{"data":[{"id":"log-1","attributes":{"timestamp":"2026-02-25T08:00:00Z","message":"hello"}}],"meta":{"status":"timeout"}}`, `[{"id":"log-1","timestamp":"2026-02-25T08:00:00Z","message":"hello"}]`, []string{"log", "search", "--query", "service:api"}},
		{"spans", "POST", "/api/v2/spans/events/search", `{"data":[{"id":"span-1","attributes":{"service":"api","trace_id":"trace-1"}}]}`, `[{"id":"span-1","service":"api","trace_id":"trace-1"}]`, []string{"trace", "search", "--query", "service:api"}},
		{"rum", "POST", "/api/v2/rum/events/search", `{"data":[{"id":"rum-1","type":"rum","attributes":{"service":"web"}}]}`, `[{"id":"rum-1","type":"rum","service":"web"}]`, []string{"rum", "search", "--query", "service:web"}},
		{"metrics query", "GET", "/api/v1/query", `{"status":"success"}`, `{"status":"success"}`, []string{"metric", "query", "--query", "avg:cpu{*}"}},
		{"metrics list", "GET", "/api/v2/metrics", `{"data":[{"id":"cpu","type":"metrics","attributes":{"metric_type":"gauge"}}]}`, `{"metrics":[{"id":"cpu","type":"metrics","metric_type":"gauge"}]}`, []string{"metrics", "list"}},
		{"metrics metadata", "GET", "/api/v1/metrics/cpu", `{"description":"CPU usage"}`, `{"description":"CPU usage","metric_name":"cpu"}`, []string{"metrics", "metadata", "cpu"}},
		{"metrics tags", "GET", "/api/v2/metrics/cpu/all-tags", `{"data":{"id":"cpu","attributes":{"tags":["env:test"],"ingested_tags":["env:test"]}}}`, `{"metric_name":"cpu","tags":["env:test"],"ingested_tags":["env:test"]}`, []string{"metrics", "tags", "cpu"}},
		{"issues search", "POST", "/api/v2/error-tracking/issues/search", `{"data":[{"id":"issue-1","attributes":{"total_count":2}}],"included":[{"id":"issue-1","type":"issue","attributes":{"state":"OPEN","error_message":"boom"}}]}`, `[{"id":"issue-1","total_count":2,"issue":` + issueJSON + `}]`, []string{"error", "search", "--query", "service:api"}},
		{"issues get", "GET", "/api/v2/error-tracking/issues/issue-1", issueResponse, issueJSON, []string{"errors", "get", "issue-1"}},
		{"issues state", "PUT", "/api/v2/error-tracking/issues/issue-1/state", issueResponse, issueJSON, []string{"errors", "set-state", "--state", "OPEN", "issue-1"}},
		{"issues assign", "PUT", "/api/v2/error-tracking/issues/issue-1/assignee", issueResponse, issueJSON, []string{"errors", "assign", "--assignee-id", "user-1", "issue-1"}},
		{"issues unassign", "DELETE", "/api/v2/error-tracking/issues/issue-1/assignee", "", `{"issue_id":"issue-1","assignee_removed":true}`, []string{"errors", "unassign", "issue-1"}},
		{"monitors list", "GET", "/api/v1/monitor", `[{"id":123,"name":"CPU","type":"query alert"}]`, `[{"id":123,"name":"CPU","type":"query alert"}]`, []string{"monitor", "list"}},
		{"monitors create", "POST", "/api/v1/monitor", `{"id":123,"name":"CPU","type":"query alert"}`, `{"id":123,"name":"CPU","type":"query alert"}`, []string{"monitors", "create", "--name", "CPU", "--type", "query alert", "--query", "avg:cpu{*} > 80", "--message", "Investigate CPU"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			app, out, diagnostics, _ := integrationApp(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != tt.method || r.URL.Path != tt.path {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("DD-API-KEY") != "test-api" || r.Header.Get("DD-APPLICATION-KEY") != "test-app" {
					t.Error("request did not use stored credentials")
				}
				if tt.name == "logs" {
					var body struct{ Filter struct{ From, To string } }
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Errorf("decode request: %v", err)
					}
					if body.Filter.From != "2026-02-25T07:45:00Z" || body.Filter.To != "2026-02-25T08:00:00Z" {
						t.Error("search did not use injected clock")
					}
				}
				if tt.response == "" {
					w.WriteHeader(http.StatusNoContent)
				} else {
					_, _ = io.WriteString(w, tt.response)
				}
			})
			args := append([]string{"ddogo", "--output", "json"}, tt.args...)
			if err := app.Run(args); err != nil {
				t.Fatalf("run command: %v", err)
			}
			// Field order is not part of the JSON contract.
			var got, want any
			if err := json.Unmarshal(out.Bytes(), &got); err != nil {
				t.Fatalf("invalid JSON output: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.want), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) || calls != 1 {
				t.Fatalf("unexpected output or request count: %s (calls=%d)", out, calls)
			}
			if tt.name == "logs" && !strings.Contains(diagnostics.String(), "partial results") {
				t.Fatal("expected timeout diagnostic on stderr")
			}
			if tt.name != "logs" && diagnostics.Len() != 0 {
				t.Fatalf("unexpected diagnostics: %s", diagnostics)
			}
		})
	}
}

func TestMonitorAlertsAndStdin(t *testing.T) {
	t.Run("derive then limit alert groups", func(t *testing.T) {
		app, out, _, _ := integrationApp(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("group_states") != "alert" {
				t.Error("expected alert group filter")
			}
			_, _ = io.WriteString(w, `[{"id":123,"name":"Errors","query":"logs(\"service:api\").last(\"1h\") > 0","state":{"groups":{"host:old":{"status":"Alert","last_triggered_ts":1710000000},"host:new":{"status":"Alert","last_triggered_ts":1710000001}}}}]`)
		})
		if err := app.Run([]string{"ddogo", "--output", "json", "monitors", "alerts", "--status", "alert", "--limit", "1"}); err != nil {
			t.Fatal(err)
		}
		var alerts []struct {
			Group         string `json:"group"`
			Investigation struct{ Source, Query, From, To string }
		}
		if err := json.Unmarshal(out.Bytes(), &alerts); err != nil {
			t.Fatal(err)
		}
		if len(alerts) != 1 || alerts[0].Group != "host:new" || alerts[0].Investigation.Source != "logs" || alerts[0].Investigation.Query != "service:api host:new" || alerts[0].Investigation.From != "2024-03-09T15:00:01Z" {
			t.Fatalf("unexpected alerts: %s", out)
		}
	})
	t.Run("read monitor body from application reader", func(t *testing.T) {
		app, _, _, _ := integrationApp(t, func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["name"] != "From stdin" {
				t.Error("unexpected monitor body")
			}
			_, _ = io.WriteString(w, `{"id":123}`)
		})
		app.Reader = strings.NewReader(`{"name":"From stdin","type":"log alert","query":"logs(*) > 0"}`)
		if err := app.Run([]string{"ddogo", "monitors", "create", "--request-file", "-"}); err != nil {
			t.Fatal(err)
		}
	})
}

func TestAuthUsesInjectedStoreAndSharedResolution(t *testing.T) {
	app, out, _, store := integrationApp(t, nil)
	t.Setenv("DD_API_KEY", "env-api")
	t.Setenv("DD_APP_KEY", "env-app")
	if err := app.Run([]string{"ddogo", "--output", "json", "--dd-api-key", "flag-api", "--profile", "work", "auth", "status"}); err != nil {
		t.Fatal(err)
	}
	var status struct {
		Effective struct {
			APIKeyFrom     string `json:"api_key_from"`
			AppKeyFrom     string `json:"app_key_from"`
			Site, SiteFrom string
		}
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if store.profile != "work" || status.Effective.APIKeyFrom != "flag_or_env" || status.Effective.AppKeyFrom != "flag_or_env" || status.Effective.Site != "datadoghq.eu" {
		t.Fatal("unexpected effective authentication")
	}
	for _, secret := range []string{"env-api", "env-app", "flag-api", "test-api", "test-app"} {
		if strings.Contains(out.String(), secret) {
			t.Fatal("auth status exposed a credential")
		}
	}
}

func TestAuthLoginAndLogout(t *testing.T) {
	app, out, _, store := integrationApp(t, nil)
	if err := app.Run([]string{"ddogo", "--profile", "work", "auth", "login", "--non-interactive", "--api-key", "new-api", "--app-key", "new-app"}); err != nil {
		t.Fatal(err)
	}
	if store.creds.APIKey != "new-api" || store.creds.AppKey != "new-app" || store.profile != "work" || strings.Contains(out.String(), "new-api") {
		t.Fatal("login did not persist credentials safely")
	}
	logout := New("test", commands.Dependencies{Store: store})
	logout.Writer, logout.ErrWriter = io.Discard, io.Discard
	if err := logout.Run([]string{"ddogo", "--profile", "work", "auth", "logout"}); err != nil {
		t.Fatal(err)
	}
	if store.creds != (auth.Credentials{}) {
		t.Fatal("logout did not remove credentials")
	}
}

type cancelOnLoadStore struct {
	*memoryStore
	cancel context.CancelFunc
}

func (s cancelOnLoadStore) Load(profile string) (auth.Credentials, error) {
	s.cancel()
	return s.memoryStore.Load(profile)
}

func TestCanceledAuthCommandsDoNotMutateStore(t *testing.T) {
	for _, args := range [][]string{
		{"auth", "login", "--non-interactive", "--api-key", "new-api", "--app-key", "new-app"},
		{"auth", "logout"},
	} {
		t.Run(args[1], func(t *testing.T) {
			app, out, _, store := integrationApp(t, nil)
			before := store.creds
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			err := app.RunContext(ctx, append([]string{"ddogo"}, args...))
			if !errors.Is(err, context.Canceled) || store.creds != before || out.Len() != 0 {
				t.Fatal("canceled auth command mutated the store or reported success")
			}
		})
	}
}

func TestLoginCanceledDuringLoadDoesNotSave(t *testing.T) {
	_, _, _, store := integrationApp(t, nil)
	before := store.creds
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app := New("test", commands.Dependencies{Store: cancelOnLoadStore{memoryStore: store, cancel: cancel}})
	app.Writer, app.ErrWriter = io.Discard, io.Discard
	err := app.RunContext(ctx, []string{"ddogo", "auth", "login", "--non-interactive", "--api-key", "new-api", "--app-key", "new-app"})
	if !errors.Is(err, context.Canceled) || store.creds != before {
		t.Fatal("login saved credentials after its context was canceled")
	}
}

func TestHelpAndInvalidArgumentsDoNotConstructClient(t *testing.T) {
	for _, args := range [][]string{
		{"--help"},
		{"logs", "search", "--help"},
		{"auth", "login", "--help"},
		{"logs", "search", "--query", "*", "--limit", "0"},
		{"rum", "search", "--query", "*", "--from", "bad-time"},
		{"spans", "search", "--query", "*", "--logs-limit", "5"},
		{"errors", "search", "--query", "*", "--limit", "101"},
		{"monitors", "alerts", "--status", "bad-status"},
		{"monitors", "create", "--request-json", "{}", "--request-file", "monitor.json"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			app, _, _, _ := integrationApp(t, nil)
			err := app.Run(append([]string{"ddogo"}, args...))
			help := args[len(args)-1] == "--help"
			if (err == nil) != help {
				t.Fatalf("unexpected command result: %v", err)
			}
		})
	}
}

func TestAPIFailureLeavesStdoutEmpty(t *testing.T) {
	app, out, _, _ := integrationApp(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"errors":["missing logs_read_data permission"]}`)
	})
	err := app.Run([]string{"ddogo", "logs", "search", "--query", "*"})
	var apiErr *datadog.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusForbidden || out.Len() != 0 {
		t.Fatalf("unexpected error/output: %v / %s", err, out)
	}
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestCommandPropagatesWriterError(t *testing.T) {
	app, _, _, _ := integrationApp(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"data":[]}`)
	})
	writeErr := errors.New("output unavailable")
	app.Writer = failingWriter{err: writeErr}
	if err := app.Run([]string{"ddogo", "--output", "json", "logs", "search", "--query", "*"}); !errors.Is(err, writeErr) {
		t.Fatalf("writer error was not propagated: %v", err)
	}
}

func TestCredentialPrecedenceAtCommandBoundary(t *testing.T) {
	app, _, _, store := integrationApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("DD-API-KEY") != "flag-api" || r.Header.Get("DD-APPLICATION-KEY") != "env-app" {
			t.Error("flags > environment > store precedence was not respected")
		}
		_, _ = io.WriteString(w, `{"data":[]}`)
	})
	t.Setenv("DD_API_KEY", "env-api")
	t.Setenv("DD_APP_KEY", "env-app")
	if err := app.Run([]string{"ddogo", "--dd-api-key", "flag-api", "--profile", "work", "logs", "search", "--query", "*"}); err != nil {
		t.Fatal(err)
	}
	if store.profile != "work" {
		t.Fatal("stored site was not resolved using selected profile")
	}
}

func TestCommandCancellation(t *testing.T) {
	started := make(chan struct{})
	app, out, _, _ := integrationApp(t, func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(started)
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
			t.Error("server request was not canceled")
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- app.RunContext(ctx, []string{"ddogo", "logs", "search", "--query", "*"}) }()
	select {
	case <-started:
		cancel()
	case <-time.After(5 * time.Second):
		t.Fatal("request did not start")
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || out.Len() != 0 {
			t.Fatalf("unexpected cancellation result: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("command did not stop after cancellation")
	}
}
