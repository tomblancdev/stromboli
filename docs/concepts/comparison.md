# Stromboli vs. the World — an honest comparison

> A fair, unflinching look at where Stromboli sits in the 2026 coding-agent
> sandbox landscape, what it does that nothing else does, where it gets beaten,
> and whether it has any reason to exist at all.

This page is intentionally not a marketing document. If a competitor is better
for your use case, you'll find it called out plainly. The verdict is at the
[bottom](#verdict-is-stromboli-useless).

---

## TL;DR

Stromboli is **a self-hosted REST API that spawns Claude Code agents inside
rootless Podman containers**. That exact sentence — *self-hosted*, *REST*,
*Claude-Code-specific*, *Podman*, *rootless* — is the niche. Move any of those
adjectives and a stronger tool already exists:

- Want a **language SDK** instead of a REST API → use the [Claude Agent SDK]
- Want a **desktop UI** for parallel agents → use [Sculptor], [Conductor], [Crystal], or [Vibe Kanban]
- Want a **managed cloud** runtime → use [Anthropic Managed Agents], [E2B], [Daytona], or [Modal]
- Want **CI integration** only → use the official [claude-code-action]
- Want **MCP-driven** worktree spawning from inside an agent → use [container-use]
- Want a **generic untrusted-code sandbox** → use [E2B], [Daytona], [Northflank], or [microsandbox]

So Stromboli is **not unique as a category** — agent sandboxing is one of the
most crowded niches in dev-tooling right now. It is, however, unique as a
**specific combination**: there are very few projects that expose Claude Code
itself as a remote, multi-tenant, OpenAPI-described HTTP service backed by
Podman with JWT auth, OpenTelemetry traces, Prometheus metrics, and SSE
streaming. If that exact shape matches your problem, Stromboli is one of maybe
two or three reasonable choices. If it doesn't, something else is better.

---

## The landscape, in one map

The 2026 coding-agent isolation space splits into roughly six categories. Pick
the row that matches what you're trying to do, then read across:

| Category | Surface | Examples | Where Stromboli fits |
|---|---|---|---|
| **Built-in agent sandboxes** | OS primitives shipped with the CLI | Anthropic `srt`, Codex CLI Seatbelt/Landlock, Gemini CLI Seatbelt | Stromboli wraps the agent in a *container* on top of these |
| **Container-wrapper CLIs** | `claudebox my-prompt` | [ClaudeBox], sandclaude, codex-container-sandbox, packnplay | Same isolation goal, but CLI-only — no HTTP, no jobs, no auth |
| **Desktop UIs for parallel agents** | Mac app | [Sculptor], [Conductor], [Crystal], [Vibe Kanban] | Different surface — they're for humans clicking buttons; Stromboli is for programs calling APIs |
| **Self-hosted agent runners (REST/SDK)** | HTTP / library | **Stromboli**, [claude-code-runner], [OpenHands], [Claude Agent SDK self-host] | This is Stromboli's actual lane |
| **Hosted sandbox SaaS** | Cloud API | [E2B], [Daytona], [Modal], [Northflank], [Fly Sprites], [Vercel Sandbox], [Blaxel], [Runloop] | Stromboli is the "I won't ship my code to a third party" version |
| **Cloud-run agents** | Anthropic / OpenAI / Cursor | [Managed Agents], Codex Cloud, Cursor background agents, Devin | Stromboli is the OSS, BYO-infra alternative |

Stromboli lives in the **fourth row**. That row has maybe four or five serious
projects. Everything else is adjacent, not competitive.

---

## Closest direct competitors

These are the projects that solve **the same problem in roughly the same shape**:
"I want to call an HTTP endpoint and have a Claude Code agent run somewhere
isolated."

### Anthropic Claude Agent SDK (self-hosted)

[Anthropic's official SDK][Claude Agent SDK] (Python + TypeScript) is the most
serious competitor. The official [hosting guide][Claude Agent SDK self-host]
literally walks you through running it in Docker/gVisor/Firecracker behind an
HTTP/WebSocket endpoint — i.e. the thing Stromboli does, but in your own
process and your own language.

| | Stromboli | Claude Agent SDK |
|---|---|---|
| Built and maintained by | Solo developer | Anthropic |
| Surface | REST + OpenAPI | Python / TS library |
| Isolation | Per-request Podman container | Whatever you wire up (gVisor, Firecracker, container, none) |
| Customising the agent loop | Limited to what `claude --headless` exposes | Full — hooks, MCP, custom tools, mid-loop control |
| Multi-agent in one process | No — one container per request | Yes — multiple sessions in one Python/Node process |
| You write any code | No (just `curl`) | Yes |
| Ops surface | One Go binary + Podman | Whatever you choose to deploy |

**Verdict:** if you're comfortable writing Python or TypeScript, the SDK is
strictly more powerful. Stromboli wins when (a) your caller isn't Python/TS,
(b) you specifically want hard container isolation per request without writing
the orchestration yourself, or (c) you want OpenAPI-described HTTP semantics out
of the box.

### ericvtheg/claude-code-runner

[claude-code-runner] is the only other widely-known **self-hosted Claude Code
runner with a REST endpoint**. It's much smaller in scope: send a prompt,
get a response. No Podman isolation, no async jobs, no SSE, no JWT, no metrics,
no SDKs.

**Verdict:** Stromboli is a strict superset for production use. claude-code-runner
is a strict superset for "I want to read 200 lines of code and understand it."

### OpenHands (formerly OpenDevin)

[OpenHands] is a much **broader** AI software-engineer platform — model-agnostic
(Claude, GPT, DeepSeek, Llama, local Ollama), with browser automation, full
delegation, planning, the works. It does have a Docker-sandboxed runtime and
does expose APIs. 68k GitHub stars, $18.8M Series A.

| | Stromboli | OpenHands |
|---|---|---|
| Scope | Spawn Claude headless, return output | Full agent platform with planner, browser, file editor, multi-agent delegation |
| Model | Claude Code only | Any LLM |
| Footprint | One Go binary, Podman | Docker, multiple services, an actual product |
| Maturity | Solo project, alpha | Funded, large community, prod deployments |
| If you want "Claude as a microservice" | ✅ fits | ❌ overkill |
| If you want "an autonomous developer" | ❌ wrong tool | ✅ fits |

**Verdict:** they're not really the same product. OpenHands is a *destination*;
Stromboli is *plumbing*. If you want plumbing, Stromboli is lighter. If you want
a destination, use OpenHands.

### container-use (Dagger)

[container-use] is the closest peer in **container-per-task isolation**, but the
surface is completely different: it's an MCP stdio server that an agent
*itself* uses to provision its own per-task containers and git worktrees. There
is no HTTP API for an *external* caller — it's a tool the agent picks up.

**Verdict:** complementary, not competing. You could imagine running container-use
*inside* a Stromboli container if you wanted nested per-subtask isolation, or
running Stromboli with container-use as an MCP server. They solve adjacent
problems.

### Pinocchio (the same author's Docker version)

Worth flagging because it's mentioned in the README: Pinocchio is essentially
"Stromboli but Docker." If you can't use Podman (Windows hosts, certain CI
environments, corporate Docker-only mandates), Pinocchio is the same project
shape on a different runtime.

---

## Adjacent categories (related, but not the same product)

### Hosted sandbox SaaS — E2B, Daytona, Modal, Northflank, Fly, Vercel, Blaxel

These are all **commercial cloud platforms** that sell "give me a sandbox via
API." They're vastly more polished than Stromboli on every operational axis:

- **[E2B]** — Firecracker microVMs, 150ms cold start, used by ~half the Fortune 500.
- **[Daytona]** — sub-90ms OCI sandboxes, snapshot/resume, Series A funded ($24M, Feb 2026).
- **[Modal]** — serverless GPU sandboxes, autoscaling, no session limits.
- **[Northflank]** — Firecracker on Kubernetes, BYOC, 2M+ microVMs/month in production.
- **[Fly Sprites]** / **[Vercel Sandbox]** / **[Blaxel]** — each with their own twist.

What they don't do: they don't give you **Claude-Code-specific** semantics. You
have to install Claude inside their sandbox, ferry credentials in, manage
sessions yourself, build the headless-flag mapping yourself. They're generic.

**Verdict:** if you want a battle-tested sandbox-as-a-service and you don't mind
running Claude *inside* it yourself, these win on reliability, performance, and
features. Stromboli wins when you (a) cannot ship code/credentials to a SaaS
vendor (regulated industries, on-prem, air-gapped), or (b) want the
Claude-specific ergonomics out of the box.

### Anthropic Managed Agents (cloud-hosted Claude agents)

[Anthropic Managed Agents] (GA April 2026) flips ownership entirely: you describe
the agent (model, prompt, tools, MCP servers, guardrails), Anthropic runs it,
including sandboxed code execution, checkpointing, credential management, and
scoped permissions.

**Verdict:** for many use cases this is now the right answer and Stromboli is
genuinely redundant. Stromboli only beats it if (a) you want Claude Code's
headless *CLI* semantics specifically (Managed Agents is a different surface),
(b) you want self-hosted/on-prem, or (c) you want to run Claude alongside your
own tooling in one container with hooks and a bind-mounted workdir.

### Desktop UIs — Sculptor, Conductor, Crystal, Vibe Kanban

[Sculptor], [Conductor], [Crystal], [Vibe Kanban] are all **desktop apps for
humans** that orchestrate parallel Claude Code agents in isolated containers or
worktrees. Sculptor uses Docker per agent, Conductor and Crystal use git
worktrees, Vibe Kanban is a kanban board around agent CLIs.

**Verdict:** completely different audience. If a human is sitting in front of a
screen managing multiple coding agents, you almost certainly want one of these,
not Stromboli. Stromboli wins only when **no human is in the loop** — webhooks,
CI, n8n flows, Slack bots, cron, batch jobs.

### Container-wrapper CLIs — ClaudeBox, sandclaude, codex-container-sandbox

These are **per-developer wrappers**: `claudebox 'fix the bug'` runs Claude in a
hardened Docker/Podman container locally. No server, no API, no multi-tenancy.

**Verdict:** if you're a single developer wanting safer local Claude runs, these
are simpler. Stromboli only makes sense if a *service* needs to spawn Claude.

### claude-code-action (official GitHub Action)

[claude-code-action] is Anthropic's official GA GitHub Action — drops Claude
Code into any GitHub workflow with one line. If "I want Claude in CI" is your
only goal, this is one line of YAML and it's done.

**Verdict:** if your use case is GitHub-only and CI-only, this beats Stromboli on
setup cost. Stromboli wins when you need to spawn agents from Slack, n8n,
arbitrary apps, your own backend, scheduled cron, or non-GitHub CI.

---

## Feature matrix

A more rigorous head-to-head with the projects in Stromboli's actual lane.
✅ = fully supported, ⚠️ = partial / DIY, ❌ = not supported, n/a = not applicable.

| Feature | Stromboli | Claude Agent SDK | claude-code-runner | OpenHands | container-use | Sculptor | E2B/Daytona |
|---|:-:|:-:|:-:|:-:|:-:|:-:|:-:|
| Self-hosted | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ (local) | ❌ (SaaS) |
| REST API surface | ✅ | ⚠️ (you build it) | ✅ | ✅ | ❌ (MCP) | ❌ (UI) | ✅ |
| OpenAPI spec | ✅ | ❌ | ❌ | ⚠️ | ❌ | ❌ | ✅ |
| Per-request container isolation | ✅ | ⚠️ (DIY) | ❌ | ✅ | ✅ | ✅ | ✅ |
| Rootless by default | ✅ (Podman) | ⚠️ | ❌ | ❌ | ⚠️ | ❌ | n/a |
| Sync / async / streaming modes | ✅✅✅ | ⚠️ (you build) | ⚠️ sync only | ✅ | n/a | n/a | ✅ |
| SSE streaming | ✅ | ⚠️ (you build) | ❌ | ✅ | n/a | ✅ | ⚠️ |
| Session continuation | ✅ | ✅ | ⚠️ | ✅ | ✅ (per branch) | ✅ | ⚠️ (DIY) |
| Webhook on completion | ✅ | ❌ (DIY) | ❌ | ⚠️ | ❌ | ❌ | ⚠️ |
| Job queue + cancel | ✅ | ❌ (DIY) | ❌ | ✅ | ❌ | ✅ | ✅ |
| JWT auth out of box | ✅ | ❌ (DIY) | ❌ | ⚠️ | n/a | n/a | ✅ |
| Prometheus metrics | ✅ | ❌ (DIY) | ❌ | ⚠️ | ❌ | ❌ | ✅ |
| OpenTelemetry tracing | ✅ | ❌ (DIY) | ❌ | ⚠️ | ❌ | ❌ | ✅ |
| Rate limiting | ✅ | ❌ (DIY) | ❌ | ⚠️ | ❌ | n/a | ✅ |
| Custom container images | ✅ | ✅ | ❌ | ✅ | ✅ | ⚠️ | ✅ |
| Multi-service compose envs | ✅ | ❌ | ❌ | ⚠️ | ❌ | ❌ | ⚠️ |
| Lifecycle hooks (on_create, on_start) | ✅ | ⚠️ | ❌ | ⚠️ | ✅ | ❌ | ⚠️ |
| Podman-native secrets | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ (theirs) |
| Model-agnostic | ❌ Claude only | ❌ Claude only | ❌ Claude only | ✅ any LLM | ✅ any agent | ⚠️ Claude+Codex | ✅ any |
| Mid-loop hooks / MCP control | ❌ | ✅ | ❌ | ✅ | ⚠️ | ⚠️ | n/a |
| Built-in browser/tooling | ❌ | ⚠️ | ❌ | ✅ | ❌ | ⚠️ | ⚠️ |
| GA / production maturity | ⚠️ alpha | ✅ Anthropic | ⚠️ hobby | ✅ Series A | ✅ Dagger-backed | ✅ beta | ✅ enterprise |
| Open source | ✅ MIT | ✅ MIT | ✅ MIT | ✅ MIT | ✅ Apache | ⚠️ partial | ⚠️ varies |
| Cost | Self-host only | Self-host only | Self-host only | Self-host only | Free | Free beta | Paid |

The matrix shows the real picture clearly: **Stromboli's only column-of-its-own
is the cluster of operational features (auth, metrics, tracing, rate limiting,
webhooks) bundled with per-request container isolation in a single self-hosted
binary.** Almost everything else is matched by something else.

---

## What Stromboli does that nothing else does (the honest list)

After working through the landscape, here is the real shortlist:

1. **REST + Podman + Claude Code, in one binary.** No other widely known project
   bundles "rootless Podman per request" + "full Claude headless flag surface
   over JSON" + "OpenAPI-described HTTP API" in a single `make run`.
2. **Production ops out of the box.** JWT auth, rate limiting, OTel tracing,
   Prometheus metrics, structured JSON logs, request IDs — without writing any
   middleware. The Agent SDK gives you none of this; you assemble it yourself.
3. **Three execution modes from the same endpoint family** (`/run`, `/run/async`,
   `/run/stream`). Sync for short tasks, async with webhooks for long ones, SSE
   for live UIs. Most peers do one, maybe two.
4. **Compose environments per agent.** Spawn Claude alongside Postgres + Redis
   for a task, tear it all down when done. Closest peers (container-use, E2B)
   either don't do this or charge for it.
5. **Lifecycle hooks at container layer**, not just at the agent layer. Run
   `pip install`, start a background service, etc. before Claude starts. The
   Agent SDK can do this in code; Stromboli does it in JSON.
6. **MCP-server wrapper exists** ([mcp-server-stromboli]) so other agents
   (Claude Desktop, Cursor) can spawn Stromboli agents as a tool. Few peers ship
   this.
7. **Native Go and TS SDKs that auto-regenerate from OpenAPI** on each release.
   Most peers leave you holding the swagger.

That's a real list. It's not a long list. It's a list.

---

## Where Stromboli loses (the honest list)

1. **One-developer project.** Stromboli is maintained by a solo author. Anthropic
   ships the Agent SDK. Daytona and E2B have funded teams. OpenHands has 68k
   stars. For mission-critical infra, that asymmetry matters.
2. **No mid-loop control.** You hand Claude a prompt and a flag set; you can't
   intercept tool calls, swap MCP servers mid-conversation, or run hooks between
   turns. The Claude Agent SDK can. This is the biggest functional gap.
3. **Claude-only.** Cannot run Codex, Gemini, Aider, OpenCode. OpenHands and
   most SaaS sandboxes are model-agnostic.
4. **Container-per-request is heavy.** Cold start is on the order of seconds
   (Podman pull cache + image load + Claude boot). Daytona claims sub-90ms,
   Blaxel sub-25ms, E2B 150ms. For high-frequency, short-prompt workloads, this
   model is wrong.
5. **Single-host by default.** No built-in scheduler across machines. To scale
   horizontally you put it behind a load balancer and shard yourself. Northflank
   and Daytona handle multi-tenant scheduling natively.
6. **Podman-only.** Beautiful when you have it; painful on Windows hosts and in
   environments that mandate Docker. (The companion project Pinocchio fills the
   Docker gap.)
7. **No GPU story.** If your agent needs GPU (vision, local model, etc.),
   Stromboli has no first-class path. Modal, Beam, Daytona do.
8. **Maturity.** Pre-1.0, alpha. Bugs exist. APIs may shift. Compare to
   Anthropic's GA Agent SDK or E2B's Fortune 500 deployments.
9. **Discoverability.** Approximately nobody has heard of Stromboli. Sculptor,
   Conductor, container-use, OpenHands all have meaningful mind-share. Picking
   Stromboli means betting on something with a tiny community.

---

## When to pick what

A practical decision tree:

- **A human is the primary user clicking buttons** → [Sculptor], [Conductor],
  [Crystal], or [Vibe Kanban].
- **You only need this in GitHub Actions** → official [claude-code-action].
- **You can let Anthropic host it** → [Anthropic Managed Agents].
- **You'll write Python or TypeScript and want max flexibility** →
  [Claude Agent SDK].
- **You need a generic untrusted-code sandbox SaaS** → [E2B], [Daytona],
  [Modal], [Northflank].
- **You want a full autonomous-developer product** → [OpenHands], Devin,
  Cursor background agents, Codex Cloud.
- **You want safer local Claude runs as one developer** → [ClaudeBox],
  sandclaude, claude-code-devcontainer.
- **You want Claude to provision its own per-task containers from inside an
  agent loop** → [container-use].
- **You want a self-hosted REST API that spawns Claude in rootless Podman, with
  auth/metrics/tracing already wired up, callable from anything that speaks
  HTTP** → **Stromboli** is a sensible pick.

That last bullet is real. It's narrow. It exists.

---

## Verdict: is Stromboli useless?

**No, but it's narrow and the niche is shrinking.**

Stromboli is **not useless** because:

- The "self-hosted, language-agnostic REST API around Claude Code with
  production ops baked in" use case is genuinely underserved. The Agent SDK
  expects you to write code; Stromboli expects you to write `curl`. These are
  different problems.
- For regulated, on-prem, air-gapped, or "we won't ship code or credentials to
  Anthropic/E2B/Daytona" environments, the SaaS options are non-starters and
  Stromboli is one of a handful of viable answers.
- The integration surface (n8n, Slack, internal apps, non-Python/TS backends,
  cron, webhooks, MCP) is genuinely easier to wire up against an HTTP+OpenAPI
  service than against a Python library.
- The bundled ops layer (JWT, rate limiting, OTel, Prometheus, structured logs)
  reflects real engineering effort that you would otherwise reinvent.

Stromboli **is at risk of becoming useless** because:

- **Anthropic Managed Agents (April 2026)** absorbs a large fraction of the use
  cases where users would have reached for Stromboli. If the cloud trust story
  is acceptable to you, Managed Agents is now the path of least resistance.
- **The Claude Agent SDK** keeps gaining features (hooks, MCP, sandboxing,
  context compaction) that Stromboli architecturally cannot match without
  becoming the SDK itself.
- **Daytona, E2B, Northflank** are racing toward "perfect generic agent
  sandboxes" with funding and dedicated teams. If they ship a "run Claude Code
  headless" preset with the same JSON ergonomics, Stromboli's last unique
  ergonomic advantage shrinks.
- **Hosted multi-agent UIs** (Sculptor, Conductor) cover the human-in-the-loop
  workflows that overlap with what many users *thought* they wanted from a REST
  API.

**Honest closing line:** Stromboli is a real answer to a real, narrow question.
It is not the future of agent infrastructure — that future belongs to managed
clouds and the Agent SDK. But for the well-defined slice of "I need Claude Code
running in rootless containers behind an HTTP endpoint on infrastructure I
control, *today*, *in production*, *without writing the runner myself*," it is
one of the few good answers, and the only one with this exact feature mix. The
honest read is: **useful, narrow, transitional, and worth using if and only if
the niche fits**.

---

## Sources

Direct competitors and adjacent products:

- [Claude Agent SDK] — Anthropic's official Python/TS SDK
- [Claude Agent SDK self-host] — Anthropic's self-hosting guide
- [Anthropic Managed Agents] — cloud-hosted agents
- [Sandboxing - Claude Code Docs][sandboxing] — Anthropic's native OS sandboxing
- [claude-code-action] — official GitHub Action
- [claude-code-runner] — ericvtheg's self-hosted runner
- [OpenHands] — open agent platform
- [container-use] — Dagger's MCP-driven container worktrees
- [Sculptor] — Imbue desktop UI
- [Conductor] — Melty Labs Mac orchestrator
- [Crystal] — Electron multi-session manager
- [Vibe Kanban] — agent kanban board
- [ClaudeBox] — macOS Seatbelt wrapper
- [E2B], [Daytona], [Modal], [Northflank], [Fly Sprites], [Vercel Sandbox],
  [Blaxel], [Runloop] — hosted sandbox SaaS

Surveys / landscape:

- [List of coding agent sandboxes (May 2026)](https://gist.github.com/wincent/2752d8d97727577050c043e4ff9e386e)
- [Top AI Code Sandbox Products in 2025 - Modal](https://modal.com/blog/top-code-agent-sandbox-products)
- [Daytona vs E2B in 2026 - Northflank](https://northflank.com/blog/daytona-vs-e2b-ai-code-execution-sandboxes)
- [Open-Source Alternatives to E2B - Beam](https://www.beam.cloud/blog/best-e2b-alternatives)
- [9 Open-Source Agent Orchestrators for AI Coding - Augment Code](https://www.augmentcode.com/tools/open-source-agent-orchestrators)
- [Container Use coverage - InfoQ](https://www.infoq.com/news/2025/08/container-use/)
- [Docker Sandboxes blog post](https://www.docker.com/blog/docker-sandboxes-run-claude-code-and-other-coding-agents-unsupervised-but-safely/)

[Claude Agent SDK]: https://platform.claude.com/docs/en/agent-sdk/hosting
[Claude Agent SDK self-host]: https://code.claude.com/docs/en/agent-sdk/hosting
[Anthropic Managed Agents]: https://platform.claude.com/docs/en/managed-agents/multi-agent
[sandboxing]: https://code.claude.com/docs/en/sandboxing
[claude-code-action]: https://github.com/anthropics/claude-code-action
[claude-code-runner]: https://github.com/ericvtheg/claude-code-runner
[OpenHands]: https://github.com/OpenHands/OpenHands
[container-use]: https://github.com/dagger/container-use
[Sculptor]: https://github.com/imbue-ai/sculptor
[Conductor]: https://conductor.build/
[Crystal]: https://github.com/stravu/crystal
[Vibe Kanban]: https://github.com/BloopAI/vibe-kanban
[ClaudeBox]: https://github.com/Greitas-Kodas/claudebox
[E2B]: https://e2b.dev/
[Daytona]: https://www.daytona.io/
[Modal]: https://modal.com/
[Northflank]: https://northflank.com/
[Fly Sprites]: https://fly.io/
[Vercel Sandbox]: https://vercel.com/
[Blaxel]: https://blaxel.ai/
[Runloop]: https://runloop.ai/
[mcp-server-stromboli]: https://github.com/tomblancdev/mcp-server-stromboli
