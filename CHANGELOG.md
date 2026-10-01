# Changelog

## Unreleased

### Changed

- API retries now use jittered backoff with a five-second cumulative retry-sleep
  budget per request. A `Retry-After` that exceeds the remaining budget returns
  the API error instead of blocking indefinitely or retrying early.
- Monitor creation is no longer automatically retried, avoiding duplicate
  monitors after ambiguous failures. Check Datadog before manually retrying a
  creation that timed out.
- Correlated-log enrichment owns its 429 wait/skip policy instead of also
  retrying 429s in the transport. Existing wait limits now correspond to actual
  log-search retry cycles rather than nested retry budgets. A `Retry-After` longer
  than the configured wait produces a nonfatal enrichment failure.
- Ctrl-C retains normal process interrupt termination, including during blocking
  input and credential-store operations. Context cancellation during span
  enrichment returns a failure rather than a successful partial response, and
  canceled auth commands do not start credential-store mutations.
- Authenticated API requests no longer follow HTTP redirects, preventing
  cross-origin credential forwarding and redirected monitor creation. Injected
  HTTP clients keep their original redirect policy outside ddogo.

### Maintenance

- Moved existing API adapters, models, pagination, and adapter tests into the
  `logs`, `spans`, `rum`, `metrics`, `monitors`, and `errortracking` domain packages.
  Datadog transport remains shared; flags, help, JSON output, and retry behavior
  are unchanged. Domain clients are concrete; span-enrichment interfaces belong
  to the workflow that consumes them.
- Extracted monitor alert workflows from CLI wiring without changing JSON fields.
- Made command streams, clock, credential store, and API client factory injectable.
- Removed CLI-framework dependencies from configuration resolution and reused
  the same precedence rules for auth status.
- Added command integration, JSON compatibility, and retry-policy tests.
