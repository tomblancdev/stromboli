# Examples & Use Cases

Real-world examples and use cases for Stromboli.

## API Client Examples

### cURL

Basic request with cURL:

```bash
# Simple prompt
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{"prompt": "Hello, Claude!"}'

# With workspace and options
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "prompt": "Analyze the code structure",
    "workdir": "/workspace",
    "podman": {
      "volumes": ["/home/user/myproject:/workspace"]
    },
    "claude": {
      "model": "sonnet",
      "max_budget_usd": 1.00
    }
  }'

# Async execution
curl -X POST http://localhost:8080/run/async \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Refactor this codebase",
    "workdir": "/workspace",
    "podman": {
      "volumes": ["/home/user/myproject:/workspace"]
    },
    "webhook_url": "https://myserver.com/webhook"
  }'

# Check job status
curl http://localhost:8080/jobs/job-abc123

# Stream output
curl -N "http://localhost:8080/run/stream?prompt=Hello"
```

### Python

```python
import requests
from typing import Optional
import json

class StromboliClient:
    """Python client for Stromboli API."""

    def __init__(self, base_url: str = "http://localhost:8080", token: Optional[str] = None):
        self.base_url = base_url.rstrip("/")
        self.session = requests.Session()
        if token:
            self.session.headers["Authorization"] = f"Bearer {token}"

    def run(
        self,
        prompt: str,
        workdir: Optional[str] = None,
        volumes: Optional[list[str]] = None,
        model: str = "sonnet",
        max_budget_usd: Optional[float] = None,
        allowed_tools: Optional[list[str]] = None,
        timeout: Optional[str] = None,
    ) -> dict:
        """Run a synchronous Claude agent."""
        payload = {"prompt": prompt}

        if workdir:
            payload["workdir"] = workdir

        claude_opts = {"model": model}
        if max_budget_usd:
            claude_opts["max_budget_usd"] = max_budget_usd
        if allowed_tools:
            claude_opts["allowed_tools"] = allowed_tools
        payload["claude"] = claude_opts

        podman_opts = {}
        if volumes:
            podman_opts["volumes"] = volumes
        if timeout:
            podman_opts["timeout"] = timeout
        if podman_opts:
            payload["podman"] = podman_opts

        response = self.session.post(f"{self.base_url}/run", json=payload)
        response.raise_for_status()
        return response.json()

    def run_async(
        self,
        prompt: str,
        workdir: Optional[str] = None,
        webhook_url: Optional[str] = None,
        **kwargs
    ) -> dict:
        """Start an async Claude agent job."""
        payload = {"prompt": prompt}
        if workdir:
            payload["workdir"] = workdir
        if webhook_url:
            payload["webhook_url"] = webhook_url

        response = self.session.post(f"{self.base_url}/run/async", json=payload)
        response.raise_for_status()
        return response.json()

    def get_job(self, job_id: str) -> dict:
        """Get job status and result."""
        response = self.session.get(f"{self.base_url}/jobs/{job_id}")
        response.raise_for_status()
        return response.json()

    def stream(self, prompt: str, workdir: Optional[str] = None):
        """Stream agent output via Server-Sent Events."""
        params = {"prompt": prompt}
        if workdir:
            params["workdir"] = workdir

        response = self.session.get(
            f"{self.base_url}/run/stream",
            params=params,
            stream=True
        )
        response.raise_for_status()

        for line in response.iter_lines():
            if line:
                line = line.decode("utf-8")
                if line.startswith("data: "):
                    yield json.loads(line[6:])

    def health(self) -> dict:
        """Check API health."""
        response = self.session.get(f"{self.base_url}/health")
        response.raise_for_status()
        return response.json()


# Usage examples
if __name__ == "__main__":
    client = StromboliClient()

    # Simple request
    result = client.run("What is 2 + 2?")
    print(result["output"])

    # With workspace and volumes
    result = client.run(
        prompt="Analyze this Python project and suggest improvements",
        workdir="/workspace",
        volumes=["/home/user/myproject:/workspace"],
        model="sonnet",
        max_budget_usd=0.50
    )
    print(result["output"])

    # Streaming
    for event in client.stream("Tell me a story"):
        if event["type"] == "output":
            print(event["content"], end="", flush=True)
        elif event["type"] == "done":
            print(f"\nSession: {event['session_id']}")
```

