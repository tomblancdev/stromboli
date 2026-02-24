# Client Examples

Copy-pasteable examples for calling Stromboli from different languages and common automation scenarios.

## curl

```bash
# Simple prompt
curl -X POST localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{"prompt": "Hello, Claude!"}'

# With workspace, auth, and options
curl -X POST localhost:8080/run \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "prompt": "Analyze the code structure",
    "workdir": "/workspace",
    "podman": {"volumes": ["/home/user/myproject:/workspace"]},
    "claude": {"model": "sonnet", "max_budget_usd": 1.00}
  }'

# Async with webhook
curl -X POST localhost:8080/run/async \
  -d '{
    "prompt": "Refactor this codebase",
    "webhook_url": "https://myserver.com/webhook"
  }'

# Stream output
curl -N "localhost:8080/run/stream?prompt=Hello"
```

## Python

```python
import requests
import json

class StromboliClient:
    def __init__(self, base_url="http://localhost:8080", token=None):
        self.base_url = base_url.rstrip("/")
        self.session = requests.Session()
        if token:
            self.session.headers["Authorization"] = f"Bearer {token}"

    def run(self, prompt, workdir=None, volumes=None, model="sonnet", **kwargs):
        payload = {"prompt": prompt, "claude": {"model": model}}
        if workdir:
            payload["workdir"] = workdir
        if volumes:
            payload.setdefault("podman", {})["volumes"] = volumes
        for key in ("max_budget_usd", "allowed_tools"):
            if key in kwargs:
                payload["claude"][key] = kwargs[key]
        resp = self.session.post(f"{self.base_url}/run", json=payload)
        resp.raise_for_status()
        return resp.json()

    def run_async(self, prompt, webhook_url=None, **kwargs):
        payload = {"prompt": prompt}
        if webhook_url:
            payload["webhook_url"] = webhook_url
        resp = self.session.post(f"{self.base_url}/run/async", json=payload)
        resp.raise_for_status()
        return resp.json()

    def get_job(self, job_id):
        resp = self.session.get(f"{self.base_url}/jobs/{job_id}")
        resp.raise_for_status()
        return resp.json()

    def stream(self, prompt, workdir=None):
        params = {"prompt": prompt}
        if workdir:
            params["workdir"] = workdir
        resp = self.session.get(f"{self.base_url}/run/stream", params=params, stream=True)
        resp.raise_for_status()
        for line in resp.iter_lines(decode_unicode=True):
            if line and line.startswith("data: "):
                yield json.loads(line[6:])

# Usage
client = StromboliClient()
result = client.run(
    "Analyze this project",
    workdir="/workspace",
    volumes=["/home/user/myproject:/workspace"],
)
print(result["output"])
```

## JavaScript

```javascript
class StromboliClient {
  constructor(baseUrl = 'http://localhost:8080', token = null) {
    this.baseUrl = baseUrl.replace(/\/$/, '');
    this.token = token;
  }

  async #fetch(path, options = {}) {
    const headers = { 'Content-Type': 'application/json', ...options.headers };
    if (this.token) headers['Authorization'] = `Bearer ${this.token}`;
    const response = await fetch(`${this.baseUrl}${path}`, { ...options, headers });
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    return response;
  }

  async run(prompt, { workdir, model = 'sonnet', volumes, maxBudget } = {}) {
    const payload = { prompt, claude: { model } };
    if (workdir) payload.workdir = workdir;
    if (volumes) payload.podman = { volumes };
    if (maxBudget) payload.claude.max_budget_usd = maxBudget;
    const resp = await this.#fetch('/run', { method: 'POST', body: JSON.stringify(payload) });
    return resp.json();
  }

  async *stream(prompt, { workdir } = {}) {
    const params = new URLSearchParams({ prompt });
    if (workdir) params.set('workdir', workdir);
    const resp = await this.#fetch(`/run/stream?${params}`);
    const reader = resp.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() || '';
      for (const line of lines) {
        if (line.startsWith('data: ')) yield JSON.parse(line.slice(6));
      }
    }
  }
}

// Usage
const client = new StromboliClient();
const result = await client.run('Analyze this project', {
  workdir: '/workspace',
  volumes: ['/home/user/myproject:/workspace'],
});
console.log(result.output);
```

