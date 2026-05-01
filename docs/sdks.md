# SDKs & Integrations

Official client libraries and integrations for the Stromboli API. Each is maintained in its own repo so it can ship at the cadence of its ecosystem (Go modules, npm, n8n's community-nodes registry, MCP's directory).

All four are kept in sync with the server's OpenAPI spec via the [release fan-out](development/sdk-contract.md) — when this server tags a release, each SDK's CI regenerates its typed client and opens a `chore: sync to stromboli vX.Y.Z` PR.

## Pick by language / runtime

<div class="grid cards" markdown>

- :material-language-go: **stromboli-go**

    Idiomatic Go client for the Stromboli REST API. Typed request/response structs, context-aware methods, streaming helpers for SSE.

    [:octicons-mark-github-16: tomblancdev/stromboli-go](https://github.com/tomblancdev/stromboli-go)

    ```go
    import "github.com/tomblancdev/stromboli-go"
    ```

- :material-language-typescript: **stromboli-ts**

    TypeScript / JavaScript SDK. Works in Node, Bun, Deno, and modern browsers. Generated types track the OpenAPI spec exactly.

    [:octicons-mark-github-16: tomblancdev/stromboli-ts](https://github.com/tomblancdev/stromboli-ts)

    ```bash
    npm install @tomblancdev/stromboli
    ```

- :material-connection: **mcp-server-stromboli**

    Model Context Protocol server that exposes Stromboli's endpoints as MCP tools. Drop it into Claude Desktop, Cursor, or any MCP-compatible client to give your editor agent direct access to spawn and manage Stromboli agents.

    [:octicons-mark-github-16: tomblancdev/mcp-server-stromboli](https://github.com/tomblancdev/mcp-server-stromboli)

- :material-graph: **n8n-nodes-stromboli** _(coming soon)_

    Community node for [n8n](https://n8n.io). Drop Stromboli into your no-code workflows: trigger agent runs from webhooks, chain them with database / messaging / API nodes, route output to Slack / Discord / email.

    Not yet published to GitHub or the n8n community-nodes registry. Will be added to the [release fan-out](development/sdk-contract.md) once it ships.

</div>

## Picking the right one

| You want to… | Use |
|---|---|
| Spawn Stromboli agents from a Go service | [stromboli-go](https://github.com/tomblancdev/stromboli-go) |
| Build a Node / Bun / browser frontend that talks to Stromboli | [stromboli-ts](https://github.com/tomblancdev/stromboli-ts) |
| Let your editor agent (Claude Desktop, Cursor) spawn Stromboli agents directly | [mcp-server-stromboli](https://github.com/tomblancdev/mcp-server-stromboli) |
| Wire Stromboli into an n8n workflow alongside other automations | n8n-nodes-stromboli _(coming soon)_ |
| Stay protocol-agnostic / hit the API from a language without an SDK | The [REST API](api/endpoints.md) is just HTTP + JSON; any HTTP client works |

## Version compatibility

Each SDK records the server tag it was last regenerated against in a `STROMBOLI_COMPAT` file (or in its README). To check that a given SDK version pairs cleanly with your server:

1. Check the SDK's `STROMBOLI_COMPAT` (or the latest release notes)
2. Ensure your server is at the same major.minor version

Mechanical changes (added optional fields) are forward-compatible — an older SDK still works against a newer server. Breaking changes (renamed endpoints, removed fields) are confined to **major** version bumps.

## Building your own SDK

If you want to ship a client for a language that isn't covered above, the [SDK contract page](development/sdk-contract.md) walks through:

- The [OpenAPI spec](api/openapi.md) (the single source of truth — generate against it, don't hand-write)
- The [release fan-out protocol](development/sdk-contract.md#the-protocol) (so your SDK gets auto-regenerated when this server changes)
- A drop-in [receiver workflow template](development/sdk-contract.md#receiver-template) (~30 lines, ecosystem-specific codegen is the only customisation)

Once your SDK is wired up, [open a PR](https://github.com/tomblancdev/stromboli/edit/main/.github/workflows/release.yml) adding it to the `notify-sdks` matrix and we'll start fanning releases your way too.