### JavaScript / Node.js

```javascript
// stromboli-client.js
class StromboliClient {
  constructor(baseUrl = 'http://localhost:8080', token = null) {
    this.baseUrl = baseUrl.replace(/\/$/, '');
    this.token = token;
  }

  async #fetch(path, options = {}) {
    const headers = {
      'Content-Type': 'application/json',
      ...options.headers,
    };

    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`;
    }

    const response = await fetch(`${this.baseUrl}${path}`, {
      ...options,
      headers,
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      throw new Error(error.error || `HTTP ${response.status}`);
    }

    return response;
  }

  async run(prompt, options = {}) {
    const { workdir, model = 'sonnet', maxBudget, allowedTools, timeout, volumes } = options;

    const payload = {
      prompt,
      claude: { model },
    };

    if (workdir) payload.workdir = workdir;
    if (volumes) {
      payload.podman = payload.podman || {};
      payload.podman.volumes = volumes;
    }
    if (maxBudget) payload.claude.max_budget_usd = maxBudget;
    if (allowedTools) payload.claude.allowed_tools = allowedTools;
    if (timeout) payload.podman = { timeout };

    const response = await this.#fetch('/run', {
      method: 'POST',
      body: JSON.stringify(payload),
    });

    return response.json();
  }

  async runAsync(prompt, options = {}) {
    const { workdir, webhookUrl, volumes } = options;

    const payload = { prompt };
    if (workdir) payload.workdir = workdir;
    if (webhookUrl) payload.webhook_url = webhookUrl;
    if (volumes) {
      payload.podman = { volumes };
    }

    const response = await this.#fetch('/run/async', {
      method: 'POST',
      body: JSON.stringify(payload),
    });

    return response.json();
  }

  async getJob(jobId) {
    const response = await this.#fetch(`/jobs/${jobId}`);
    return response.json();
  }

  async *stream(prompt, options = {}) {
    const { workdir } = options;
    const params = new URLSearchParams({ prompt });
    if (workdir) params.set('workdir', workdir);

    const response = await this.#fetch(`/run/stream?${params}`);
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() || '';

      for (const line of lines) {
        if (line.startsWith('data: ')) {
          yield JSON.parse(line.slice(6));
        }
      }
    }
  }

  async health() {
    const response = await this.#fetch('/health');
    return response.json();
  }
}

// Usage
const client = new StromboliClient();

// Simple request
const result = await client.run('What is 2 + 2?');
console.log(result.output);

// With workspace and volumes
const analysis = await client.run('Analyze this project', {
  workdir: '/workspace',
  volumes: ['/home/user/myproject:/workspace'],
  model: 'sonnet',
  maxBudget: 0.50,
});
console.log(analysis.output);

// Streaming
for await (const event of client.stream('Tell me a story')) {
  if (event.type === 'output') {
    process.stdout.write(event.content);
  }
}

export { StromboliClient };
```

### Go

```go
package stromboli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Client is a Stromboli API client
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// RunRequest represents a run request
type RunRequest struct {
	Prompt  string         `json:"prompt"`
	Workdir string         `json:"workdir,omitempty"`
	Claude  *ClaudeOptions `json:"claude,omitempty"`
	Podman  *PodmanOptions `json:"podman,omitempty"`
}

// ClaudeOptions configures Claude behavior
type ClaudeOptions struct {
	Model        string   `json:"model,omitempty"`
	MaxBudgetUSD float64  `json:"max_budget_usd,omitempty"`
	AllowedTools []string `json:"allowed_tools,omitempty"`
}