## Go

```go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Client struct {
	BaseURL string
	Token   string
}

type RunRequest struct {
	Prompt  string          `json:"prompt"`
	Workdir string          `json:"workdir,omitempty"`
	Claude  *ClaudeOptions  `json:"claude,omitempty"`
	Podman  *PodmanOptions  `json:"podman,omitempty"`
}

type ClaudeOptions struct {
	Model        string  `json:"model,omitempty"`
	MaxBudgetUSD float64 `json:"max_budget_usd,omitempty"`
}

type PodmanOptions struct {
	Volumes []string `json:"volumes,omitempty"`
}

type RunResponse struct {
	Output    string `json:"output"`
	SessionID string `json:"session_id"`
}

func (c *Client) Run(ctx context.Context, req *RunRequest) (*RunResponse, error) {
	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/run", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, b)
	}
	var result RunResponse
	json.NewDecoder(resp.Body).Decode(&result)
	return &result, nil
}

// Usage:
// client := &Client{BaseURL: "http://localhost:8080"}
// result, _ := client.Run(context.Background(), &RunRequest{
//     Prompt:  "Analyze this code",
//     Workdir: "/workspace",
//     Podman:  &PodmanOptions{Volumes: []string{"/home/user/project:/workspace"}},
// })
```

## CI/CD code review

Run Stromboli as a service container in GitHub Actions to review PRs:

```yaml
# .github/workflows/ai-review.yml
name: AI Code Review
on:
  pull_request:
    types: [opened, synchronize]

jobs:
  review:
    runs-on: ubuntu-latest
    services:
      stromboli:
        image: ghcr.io/tomblancdev/stromboli:latest
        ports:
          - 8080:8080
        volumes:
          - /home/runner/work:/workspace
        env:
          STROMBOLI_AUTH_ENABLED: "false"

    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Setup credentials
        run: echo '${{ secrets.CLAUDE_CREDENTIALS }}' > .claude-credentials.json

      - name: Wait for Stromboli
        run: |
          for i in {1..30}; do curl -s localhost:8080/health && break; sleep 1; done

      - name: Review
        run: |
          FILES=$(git diff --name-only origin/${{ github.base_ref }}...HEAD | tr '\n' ' ')
          REVIEW=$(curl -s -X POST localhost:8080/run \
            -H "Content-Type: application/json" \
            -d "{
              \"prompt\": \"Review these changed files for bugs and improvements: $FILES\",
              \"workdir\": \"/workspace/${{ github.repository }}/${{ github.ref_name }}\",
              \"claude\": {\"model\": \"sonnet\", \"allowed_tools\": [\"Read\", \"Grep\", \"Glob\"]}
            }" | jq -r '.output')
          echo "$REVIEW" > review.md

      - name: Post comment
        uses: actions/github-script@v7
        with:
          script: |
            const review = require('fs').readFileSync('review.md', 'utf8');
            github.rest.issues.createComment({
              owner: context.repo.owner, repo: context.repo.repo,
              issue_number: context.issue.number,
              body: `## AI Code Review\n\n${review}`
            });
```

## Batch code migration

Apply a migration across multiple files using Python:

```python
import json

client = StromboliClient(token="your-token")

# Find files that need migration
result = client.run(
    "Find all files using print() instead of logging. Output as JSON: {\"files\": [...]}",
    workdir="/workspace",
    volumes=["/home/user/myproject:/workspace"],
    allowed_tools=["Glob", "Grep", "Read"],
)
files = json.loads(result["output"])["files"]

# Migrate each file
for f in files:
    client.run(
        f"Replace print() with logging in {f}. Import logging if needed.",
        workdir="/workspace",
        volumes=["/home/user/myproject:/workspace"],
        allowed_tools=["Read", "Edit"],
    )
```

## Test generator

Generate tests for existing code:

```python
client = StromboliClient(token="your-token")
result = client.run(
    "Analyze src/utils.py and generate pytest tests covering all public functions. "
    "Include edge cases. Write the tests to tests/test_utils.py.",
    workdir="/workspace",
    volumes=["/home/user/myproject:/workspace"],
    allowed_tools=["Read", "Glob", "Grep", "Write"],
)
```
