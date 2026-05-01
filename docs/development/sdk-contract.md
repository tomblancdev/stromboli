# SDK Contract & Release Fan-out

This page describes how downstream client SDKs (`stromboli-go`, `stromboli-ts`, `mcp-server-stromboli`, and the upcoming `n8n-nodes-stromboli`) stay in sync with the server's OpenAPI surface.

The contract is intentionally one-way: when a tag is pushed on this repo, a `repository_dispatch` fires to each registered SDK. The SDK receives the new spec, regenerates its typed client, and opens a PR titled `chore: sync to stromboli vX.Y.Z`. If the SDK's own CI passes, the PR can auto-merge. No human in the loop unless the regeneration introduces an actual breaking change.

## Why a dispatch and not a monorepo

Each SDK has its own ecosystem expectations — npm publishes for TS, pkg.go.dev for Go, n8n's community-nodes registry, MCP's directory. A monorepo merge would force every consumer to adopt this repo's release cadence and tooling. Independent repos with a contract bus stay autonomous.

## The protocol

When this repo's [release workflow](https://github.com/tomblancdev/stromboli/actions/workflows/release.yml) finishes successfully on a tag push, it sends one `repository_dispatch` per SDK:

```
POST /repos/{owner}/{sdk-repo}/dispatches
{
  "event_type": "stromboli-released",
  "client_payload": {
    "version":     "v0.5.3-alpha",
    "swagger_url": "https://raw.githubusercontent.com/.../v0.5.3-alpha/docs/swagger/swagger.json",
    "release_url": "https://github.com/.../releases/tag/v0.5.3-alpha"
  }
}
```

The fan-out matrix lives in `.github/workflows/release.yml` under the `notify-sdks` job. Adding a new SDK repo is a one-line matrix entry on the server side.

## Adding a new SDK to the fan-out

Two steps:

1. **Server side (this repo):** add the repo name to the `notify-sdks` job's matrix. PR-able by anyone with write access here.
2. **SDK side (the new repo):** drop in the receiver workflow below.

The dispatch will fire whether or not the SDK has a receiver — clients that haven't wired one up just ignore the event.

## Receiver template

Drop this into the SDK's `.github/workflows/sync-stromboli.yml`. The codegen step is the only part you customize per ecosystem.

!!! warning "Enable PR creation on the SDK repo first"
    GitHub Actions is **not allowed to create or approve pull requests by default** — even with `pull-requests: write` in the workflow permissions block. The `peter-evans/create-pull-request` step will fail with `GitHub Actions is not permitted to create or approve pull requests` until you flip a repo-level setting:

    1. Go to `Settings → Actions → General` on the SDK repo
    2. Under **Workflow permissions**, check **Allow GitHub Actions to create and approve pull requests**
    3. Save

    This needs to be done **once per SDK repo** that adds a receiver. URL pattern: `https://github.com/<owner>/<sdk>/settings/actions`.

