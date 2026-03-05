# ddogo

Lightweight CLI for consuming Datadog logs and spans from the command line.

## Installation

### Homebrew

```bash
brew tap gabrielmbmb/homebrew-tap
brew install ddogo
```

### Go

```bash
go install github.com/gabrielmbmb/ddogo/cmd/ddogo@latest
```

## Authentication

Recommended local setup is storing credentials in your OS secure keychain:

```bash
ddogo auth login
```

This saves credentials under the `default` profile and uses them automatically.

You can still use environment variables (recommended for CI/non-interactive usage):

```bash
export DD_API_KEY=<your-api-key>
export DD_APP_KEY=<your-app-key>
export DD_SITE=datadoghq.eu  # optional, defaults to datadoghq.com
```

Or pass keys directly as flags:

```bash
ddogo --dd-api-key <key> --dd-app-key <key> logs search --query '...'
```

Auth precedence is: **flags > environment > secure store > defaults**.

Useful auth commands:

```bash
ddogo auth status
ddogo auth logout
ddogo auth login --profile work
```

## Commands

### `auth`

Manage persisted credentials in the OS keychain.

```bash
ddogo auth login
ddogo auth status
ddogo auth logout
```

### `logs search`

Search logs within a time window.

```
ddogo logs search --query <query> [--from <time>] [--to <time>] [--limit <n>]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--query`, `-q` | Datadog log query (required) | — |
| `--from` | Start time — RFC3339 or relative (e.g. `-15m`, `-1h`) | `-15m` |
| `--to` | End time — RFC3339 or `now` | `now` |
| `--limit` | Maximum number of logs to return | `100` |

**Examples:**

```bash
# Errors from the last 15 minutes
ddogo logs search --query 'service:api status:error'

# Logs from the last hour, up to 500 results
ddogo logs search --query 'env:prod' --from -1h --limit 500

# Specific time range
ddogo logs search --query 'service:worker' \
  --from 2026-02-25T07:00:00Z \
  --to 2026-02-25T08:00:00Z

# Machine-readable output
ddogo logs search --query 'status:error' --output json | jq '.[] | .message'
```

Datadog logs query warnings/timeouts are printed to `stderr` while logs results remain on `stdout`.

### `spans search`

Search spans within a time window.

```
ddogo spans search --query <query> [--from <time>] [--to <time>] [--limit <n>]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--query`, `-q` | Datadog span query (required) | — |
| `--from` | Start time — RFC3339 or relative (e.g. `-15m`, `-1h`) | `-15m` |
| `--to` | End time — RFC3339 or `now` | `now` |
| `--limit` | Maximum number of spans to return | `100` |
| `--with-logs` | Fetch correlated logs for each returned span | `false` |
| `--logs-query` | Additional Datadog log filter used with `--with-logs` | — |
| `--logs-from` | Correlated logs start time; defaults to `--from` | inherited |
| `--logs-to` | Correlated logs end time; defaults to `--to` | inherited |
| `--logs-limit` | Correlated logs max results per span | `20` |
| `--logs-rate-limit-mode` | 429 handling for correlated logs: `skip` or `wait` | `skip` |
| `--logs-rate-limit-wait` | Wait duration between retries when mode is `wait` | `30s` |
| `--logs-rate-limit-max-waits` | Max wait+retry cycles on 429 when mode is `wait` | `3` |

**Examples:**

```bash
# Spans from the last 15 minutes
ddogo spans search --query 'service:api'

# Spans with machine-readable output
ddogo spans search --query 'env:prod' --from -1h --output json | jq '.[0]'

# Fetch logs correlated to each span (same query naming style as logs)
ddogo spans search --query 'service:api' --with-logs \
  --logs-query 'status:error' \
  --logs-from -30m \
  --logs-to now \
  --logs-limit 10

# On 429s, wait and retry instead of skipping remaining spans
ddogo spans search --query 'service:api' --with-logs \
  --logs-rate-limit-mode wait \
  --logs-rate-limit-wait 45s \
  --logs-rate-limit-max-waits 5
```

**429 handling when using `--with-logs`:**

- `--logs-rate-limit-mode skip` (default): after a 429, skip log enrichment for remaining not-yet-processed spans and continue returning spans.
- `--logs-rate-limit-mode wait`: on 429, wait `--logs-rate-limit-wait` and retry up to `--logs-rate-limit-max-waits` times.

In both modes, spans are still returned. Per-span enrichment failures are exposed in `logs_error` (JSON output) and warnings are printed to `stderr`.

## Global flags

| Flag | Description | Default |
|------|-------------|---------|
| `--output`, `-o` | Output format: `pretty` or `json` | `pretty` |
| `--dd-api-key` | Datadog API key | `$DD_API_KEY` |
| `--dd-app-key` | Datadog application key | `$DD_APP_KEY` |
| `--site` | Datadog site | `datadoghq.com` |
| `--profile` | Credential profile from secure store | `default` |

## License

[Apache 2.0](LICENSE)