// PodmanOptions configures container settings
type PodmanOptions struct {
	Volumes []string `json:"volumes,omitempty"`
	Timeout string   `json:"timeout,omitempty"`
	Memory  string   `json:"memory,omitempty"`
}

// RunResponse is the response from a run request
type RunResponse struct {
	Output    string `json:"output"`
	SessionID string `json:"session_id"`
	Model     string `json:"model"`
}

// NewClient creates a new Stromboli client
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL:    baseURL,
		HTTPClient: http.DefaultClient,
	}
}

// WithToken sets the authentication token
func (c *Client) WithToken(token string) *Client {
	c.Token = token
	return c
}

// Run executes a synchronous Claude agent
func (c *Client) Run(ctx context.Context, req *RunRequest) (*RunResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/run", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}

	var result RunResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// Health checks the API health
func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.BaseURL+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy: status %d", resp.StatusCode)
	}

	return nil
}

// Example usage:
//
//   client := stromboli.NewClient("http://localhost:8080")
//
//   result, err := client.Run(context.Background(), &stromboli.RunRequest{
//       Prompt:    "Analyze this code",
//       Workdir: "/workspace",
//       Podman: &stromboli.PodmanOptions{
//           Volumes: []string{"/home/user/project:/workspace"},
//       },
//       Claude: &stromboli.ClaudeOptions{
//           Model:        "sonnet",
//           MaxBudgetUSD: 0.50,
//       },
//   })
```

---

## Real-World Use Cases

### 1. CI/CD Code Review Bot

Automatically review pull requests in your CI pipeline.

#### Option A: Service Container (Recommended)

Run Stromboli as a **service container** in your workflow. This gives the agent full access to your codebase:

```yaml
# .github/workflows/ai-review.yml
name: AI Code Review

on:
  pull_request:
    types: [opened, synchronize]

jobs:
  review:
    runs-on: ubuntu-latest

    # Run Stromboli as a service container
    services:
      stromboli:
        image: ghcr.io/tomblancdev/stromboli:latest
        ports:
          - 8080:8080
        volumes:
          # Mount the runner's work directory into Stromboli
          - /home/runner/work:/workspace
        env:
          # No auth needed - internal to workflow
          STROMBOLI_AUTH_ENABLED: "false"
          # Mount Claude credentials from secrets
          STROMBOLI_AGENT_CREDENTIALS_FILE: "/workspace/.claude-credentials.json"

    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Setup Claude credentials
        run: |
          echo '${{ secrets.CLAUDE_CREDENTIALS }}' > .claude-credentials.json

      - name: Wait for Stromboli
        run: |
          for i in {1..30}; do
            curl -s http://localhost:8080/health && break
            sleep 1
          done

      - name: Get changed files
        id: changed
        run: |
          echo "files=$(git diff --name-only origin/${{ github.base_ref }}...HEAD | tr '\n' ' ')" >> $GITHUB_OUTPUT

      - name: AI Review
        run: |
          # The workspace path inside Stromboli's container
          WORKSPACE="/workspace/${{ github.repository }}/${{ github.ref_name }}"

          RESPONSE=$(curl -s -X POST http://localhost:8080/run \
            -H "Content-Type: application/json" \
            -d '{
              "prompt": "Review the code changes in this PR for bugs, security issues, and improvements. Focus on the changed files: ${{ steps.changed.outputs.files }}",
              "workdir": "'"$WORKSPACE"'",
              "claude": {
                "model": "sonnet",
                "max_budget_usd": 1.00,
                "allowed_tools": ["Read", "Grep", "Glob"]
              }
            }')

          echo "$RESPONSE" | jq -r '.output' > review.md

      - name: Post Review Comment
        uses: actions/github-script@v7
        with:
          script: |
            const fs = require('fs');
            const review = fs.readFileSync('review.md', 'utf8');
            github.rest.issues.createComment({
              owner: context.repo.owner,
              repo: context.repo.repo,
              issue_number: context.issue.number,
              body: `## 🤖 AI Code Review\n\n${review}`
            });

      - name: Cleanup credentials
        if: always()
        run: rm -f .claude-credentials.json
