# Changelog

All notable changes to Stromboli will be documented here.

## [Unreleased]

_(empty — add new entries here as PRs land)_

---

## [0.5.7-alpha] - 2026-05-06

### Documentation

- **Receiver workflow template hardened** in response to two real-world failures found while landing the first receiver on `stromboli-go`: (1) the cross-repo release lookup now uses `curl` against the public REST API instead of `gh release view --repo`, which 404s because `GITHUB_TOKEN` in the SDK runner is scoped to its own repo; (2) it hits `/releases` (plural) instead of `/releases/latest`, which 404s on a repo that only ships `-alpha` / `-beta` tags. An inline warning calls out the repo-level "Allow GitHub Actions to create and approve pull requests" toggle, which the receiver workflow needs even with `pull-requests: write`. (#111)

### Dependencies

- `google.golang.org/grpc` 1.80.0 → 1.81.0 — bug fixes in xDS resource validation and HTTP/2 stream handling; pool HTTP/2 framer read buffers to reduce idle memory consumption (Linux ALTS + non-encrypted transports). New experimental SNI/SAN validation behind `GRPC_EXPERIMENTAL_XDS_SNI`. Minimum Go version raised to 1.25 (already met). (#112)
- GitHub Actions group bumps moving runners from the deprecated Node 20 to Node 24: `docker/setup-buildx-action` v3 → v4, `docker/build-push-action` v6 → v7, `golangci/golangci-lint-action` v8 → v9. Requires Actions Runner v2.327.1+. (#113)

---

## [0.5.6-alpha] - 2026-05-01

### Fixed

- **`notify-sdks` fan-out no longer 404s on n8n.** v0.5.5-alpha surfaced that `n8n-nodes-stromboli` is a local-only repo and has never been pushed to GitHub — its matrix iteration failed with HTTP 404 and propagated up to mark the whole release run as failed (the GitHub release artefacts still landed correctly). Removed it from the matrix with a TODO comment so it can be re-added in one line once the repo is published. (#109)

### Documentation

- `n8n-nodes-stromboli` is now marked "coming soon" everywhere it appears (home page, SDKs page, README, sdk-contract intro) — no broken links to a non-existent GitHub URL. (#109)

### Ecosystem

- **First receiver workflow landed on `stromboli-go`** ([tomblancdev/stromboli-go#22](https://github.com/tomblancdev/stromboli-go/pull/22)). It listens on the `stromboli-released` dispatch (and `workflow_dispatch` for manual triggering), records the version to `STROMBOLI_COMPAT`, and opens a `chore: sync to stromboli vX.Y.Z` PR. Codegen is intentionally TODO — the point of landing the minimal receiver now is to make the dispatch path observable in the Actions tab, where unhandled `repository_dispatch` events leave no trace.

---

## [0.5.5-alpha] - 2026-05-01

### Added

- **[SDKs & Integrations](sdks.md) page** — landing page for the four official client libraries (`stromboli-go`, `stromboli-ts`, `mcp-server-stromboli`, `n8n-nodes-stromboli`) with a "picking the right one" guide, version-compatibility notes, and a "build your own" pointer to the OpenAPI spec + receiver-workflow template. (#107)

### Documentation

- Home page (`docs/index.md`) gets a "Talk to it from your stack" section linking each SDK; until now the docs talked about the API surface as if everyone would hit it via curl. (#107)
- Top-level `README.md` shows the SDKs table near the top of the GitHub landing page so it's visible without scrolling. (#107)

---

## [0.5.4-alpha] - 2026-05-01

### Added

- **Release fan-out to client SDKs.** When a tag is pushed and the release workflow finishes, a `notify-sdks` job sends a `repository_dispatch` (`event_type=stromboli-released`) to each registered SDK repo with the new version + swagger URL. SDKs wire the dispatch into their own CI to regenerate typed clients and open a `chore: sync to stromboli vX.Y.Z` PR — auto-mergeable when the diff is mechanical, held for review when it isn't. Targets: `stromboli-go`, `stromboli-ts`, `mcp-server-stromboli`, `n8n-nodes-stromboli`. Protocol + receiver template documented at [SDK Release Fan-out](development/sdk-contract.md). (#104)
- Fan-out skips gracefully with a warning when `SDK_DISPATCH_TOKEN` (the cross-repo PAT) isn't configured — a release never fails because of downstream tooling.

---

## [0.5.3-alpha] - 2026-05-01

### Added

- **`disable_idle_timeout` on `POST /agents`** — opt-out from the per-agent idle watchdog for service-style deployments where the caller owns lifecycle via explicit `DELETE`. When set, the watchdog goroutine isn't started and the agent runs until DELETE or server shutdown. `Snapshot.idle_timeout_disabled` (omitempty) surfaces the flag back to observers, and stromboli logs a loud `WARN` on spawn so a forgotten long-lived agent is visible. Use only when the lifecycle is genuinely external — without the watchdog, a buggy caller can leak an agent indefinitely. (#102)

---

## [0.5.2-alpha] - 2026-05-01

### Fixed

The `/agents` endpoint shipped in v0.5.0-alpha but had never been exercised end-to-end. Five distinct bugs in the spawn path, all surfacing as `signal: killed` or `exit status 1` with no transcript to debug from. Agents now work as designed — sub-second turn latency in a warm container after the initial boot.

- **Session subdirectory not pre-created** — Podman's `-v <host>:<container>` requires the host path to exist before mount, so the spawner died with `statfs ...: no such file or directory`. `buildAgentArgv` now `MkdirAll`s the per-session bind-mount source. (#100)
- **Process bound to HTTP request context** — `agent.Manager.Create` passed the gin handler's request context all the way to `exec.CommandContext`, so the agent process was SIGKILLed the instant the spawn handler returned 201 Created. Switched to `context.Background()`; agent lifetime is owned by `Manager.Stop` / `StopAll` / `watchIdle`. (#100)
- **Missing `claude` prepend on argv** — the agent image's entrypoint passed our flags to `node` (the image's default binary) instead of `claude`, so `node` printed `bad option: --input-format` and exited. (#100)
- **Container ran as root** — without `--userns=keep-id` and `--user $UID:$GID`, claude refuses `--dangerously-skip-permissions` for the root user (security feature). Added both. (#100)
- **`--verbose` missing** — claude requires it whenever `--output-format stream-json` is set. (#100)

### Changed

- **`req.Claude` now threaded through to `/agents`.** Was parsed off the wire and silently dropped — operators sending `claude.model`, `claude.effort`, `claude.allowed_tools`, `claude.prompt_caching_ttl`, etc. to `/agents` got the CLI defaults while the same fields worked fine on `/run`. Refactored `runner.PodmanRunner.applyClaudeOptions` and `applyClaudeEnvVars` into package-level helpers in `internal/claude` (`claude.ApplyOptions` and `claude.EnvVars`); both `/run` and `/agents` now share one option-threading path. Three flags are pinned regardless of caller input — `--input-format`, `--output-format`, `--verbose` — because the agent dispatcher reads stdout line-by-line as JSON; flipping output to `text` would silently break every subscriber. (#100)

### Tests

- Per-bug regression coverage in `cmd/stromboli/main_test.go` for `buildAgentArgv` (rejects un-creatable session dir, claude prepend, `--userns=keep-id` + `--user UID:GID`, `--verbose`, stream-json input/output pinned even when caller requests "text"/"json", `-w` lands before image, claude options threaded through, env-var-only options reach podman `-e`). (#100)
- New `internal/claude/options_test.go` — per-field coverage of `ApplyOptions` and `EnvVars` (model, effort, permissions, tools, budget/turns pointer semantics, prompt-cache TTL recognised values, Bedrock tier, PowerShell tool). (#100)

### Known limitations

- `GET /sessions/{id}/messages` does not return persistent-agent transcripts. Claude's CLI in stream-json mode runs in print mode which is intentionally ephemeral — events flow live via SSE only, nothing is persisted to `.claude/projects/<encoded-cwd>/<session>.jsonl`. Subscribe to `GET /agents/{id}/stream` for real-time replay.

---

## [0.5.1-alpha] - 2026-05-01

### Documentation

- New [persistent agents guide](guides/persistent-agents.md) — end-to-end coverage of the `/agents/*` API: when to use a long-lived process vs. `/run`, lifecycle state machine, idle-timeout sizing, SSE event types, and a worked on-call-bot example. (#97)
- New [performance & cost tuning guide](guides/performance-tuning.md) — bundles the `claude.*` cost knobs that were scattered across the endpoints reference: token usage / estimated cost on `RunResponse.usage`, effort levels, prompt caching TTL (`5m` / `1h`), Bedrock service tier, PowerShell tool. Combination template for a low-cost persistent agent. (#97)
- New [webhook security guide](guides/webhook-security.md) — Go / Python / Node verifier snippets for HMAC-signed callbacks, retry semantics (timestamp + signature reused), secret-rotation playbook, and the related trusted-proxy allowlist. (#97)
- New [v0.5.0 upgrade guide](getting-started/upgrade-to-0.5.0.md) — TL;DR migration checklist for the four breaking default changes (auth on, metrics on localhost, tracing TLS, JSON logs), per-change deep-dives, dropped Windows binaries, sanity-check curl sequence. (#98)
- [Production hardening](security/production.md) refreshed for v0.5.0 — required-checklist updated with trusted proxies and webhook signing, new sections on token blacklist backend choice / webhook signing / X-Forwarded-For trust, alert-target additions, version pin example bumped. (#98)
- Cross-links so the new content is reachable: home page feature grid, "How It Works" execution-modes table grew from 3 to 4 entries, `running-agents.md` ↔ `persistent-agents.md`, `sessions.md` finally documents the `UserPromptSubmit` title hook from #83/#86. (#97)

### Changed

- **OpenAPI: `agent.CreateRequest.claude` is now fully typed.** Previously rendered as `additionalProperties: {}` (opaque object), losing the entire Claude CLI option schema. Switched to `*types.ClaudeOptions` — the same struct `RunRequest.claude` uses — so the spec `$ref`s a single shared definition across both endpoints. Pure schema win; the field was inert (never threaded through to the argv builder yet). (#96)
- **OpenAPI: `DELETE /jobs/{id}` returns a typed `JobCancelResponse`** instead of the unhelpful `map[string]interface{}` it previously generated. (#96)

---

## [0.5.0-alpha] - 2026-05-01

### Added

#### Persistent Agents
- New `/agents/*` endpoint family for long-lived Claude processes with stream-json I/O — sub-second turn latency for event-driven workloads (sensor buses, on-call bots). See [API endpoints](api/endpoints.md#persistent-agents). (#60, #73)
- Per-session token usage and estimated USD cost surfaced on `RunResponse.usage` (read from session JSONL, best-effort). (#56, #72)
- `claude.effort` request field exposes the upstream CLI's thinking/agentic complexity level (`low`, `medium`, `high`, `xhigh`, `max` — accepted subset depends on model). (#74, #84)
- Env-var passthrough for runtime tuning: `claude.prompt_caching_ttl` (`5m`/`1h`), `claude.bedrock_service_tier`, `claude.enable_powershell_tool`. (#76, #77, #81, #85)
- Session titles surfaced from a `UserPromptSubmit` hook returning `hookSpecificOutput.sessionTitle`. `GET /sessions` now returns `{id, title}` records. (#83, #86)
- New `GET /sessions/:id/messages/:message_id` endpoint to fetch a single message without re-reading the whole transcript.
- **Webhook HMAC-SHA256 signing.** Outgoing async-job webhooks now carry `X-Stromboli-Signature` (`sha256=<hex>`) and `X-Stromboli-Timestamp` headers when `STROMBOLI_WEBHOOK_SIGNING_SECRET` is set. Receivers verify with the new `webhook.Verify()` helper. Retries reuse the same timestamp+signature so the receiver's freshness window evaluates the original send time. Empty secret = unsigned (legacy/dev) — the server logs a `WARN` on startup so the missing config is loud. (#88)
- **Pluggable token blacklist** with `memory` (default — fastest, lost on restart) and `bolt` (durable single-file store) backends. Switch via `STROMBOLI_AUTH_BLACKLIST_BACKEND`; the interface is designed so future Redis/Postgres backends slot in without API or middleware churn. New env vars: `STROMBOLI_AUTH_BLACKLIST_BOLT_PATH`, `STROMBOLI_AUTH_BLACKLIST_CLEANUP_INTERVAL`. (#92)
- **Trusted-proxy allowlist for `X-Forwarded-For`** via `STROMBOLI_RATE_LIMIT_TRUSTED_PROXIES` (comma-separated CIDRs or bare IPs). Defaults to empty: forwarding headers are ignored entirely so an internet-facing client can't spoof its rate-limit bucket. When the immediate peer is in the allowlist, the leftmost XFF entry is used as the client IP. (#88)

### Changed

- **BREAKING (operational): Auth enabled by default.** `STROMBOLI_AUTH_ENABLED` now defaults to `true`. The server fails fast at startup if `STROMBOLI_JWT_SECRET` is empty, shorter than 32 chars, or matches a known placeholder. Existing setups must either set a real JWT secret or explicitly opt out with `STROMBOLI_AUTH_ENABLED=false`. (#45, #64)
- **Logs are JSON by default** so log aggregators can parse them without preprocessing. Set `STROMBOLI_LOG_FORMAT=text` for human-friendly output during local dev. (#47, #65)
- **Log level configurable** via `STROMBOLI_LOG_LEVEL` (`debug`/`info`/`warn`/`error`). (#48, #67)
- **Tracing TLS by default**: `STROMBOLI_TRACING_INSECURE` now defaults to `false`. Plaintext OTLP only in dev. (#49, #70)
- **Metrics on a separate listener bound to localhost** (`127.0.0.1:9090` by default) — never co-located with the public API port. Run a Prometheus sidecar or override `STROMBOLI_METRICS_ADDRESS` if you need cross-pod scraping. (#46, #66)

### Fixed

- Dev compose now uses a named volume for sessions (was a bind mount that dropped state across `compose down`). (#50, #69)
- `.dockerignore` slims the build context. (#51, #68)
- **`forwardLines` tolerates oversized agent stdout** (>10 MiB lines). Previously `bufio.Scanner` permanently failed on the first oversized line; with no reader draining the OS pipe, Claude's next stdout write blocked and `cmd.Wait` hung forever. Now emits a single truncation marker and keeps reading the next line. (#91)
- **Error wrapping (`%w`) in `compose/validator`, `runner/cleanup`, `secrets/registry`** — was returning raw `err` values that hid which operation failed in the chain. (#89)
- **`watchIdle` exits promptly on agent shutdown** instead of holding the goroutine alive for up to 30 s waiting for the next ticker fire. New per-agent `done` channel that `markExited` closes via `sync.Once`. (#89)
- **`X-RateLimit-Remaining` reports available tokens, not consumed.** Formula was `Burst() - Tokens()` which inverted the semantics the header is supposed to convey. Now `int(l.Tokens())`, floored at zero. (#89)
- Logout (`POST /auth/logout`) now returns 503 instead of silently 200-ing when the configured blacklist is nil or its `Add` errors. The auth middleware fails closed on a non-nil error from the blacklist backend (e.g. transient bolt I/O). (#92)

### Infra

- Kubernetes manifests under `deployments/kubernetes/` (namespace, ConfigMap, Secret example, Deployment, Service, Kustomization).
- Prometheus alert rules under `deployments/grafana/`.
- Dev image notes in `deployments/`. (#53, #54, #55, #71)
- CI pipeline for lint, tests, and Docker build. (#44, #63)

#### Lifecycle Hooks
- **OnCreateCommand**: Run commands once when session is first created (e.g., `pip install`)
- **PostCreate**: Run commands after OnCreateCommand completes (e.g., build steps)
- **PostStart**: Run commands on every container start (e.g., start background services)
- **Hooks Timeout**: Configurable timeout for hook execution (`hooks_timeout`)
- Hooks are chained with fail-fast behavior - if any hook fails, execution stops
- Shell escaping for all hook arguments to prevent injection attacks
- Documentation: [Lifecycle Hooks Guide](guides/lifecycle-hooks.md)

#### Compose Environments
- **Multi-service environments**: Run Claude agents in Docker/Podman Compose stacks
- **Service selection**: Specify which service Claude runs in via `environment.service`
- **Health check waiting**: Stromboli waits for all services to become healthy
- **Stack lifecycle management**: Automatic cleanup on session destroy or TTL expiry
- **Security validation**: Blocks privileged containers, host network, and dangerous configurations
- Configuration options: `allow_privileged`, `allow_host_network`, `allow_host_volumes`
- Timeout configuration: `build_timeout`, `health_timeout`, `stack_ttl`
- Documentation: [Compose Environments Guide](guides/compose-environments.md)

#### Image Discovery API
- **GET /images**: List all local images sorted by compatibility rank
- **GET /images/:name**: Inspect a specific image with detailed metadata
- **GET /images/search**: Search container registries (Docker Hub, etc.)
- **POST /images/pull**: Pull an image from a registry
- Compatibility ranking system (1-4) to identify Claude-compatible images

### Security

- Compose file validation with security checks for dangerous configurations
- Lifecycle hooks validation with length limits and shell escaping
- TOCTOU protection for compose file parsing
- **JWT algorithm pinned to HS256 explicitly.** The validator used a `*jwt.SigningMethodHMAC` type assertion that silently accepted HS384/HS512 and relied on the library to refuse `alg: none`. Replaced with an explicit `token.Method != jwt.SigningMethodHS256` check that fails closed regardless of library behavior. (#88)
- **Workspace symlink validation fails closed.** `Validator.Validate` previously fell back to the unresolved cleaned path on every `EvalSymlinks` error — including symlink loops, where a crafted A→B→A pair could pass the allowlist check via the pre-resolution path. Now only `fs.ErrNotExist` is tolerated (the workspace is created later); loops, permission errors, and other failures are rejected up front. (#88)

### Tests

- New `internal/agent/process_test.go` covers the previously-untested `processSpawner` end-to-end against real subprocesses: stdout fan-out, stderr prefixing, stdin `Send`, escalating `Stop` (stdin-close → SIGTERM → SIGKILL), oversized-line truncation, empty-argv rejection, and start-failure error wrapping. (#90, #91)
- `internal/job/job_test.go` cleanup-removes-X subtests use `require.Eventually` instead of fixed `time.Sleep(50ms)` barriers — finishes in ~10 ms each and tolerant of slow CI schedulers. (#90)
- New trusted-proxy and signing-verify tests cover the security additions above. (#88)
- Bolt-backed blacklist tests cover persistence across close/reopen, startup cleanup, lazy expiry filtering, and expiry overwrite on re-`Add`. (#92)

---

## [0.3.0-alpha] - 2026-01-31

### Changed

- **BREAKING: Renamed `workspace` to `workdir`**:
  - `workdir` sets the **working directory** inside the container (e.g., `/workspace`)
  - Use `podman.volumes` to mount host directories into the container
  - Example migration:
    ```json
    // Before (v0.2.0)
    {"workspace": "/home/user/project"}

    // After
    {
      "workdir": "/workspace",
      "podman": {"volumes": ["/home/user/project:/workspace"]}
    }
    ```

- **BREAKING: Default-deny volume security**: When `allowed_volumes` is empty, all volume mounts are now **DENIED** by default (was: allow all). Set `STROMBOLI_AGENT_ALLOW_ALL_VOLUMES=true` for development.

- **Agent entrypoint simplified**: Removed `claude` from entrypoint command. The runner now always prepends `claude` when `MOUNT_CLAUDE_CLI=true`.

### Added

- **Workdir auto-creation**: If `workdir` doesn't exist in the container, it's automatically created (configurable via `STROMBOLI_AGENT_WORKDIR_AUTO_CREATE`)
- **Volume validation**: Volume host paths are validated against `allowed_volumes` allowlist (`STROMBOLI_AGENT_ALLOWED_VOLUMES`)
- **Sessions host path**: New `STROMBOLI_AGENT_SESSIONS_HOST_DIR` config for containerized deployments where Stromboli runs inside a container
- **Symlink bypass prevention**: Host paths are resolved via `filepath.EvalSymlinks()` before validation
- **Container path blocklist**: Sensitive container paths are blocked (`/etc`, `~/.claude`, `~/.ssh`, `~/.aws`, etc.)
- **Mount options validation**: Only safe mount options allowed (`ro`, `rw`, `z`, `Z`, `noexec`, `nosuid`, `nodev`, etc.)
- **Workdir character validation**: Workdir paths validated for shell-safe characters only

### Security

- Defense-in-depth volume validation with multiple security layers
- Explicit error messages for security rejections (e.g., "Alpine/musl-based images not supported")

---

## [0.2.0-alpha] - 2026-01-30

### Added

#### Release & Distribution
- **Cross-platform binaries**: Linux, macOS, Windows (amd64 + arm64)
- **Docker images**: Published to `ghcr.io/tomblancdev/stromboli`
- **Agent image**: Separate `ghcr.io/tomblancdev/stromboli-agent` with Claude CLI
- **Native cross-compilation**: Fast ARM64 builds (no QEMU emulation)

#### Image Architecture
- **CLI image auto-pull**: Automatically pulls Claude CLI image on startup if missing
- **Dynamic image support**: Mount Claude CLI into any glibc-based container (Python, Node, Go, etc.)
- **Image compatibility checking**: Warns about incompatible Alpine/musl images

#### Documentation
- **Examples & Use Cases**: Multi-language API clients (Python, JavaScript, Go, curl)
- **CI/CD Integration**: Service container approach for full codebase access
- **Security Guide**: Threat model, TLS setup, audit logging, production checklist
- **Troubleshooting Guide**: Error reference, debugging tips, FAQ
- **OpenAPI Reference**: Interactive Swagger UI, ReDoc, downloadable specs
- **Contributing Guide**: Code architecture, request flow diagrams, testing patterns
- **Mermaid diagrams**: Visual architecture and flow diagrams

### Changed
- **Configuration**: All settings now documented with environment variables
- **Install files**: Comprehensive docker-compose.yml and stromboli.example.yaml

### CI/CD
- **Release workflow**: Automated binary + Docker builds on version tags
- **Agent image workflow**: Auto-builds when Dockerfile.claude-cli changes
- **OpenAPI validation**: Ensures specs are up-to-date
- **Versioned documentation**: Each release has frozen docs + OpenAPI specs

### Fixed
- Docker ARM64 build performance (was 15-20 min, now ~2 min)
- Documentation link validation

---

## [0.1.5-alpha] - 2025-01-26

### Added
- **Credentials Sync**: Automatic synchronization of Claude credentials with Podman secrets
- **Generic Secrets Injection**: Mount Podman secrets as environment variables via `secrets_env`
- **Input Validation**: Comprehensive validation for secrets environment variables
- **/secrets Endpoint**: List available Podman secrets via API

### Security
- Block dangerous environment variables (LD_PRELOAD, LD_LIBRARY_PATH)
- Environment variable name validation (must match `^[a-zA-Z_][a-zA-Z0-9_]*$`)
- Maximum 50 secrets per request

## [0.1.4-alpha] - 2025-01-25

### Added
- **Dynamic Container Images**: Support for multiple container images with pattern allowlist
- **Version Info**: `/version` endpoint and startup version logging
- **Container Naming**: Unique container names with `stromboli-` prefix
- **Orphan Cleanup**: Automatic cleanup of orphaned containers on startup

### Fixed
- Version injection into Docker server image during build

## [0.1.3-alpha] - 2025-01-24

### Added
- Initial public release
- Core API for running Claude Code agents
- Session management (create, resume, destroy)
- Async job execution with polling
- Workspace mounting with allowlist security
- JWT authentication support
- Rate limiting middleware
- Health check endpoint

### Security
- Container isolation via Podman
- Workspace allowlist validation
- Read-only credential mounting