```yaml
name: Sync OpenAPI from stromboli

on:
  repository_dispatch:
    types: [stromboli-released]
  workflow_dispatch:
    inputs:
      version:
        description: 'stromboli tag (e.g. v0.5.3-alpha)'
        required: true
      swagger_url:
        description: 'Override swagger URL (optional)'
        required: false

permissions:
  contents: write
  pull-requests: write

jobs:
  sync:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0

      - name: Resolve dispatch payload
        id: payload
        run: |
          if [ "${{ github.event_name }}" = "workflow_dispatch" ]; then
            echo "version=${{ inputs.version }}"           >> "$GITHUB_OUTPUT"
            echo "swagger_url=${{ inputs.swagger_url }}"   >> "$GITHUB_OUTPUT"
          else
            echo "version=${{ github.event.client_payload.version }}"           >> "$GITHUB_OUTPUT"
            echo "swagger_url=${{ github.event.client_payload.swagger_url }}"   >> "$GITHUB_OUTPUT"
          fi

      - name: Fetch new swagger
        run: |
          mkdir -p .stromboli-sync
          URL="${{ steps.payload.outputs.swagger_url }}"
          if [ -z "$URL" ]; then
            URL="https://raw.githubusercontent.com/tomblancdev/stromboli/${{ steps.payload.outputs.version }}/docs/swagger/swagger.json"
          fi
          curl -fsSL "$URL" -o .stromboli-sync/swagger.json
          jq -r '.info.version' .stromboli-sync/swagger.json

      # ─── PER-ECOSYSTEM CODEGEN ─── (replace with whatever your client uses)
      #
      #   Go SDK:    swag init / openapi-generator-cli generate -g go
      #   TS SDK:    npx openapi-typescript .stromboli-sync/swagger.json -o src/api.ts
      #   MCP:       custom — usually re-emits tool descriptors from the spec
      #   n8n:       regen the resource/operation descriptions
      #
      # The job opens a PR with whatever this step produces; if codegen
      # bails, the PR doesn't open and the failure shows up as a CI run.
      - name: Regenerate typed client
        run: |
          # TODO: ecosystem-specific codegen here
          echo "implement me"

      - name: Record compatibility marker
        run: |
          echo "${{ steps.payload.outputs.version }}" > STROMBOLI_COMPAT

      - name: Open PR with regenerated client
        uses: peter-evans/create-pull-request@v7
        with:
          commit-message: 'chore: sync to stromboli ${{ steps.payload.outputs.version }}'
          title: 'chore: sync to stromboli ${{ steps.payload.outputs.version }}'
          body: |
            Automated regeneration triggered by stromboli release.

            - **Server tag:** ${{ steps.payload.outputs.version }}
            - **Spec source:** ${{ steps.payload.outputs.swagger_url }}
            - **Release notes:** ${{ github.event.client_payload.release_url }}

            If the diff is mechanical (new field added to a request/response),
            this PR can auto-merge once CI is green. If the regeneration
            introduces a breaking change in the public surface, hold for
            human review.
          branch: sync-stromboli-${{ steps.payload.outputs.version }}
          delete-branch: true
          labels: stromboli-sync, automated
```

## Required repo secret

The fan-out job in this repo needs a token with `repo` scope on every target SDK. Two ways to provision:

| Option | Pros | Cons |
|---|---|---|
| **Personal Access Token** (classic, `repo` scope, stored as `SDK_DISPATCH_TOKEN`) | Simple, works today | Tied to a single account; expires |
| **GitHub App** with `actions: write` + `metadata: read` on each SDK repo, generate an installation token at job time | Auditable, tied to the org | More setup; needs a small token-mint step |

Start with a PAT (`Settings → Secrets and variables → Actions → New repository secret`, name `SDK_DISPATCH_TOKEN`). Move to a GitHub App if/when secret rotation becomes a chore.

If the secret is missing, the fan-out step **logs a warning and skips** — the release itself never fails because of the SDK side. That's deliberate: a release should never block on downstream tooling, only notify it.

## Compatibility marker

Each SDK is encouraged to write the server tag it was last synced against to a `STROMBOLI_COMPAT` file (or the README). That way an SDK consumer can answer "does `stromboli-go@v0.5.3` work with my `stromboli@v0.5.3` server?" by checking one file.

The receiver template above sets it automatically.

## Future layers (not yet implemented)

- **Layer 2: version coupling.** Have the SDK auto-tag its release at the same version (`stromboli-go@v0.5.3`) once the sync PR merges, so the version mapping is unambiguous.
- **Layer 3: contract tests in this repo's CI.** A matrix job that clones each SDK on every PR here, points its e2e tests at a localhost server built from the PR commit, and fails the PR if anything regresses. Catches breaking changes before they ever reach a tag.

Both are bigger lifts; land Layer 1 (this page) first and see how often Layers 2/3 would actually pay off.