```

!!! tip "How it works"
    Service containers share the runner's filesystem via volume mounts.
    After `actions/checkout`, the code is visible to Stromboli at `/workspace/<repo>/<branch>`.

#### Option B: Send Diff in Prompt (Remote Stromboli)

If you have a remote Stromboli server, send the code diff directly in the prompt:

```yaml
# .github/workflows/ai-review.yml
name: AI Code Review

on:
  pull_request:
    types: [opened, synchronize]

jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Get diff
        id: diff
        run: |
          # Get the diff (truncate if too large for prompt)
          DIFF=$(git diff origin/${{ github.base_ref }}...HEAD | head -c 50000)
          # Escape for JSON
          DIFF_ESCAPED=$(echo "$DIFF" | jq -Rs .)
          echo "diff=$DIFF_ESCAPED" >> $GITHUB_OUTPUT

      - name: AI Review
        run: |
          RESPONSE=$(curl -s -X POST ${{ secrets.STROMBOLI_URL }}/run \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${{ secrets.STROMBOLI_TOKEN }}" \
            -d "{
              \"prompt\": \"Review this code diff for bugs, security issues, and improvements:\\n\\n${{ steps.diff.outputs.diff }}\",
              \"claude\": {
                \"model\": \"sonnet\",
                \"max_budget_usd\": 0.50
              }
            }")

          echo "$RESPONSE" | jq -r '.output' > review.md

      - name: Post Review Comment
        uses: actions/github-script@v7
        with:
          script: |
            const fs = require('fs');
            const review = fs.readFileSync('review.md', 'utf8');
            github.rest.issues.createComment({
              owner: context.repo.owner,
              repo: context.repo.repo,
              issue_number: context.issue.number,
              body: `## 🤖 AI Code Review\n\n${review}`
            });
```

!!! warning "Limitations"
    - Diff is truncated to ~50KB to fit in prompt
    - Agent cannot explore the full codebase
    - Best for small, focused PRs

#### Option C: Self-Hosted Runner

If Stromboli runs on the same machine as your self-hosted runner:

```yaml
jobs:
  review:
    runs-on: self-hosted  # Must be on same machine as Stromboli
    steps:
      - uses: actions/checkout@v4

      - name: AI Review
        run: |
          # Workspace path is directly accessible to local Stromboli
          curl -s -X POST http://localhost:8080/run \
            -H "Content-Type: application/json" \
            -d '{
              "prompt": "Review this codebase for issues",
              "workdir": "${{ github.workspace }}",
              "claude": {
                "model": "sonnet",
                "allowed_tools": ["Read", "Grep", "Glob"]
              }
            }'
```

### 2. Documentation Generator

Generate documentation for your codebase.

```python
import os
from stromboli_client import StromboliClient

def generate_docs(project_path: str, output_dir: str):
    """Generate documentation for a project."""
    client = StromboliClient(
        base_url=os.environ["STROMBOLI_URL"],
        token=os.environ["STROMBOLI_TOKEN"]
    )

    # Generate README
    result = client.run(
        prompt="""Analyze this project and generate a comprehensive README.md including:
        - Project overview
        - Installation instructions
        - Usage examples
        - API documentation
        - Contributing guidelines

        Output only the markdown content.""",
        workdir=project_path,
        model="sonnet",
        max_budget_usd=1.00,
        allowed_tools=["Read", "Glob", "Grep"]
    )

    with open(os.path.join(output_dir, "README.md"), "w") as f:
        f.write(result["output"])

    # Generate API docs for each module
    result = client.run(
        prompt="""List all Python modules in this project that have public APIs.
        Output as JSON: {"modules": ["path/to/module.py", ...]}""",
        workdir=project_path,
        allowed_tools=["Glob", "Read"]
    )

    import json
    modules = json.loads(result["output"])["modules"]

    for module in modules:
        doc_result = client.run(
            prompt=f"""Generate API documentation for {module}.
            Include all public classes, functions, and their parameters.
            Output as markdown.""",
            workdir=project_path,
            allowed_tools=["Read"]
        )

        module_name = os.path.basename(module).replace(".py", "")
        with open(os.path.join(output_dir, f"{module_name}.md"), "w") as f:
            f.write(doc_result["output"])

