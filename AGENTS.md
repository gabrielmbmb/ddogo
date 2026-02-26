# AGENTS.md

Agent instructions for this repository.

## Project overview

`ddogo` is a lightweight, modern CLI focused on consuming logs from Datadog via the Logs Search API.

Primary near-term goal:
- Reliable, ergonomic log consumption from Datadog Logs Search API.

Probable future scope:
- Additional Datadog observability surfaces (e.g., traces, APM, metrics, events) while keeping the CLI cohesive and minimal.

## Product principles

1. **CLI-first UX**
   - Fast startup, clear defaults, predictable behavior.
   - Great `--help`, examples, and discoverability.
   - Output suitable for both humans and automation.

2. **Unix philosophy**
   - Compose well with pipes and tools (`jq`, `grep`, etc.).
   - Separate structured output (`json`) from readable output (`table`/`pretty`).

3. **Reliability over novelty**
   - Handle API errors, rate limits, retries, and partial failures gracefully.
   - Exit codes are meaningful and stable.

4. **Security and privacy by default**
   - Never print secrets/tokens.
   - Avoid accidental leakage of sensitive log content.

5. **Extensible architecture**
   - Keep Datadog client and domain logic decoupled from CLI wiring.
   - Design command boundaries that can grow to traces/APM without rewrites.

## Engineering standards

- Prefer small, focused modules and pure functions where practical.
- Use explicit types and schemas for API request/response boundaries.
- Keep command handlers thin; move business logic to reusable services.
- Add tests for parsing, filtering, pagination, and error handling paths.
- Include integration-style tests for Datadog API interaction with mocks/fixtures.
- Keep dependencies minimal and well-maintained.

## CLI behavior guidelines

- Support global options for:
  - auth (env-first),
  - output format,
  - verbosity/logging,
  - pagination/limit/window controls.
- Provide `--output json` for machine-readable output with stable fields.
- Human-readable mode should be concise and color-safe (respect `NO_COLOR`).
- Use stderr for diagnostics/errors, stdout for primary command output.
- Use non-zero exit codes for failures; document all exit codes.

## Datadog integration guidelines

- Respect Datadog API limits and implement retry/backoff for transient failures.
- Make time windows explicit and timezone-safe (default UTC unless configured).
- Support incremental consumption patterns (cursor/time-based where available).
- Validate and sanitize user-provided queries/options before requests.

## Datadog Logs endpoints in scope

Primary endpoints for `ddogo` (log consumption):

1. **POST `/api/v2/logs/events/search`** (preferred)
   - Main endpoint for `logs search`.
   - Supports rich filters (`query`, `from`, `to`, `indexes`, `storage_tier`), sorting, and cursor pagination (`page.limit`, `page.cursor`).
   - Use `meta.page.after` for incremental pagination.

2. **GET `/api/v2/logs/events`** (optional companion)
   - Query-string variant of v2 search.
   - Useful for simple requests and compatibility with `links.next` URL pagination.

3. **POST `/api/v1/logs-queries/list`** (legacy fallback only)
   - Keep for backward compatibility if needed.
   - Pagination model differs (`startAt` request, `nextLogId` response).
   - Prefer v2 for new features and stable forward compatibility.

Authentication/permissions for search endpoints:
- Require both `DD_API_KEY` and `DD_APP_KEY`.
- Require Datadog permission: `logs_read_data`.

Current out-of-scope endpoints (unless explicitly adding new commands):
- Log ingestion: `POST /v1/input`, `POST /api/v2/logs`
- Log analytics aggregate: `POST /api/v2/logs/analytics/aggregate`

Retry/error handling policy for search:
- Retry with backoff on transient failures (`429`, `408`, `5xx`, and transport timeouts).
- Do not retry client/auth errors (`400`, `401`, `403`) without user action.

## Configuration and auth

- Prefer environment variables for secrets and sensitive settings.
- Support explicit config file paths and the following precedence rules:
  1. CLI flags
  2. Environment variables
  3. Config file
  4. Built-in defaults
- Never commit real tokens or example values that look real.

## Documentation requirements

- Every user-facing command must have:
  - clear description,
  - argument/flag docs,
  - at least one practical example.
- Maintain a concise README quickstart.
- Keep a changelog with user-visible behavior changes.

## Quality gates (minimum)

Before merging, ensure:
- formatting/linting pass,
- tests pass,
- basic help output is accurate,
- no secrets in code/tests/docs,
- error messages are actionable.

## Future-proofing (for traces/APM/etc.)

When adding new domains:
- Reuse shared transport/auth/config infrastructure.
- Keep domain-specific logic isolated under clear module boundaries.
- Preserve backward compatibility for existing commands/flags unless versioned.

## Collaboration notes for agents

- Prefer iterative, reviewable changes.
- After modifying code or tests, always run the test suite (`go test ./...`) before reporting completion or committing.
- State assumptions explicitly in PR/commit notes.
- If requirements are ambiguous, ask targeted questions before large refactors.
- Do not silently change UX/flags/output contracts.

## Project decisions

The following decisions are currently locked:
- Language/runtime: **Go**
- CLI framework: **urfave/cli**
- Packaging/distribution strategy: **popular channels with Homebrew support** (plus additional channels as needed)
- Minimum supported OS/platforms: **Linux, macOS (Darwin), Windows**
- Logging/output defaults: **pretty** for humans by default, with `--output json` for automation
- Versioning/release automation: **Semantic Versioning + GoReleaser + GitHub Releases**
