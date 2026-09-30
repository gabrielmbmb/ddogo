# Architecture

Keep ddogo idiomatic Go and as simple as possible. Prefer functions and concrete
values; introduce an interface, package, or layer only when it solves a current
problem. Do not add pass-through services, a dependency-injection framework, or
speculative abstractions for future Datadog endpoints.

## Responsibilities

- `cmd/ddogo`: process entry point and exit reporting. Keep default OS interrupt
  handling so Ctrl-C also stops blocking input and credential-store operations.
- `internal/cli`: application construction and production dependency defaults.
- `internal/cli/commands`: flags, CLI-specific validation, request construction,
  and presentation. Use the app's Reader, Writer, and ErrWriter, not process-global
  streams.
- `internal/config`: framework-independent configuration validation and
  supplied > stored > default resolution. The CLI framework handles flags >
  environment precedence. Auth status uses the same resolution logic.
- `internal/auth`: credentials and OS keyring persistence. Keep the existing
  Store interface so commands can use an in-memory store in tests.
- `internal/datadog`: typed API requests/responses, endpoint mapping, pagination,
  and shared HTTP transport. Keep domain clients in focused files in this package.
- `internal/spans`: span search and optional log-enrichment orchestration.
- `internal/monitors`: pure alert-group derivation and best-effort investigation
  context. These derived models belong here, not in the HTTP client.
- `internal/output`: human-readable and JSON presentation with stable fields.

The CLI may depend on application operations and API clients. Application
workflows must not depend on the CLI framework or process-global streams. Do not
copy every Datadog model into a second representation just to enforce a diagram.

## Dependencies and tests

`cli.New` accepts `commands.Dependencies`: a credential Store, a clock, and a
client factory. Zero values select production defaults. The factory runs only
when a command needs the API, so help and auth commands do not require API keys.
Tests supply a fixed clock, an in-memory store, and a client pointing to an
`httptest.Server`; app streams capture input, results, and diagnostics.

Keep transport and workflow failures separate. HTTP retries use endpoint-specific
policies: searches and idempotent operations may be replayed, monitor creation
must not be replayed automatically. Span enrichment owns 429 scheduling; its log
requests disable transport-level 429 retries so wait budgets do not multiply.
Transport retry sleeps have a five-second cumulative budget per request, distinct
from the HTTP attempt timeout. `ClientConfig.MaxRetryWait` can override this for
embedded callers and tests. Enrichment bounds each wait by its configured duration
and limits wait attempts per span. If Retry-After exceeds the relevant budget,
return the error instead of sleeping indefinitely or retrying early. API calls
and waits respect context cancellation; auth checks cancellation before mutations.

Authenticated API requests must not follow redirects: even same-origin redirects
can replay writes, and cross-origin redirects can forward Datadog headers. Clone
injected HTTP clients before setting the redirect policy so callers are unaffected.

Before changing flags, JSON shapes, or exit behavior, add a compatibility test.
Before extracting a shared helper, identify actual duplication and keep
endpoint-specific behavior explicit. Run formatting/linting, `go test ./...`,
`go test -race ./...`, and `go vet ./...`.