if __name__ == "__main__":
    generate_docs("/path/to/project", "/path/to/docs")
```

### 3. Batch Code Migration

Migrate code patterns across multiple files.

```python
import os
import json
from stromboli_client import StromboliClient

def batch_migrate(project_path: str, migration_prompt: str):
    """Apply a migration to all matching files."""
    client = StromboliClient(
        base_url=os.environ["STROMBOLI_URL"],
        token=os.environ["STROMBOLI_TOKEN"]
    )

    # First, identify files that need migration
    result = client.run(
        prompt=f"""Find all files that match this criteria and need migration:
        {migration_prompt}

        Output as JSON: {{"files": ["path1", "path2", ...]}}""",
        workdir=project_path,
        allowed_tools=["Glob", "Grep", "Read"]
    )

    files = json.loads(result["output"])["files"]
    print(f"Found {len(files)} files to migrate")

    # Process each file
    for file_path in files:
        print(f"Migrating: {file_path}")

        result = client.run(
            prompt=f"""Apply this migration to {file_path}:
            {migration_prompt}

            Make the changes directly to the file.""",
            workdir=project_path,
            model="sonnet",
            allowed_tools=["Read", "Edit"]
        )

        print(f"  Done: {result['output'][:100]}...")

# Example: Migrate from print statements to logging
batch_migrate(
    "/home/user/myproject",
    """Replace all print() statements with proper logging:
    - Import logging at the top if not present
    - Use logger.info() for informational prints
    - Use logger.debug() for debug prints
    - Use logger.error() for error prints
    """
)
```

### 4. Webhook Handler for Async Jobs

Handle webhook notifications from async jobs.

```python
# webhook_server.py
from flask import Flask, request, jsonify
import json

app = Flask(__name__)

@app.route("/webhook", methods=["POST"])
def handle_webhook():
    """Handle Stromboli job completion webhooks."""
    data = request.json

    job_id = data["job_id"]
    status = data["status"]
    output = data.get("output", "")
    session_id = data.get("session_id", "")

    print(f"Job {job_id} completed with status: {status}")

    if status == "completed":
        # Process successful result
        process_result(job_id, output, session_id)
    elif status == "failed":
        # Handle failure
        handle_failure(job_id, data.get("error", "Unknown error"))

    return jsonify({"received": True})

def process_result(job_id: str, output: str, session_id: str):
    """Process successful job result."""
    # Save to database, send notification, etc.
    print(f"Processing result for job {job_id}")
    print(f"Output: {output[:200]}...")

    # Example: Save to file
    with open(f"results/{job_id}.txt", "w") as f:
        f.write(output)

def handle_failure(job_id: str, error: str):
    """Handle failed job."""
    print(f"Job {job_id} failed: {error}")
    # Send alert, retry, etc.

if __name__ == "__main__":
    app.run(host="0.0.0.0", port=5000)
```

### 5. Interactive Chat Application

Build a chat interface with session continuity.

```python
# chat_app.py
import os
from stromboli_client import StromboliClient

def chat():
    """Interactive chat with Claude via Stromboli."""
    client = StromboliClient(
        base_url=os.environ.get("STROMBOLI_URL", "http://localhost:8080")
    )

    session_id = None
    workspace = os.getcwd()

    print("Stromboli Chat (type 'quit' to exit, 'new' for new session)")
    print(f"Workdir: {workspace}")
    print("-" * 50)

    while True:
        try:
            user_input = input("\nYou: ").strip()
        except (KeyboardInterrupt, EOFError):
            break

        if user_input.lower() == "quit":
            break
        elif user_input.lower() == "new":
            session_id = None
            print("Started new session")
            continue
        elif not user_input:
            continue

        # Build request
        payload = {
            "prompt": user_input,
            "workdir": workspace,
            "claude": {"model": "sonnet"}
        }

        if session_id:
            payload["session_id"] = session_id

        print("\nClaude: ", end="", flush=True)

        # Use streaming for real-time output
        for event in client.stream(user_input, {"workdir": workspace}):
            if event["type"] == "output":
                print(event["content"], end="", flush=True)
            elif event["type"] == "done":
                session_id = event.get("session_id")

        print()  # New line after response

