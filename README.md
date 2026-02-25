# ddogo

Lightweight CLI for consuming Datadog logs from the command line.

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