if __name__ == "__main__":
    chat()
```

### 6. Test Generator

Generate tests for existing code.

```python
import os
import json
from stromboli_client import StromboliClient

def generate_tests(project_path: str, source_file: str):
    """Generate tests for a source file."""
    client = StromboliClient(
        base_url=os.environ["STROMBOLI_URL"],
        token=os.environ["STROMBOLI_TOKEN"]
    )

    # Analyze the source file
    result = client.run(
        prompt=f"""Analyze {source_file} and generate comprehensive tests.

        Requirements:
        - Use pytest framework
        - Cover all public functions and classes
        - Include edge cases and error conditions
        - Use appropriate mocking where needed
        - Follow the existing test patterns in this project

        Create the test file in the tests/ directory with proper naming.""",
        workdir=project_path,
        model="sonnet",
        max_budget_usd=1.00,
        allowed_tools=["Read", "Glob", "Grep", "Write"]
    )

    print(result["output"])

# Example
generate_tests("/home/user/myproject", "src/utils.py")
```

---

## Best Practices

### Error Handling

Always handle errors gracefully:

```python
from stromboli_client import StromboliClient
import requests

client = StromboliClient()

try:
    result = client.run("Analyze this code", workdir="/path/to/project")
    print(result["output"])
except requests.exceptions.ConnectionError:
    print("Error: Cannot connect to Stromboli server")
except requests.exceptions.HTTPError as e:
    if e.response.status_code == 401:
        print("Error: Authentication failed")
    elif e.response.status_code == 429:
        print("Error: Rate limited, try again later")
    elif e.response.status_code == 400:
        error = e.response.json()
        print(f"Error: {error.get('error', 'Bad request')}")
    else:
        print(f"Error: HTTP {e.response.status_code}")
except Exception as e:
    print(f"Unexpected error: {e}")
```

### Rate Limiting

Implement backoff for rate limits:

```python
import time
import requests

def run_with_retry(client, prompt, max_retries=3, **kwargs):
    """Run with exponential backoff on rate limits."""
    for attempt in range(max_retries):
        try:
            return client.run(prompt, **kwargs)
        except requests.exceptions.HTTPError as e:
            if e.response.status_code == 429 and attempt < max_retries - 1:
                wait_time = 2 ** attempt  # 1, 2, 4 seconds
                print(f"Rate limited, waiting {wait_time}s...")
                time.sleep(wait_time)
            else:
                raise
```

### Session Management

Reuse sessions for conversation continuity:

```python
class ConversationManager:
    def __init__(self, client):
        self.client = client
        self.sessions = {}  # user_id -> session_id

    def chat(self, user_id: str, message: str, workdir: str = None):
        """Send message with session continuity."""
        session_id = self.sessions.get(user_id)

        payload = {"prompt": message}
        if session_id:
            payload["session_id"] = session_id
        if workdir:
            payload["workdir"] = workdir

        result = self.client.run(**payload)

        # Store session for future messages
        self.sessions[user_id] = result["session_id"]

        return result["output"]

    def clear_session(self, user_id: str):
        """Start fresh conversation."""
        if user_id in self.sessions:
            del self.sessions[user_id]
```

---

## Next Steps

- [Running Agents](running-agents.md) - Core agent documentation
- [Sessions](sessions.md) - Session management details
- [Secrets](secrets.md) - Secure credential handling
- [API Reference](../api/overview.md) - Complete API documentation
