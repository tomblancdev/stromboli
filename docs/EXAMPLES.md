# Stromboli Usage Examples

Practical examples demonstrating how to use the Stromboli API for various use cases.

## Table of Contents

- [Basic Usage](#basic-usage)
- [Session Management](#session-management)
- [Async Execution with Polling](#async-execution-with-polling)
- [Streaming Output](#streaming-output)
- [Webhook Integration](#webhook-integration)
- [Resource Limits](#resource-limits)
- [Advanced Claude Options](#advanced-claude-options)
- [Error Handling](#error-handling)
- [Integration Examples](#integration-examples)

---

## Basic Usage

### Simple Prompt Execution

Execute a simple prompt without workspace:

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "What is the capital of France?",
    "claude": {
      "dangerously_skip_permissions": true
    }
  }'
```

**Response:**
```json
{
  "id": "run-abc123",
  "status": "completed",
  "output": "The capital of France is Paris.",
  "session_id": "sess-def456"
}
```

### Code Analysis

Analyze code in a workspace:

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Analyze the code in this project and identify potential bugs",
    "workspace": "/home/user/myproject",
    "claude": {
      "model": "sonnet",
      "dangerously_skip_permissions": true
    }
  }'
```

### Code Generation

Generate new code in a workspace:

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Create a REST API server in Go with health check endpoint",
    "workspace": "/home/user/newproject",
    "claude": {
      "model": "sonnet",
      "dangerously_skip_permissions": true,
      "system_prompt": "You are a senior Go developer. Write clean, idiomatic Go code with proper error handling."
    }
  }'
```

---

## Session Management

### Starting a Conversation

First request creates a new session:

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Create a new Go project structure for a REST API",
    "workspace": "/home/user/api-project",
    "claude": {
      "dangerously_skip_permissions": true
    }
  }'
```

**Response includes session_id:**
```json
{
  "id": "run-abc123",
  "status": "completed",
  "output": "I've created the project structure...",
  "session_id": "sess-550e8400-e29b-41d4-a716-446655440000"
}
```

### Continuing a Conversation

Use the session_id to continue the conversation:

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Now add unit tests for the handlers",
    "workspace": "/home/user/api-project",
    "claude": {
      "session_id": "sess-550e8400-e29b-41d4-a716-446655440000",
      "resume": true,
      "dangerously_skip_permissions": true
    }
  }'
```

### Using Continue Mode

Continue the most recent conversation in the workspace:

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Add error handling to the authentication middleware",
    "workspace": "/home/user/api-project",
    "claude": {
      "continue": true,
      "dangerously_skip_permissions": true
    }
  }'
```

### Listing Sessions

Get all active sessions:

```bash
curl http://localhost:8080/sessions
```

**Response:**
```json
{
  "sessions": [
    "sess-550e8400-e29b-41d4-a716-446655440000",
    "sess-7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "sess-8d45fd7a-6f4c-42aa-8a7f-d7c85a92e5f1"
  ]
}
```

### Cleaning Up Sessions

Delete a session when done:

```bash
curl -X DELETE http://localhost:8080/sessions/sess-550e8400-e29b-41d4-a716-446655440000
```

**Response:**
```json
{
  "success": true,
  "session_id": "sess-550e8400-e29b-41d4-a716-446655440000"
}
```

---

## Async Execution with Polling

### Start Long-Running Task

Submit a task that will take time:

```bash
curl -X POST http://localhost:8080/run/async \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Refactor the entire codebase to use dependency injection",
    "workspace": "/home/user/large-project",
    "claude": {
      "model": "opus",
      "dangerously_skip_permissions": true,
      "max_budget_usd": 10.0
    },
    "podman": {
      "timeout": "30m",
      "memory": "2g",
      "cpus": "2"
    }
  }'
```

**Response:**
```json
{
  "job_id": "job-abc123def456"
}
```

### Poll for Status

Check job status periodically:

```bash
#!/bin/bash
JOB_ID="job-abc123def456"

while true; do
  RESPONSE=$(curl -s http://localhost:8080/jobs/$JOB_ID)
  STATUS=$(echo $RESPONSE | jq -r '.status')

  echo "Job status: $STATUS"

  if [[ "$STATUS" == "completed" ]]; then
    echo "Output:"
    echo $RESPONSE | jq -r '.output'
    break
  elif [[ "$STATUS" == "failed" ]]; then
    echo "Error:"
    echo $RESPONSE | jq -r '.error'
    break
  elif [[ "$STATUS" == "cancelled" ]]; then
    echo "Job was cancelled"
    break
  fi

  sleep 5
done
```

### List All Jobs

View all async jobs:

```bash
curl http://localhost:8080/jobs
```

**Response:**
```json
{
  "jobs": [
    {
      "id": "job-abc123def456",
      "status": "running",
      "created_at": "2025-01-22T10:00:00Z",
      "updated_at": "2025-01-22T10:05:00Z"
    },
    {
      "id": "job-xyz789",
      "status": "completed",
      "output": "Refactoring complete...",
      "session_id": "sess-abc123",
      "created_at": "2025-01-22T09:00:00Z",
      "updated_at": "2025-01-22T09:30:00Z"
    }
  ]
}
```

### Cancel a Job

Cancel a running job:

```bash
curl -X DELETE http://localhost:8080/jobs/job-abc123def456
```

**Response:**
```json
{
  "cancelled": true,
  "job_id": "job-abc123def456"
}
```

---

## Streaming Output

### Stream with curl

Watch output in real-time:

```bash
curl -N "http://localhost:8080/run/stream?prompt=Write%20a%20REST%20API%20in%20Go&workspace=/home/user/project"
```

**Output:**
```
data: Analyzing your request...

data: I'll create a REST API in Go for you.

data: Creating file main.go...

data: package main

data: import (

data:     "encoding/json"

event: done
data: {"id":"run-abc123","session_id":"sess-def456","status":"completed"}
```

### Stream with JavaScript

```javascript
const prompt = encodeURIComponent("Write a hello world program");
const eventSource = new EventSource(
  `http://localhost:8080/run/stream?prompt=${prompt}`
);

let output = '';

eventSource.onmessage = (event) => {
  output += event.data + '\n';
  console.log(event.data);
  document.getElementById('output').textContent = output;
};

eventSource.addEventListener('done', (event) => {
  const result = JSON.parse(event.data);
  console.log('Completed:', result);
  console.log('Session ID:', result.session_id);
  eventSource.close();
});

eventSource.addEventListener('error', (event) => {
  console.error('Error:', event.data);
  eventSource.close();
});

eventSource.onerror = (error) => {
  console.error('Connection error:', error);
  eventSource.close();
};
```

### Stream with Python

```python
import requests
import json

url = "http://localhost:8080/run/stream"
params = {
    "prompt": "Write a Python function to sort a list",
    "workspace": "/home/user/project"
}

with requests.get(url, params=params, stream=True) as response:
    for line in response.iter_lines():
        if line:
            line_str = line.decode('utf-8')

            if line_str.startswith('data: '):
                data = line_str[6:]
                print(data)

            elif line_str.startswith('event: done'):
                # Next line contains the result
                continue

            elif line_str.startswith('event: error'):
                print(f"Error occurred", file=sys.stderr)
```

---

## Webhook Integration

### Async Job with Webhook

Submit job with webhook notification:

```bash
curl -X POST http://localhost:8080/run/async \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Add integration tests to the API endpoints",
    "workspace": "/home/user/api-project",
    "webhook_url": "https://myapp.com/api/webhook",
    "claude": {
      "model": "sonnet",
      "dangerously_skip_permissions": true
    }
  }'
```

### Webhook Handler (Express.js)

```javascript
const express = require('express');
const app = express();

app.use(express.json());

app.post('/api/webhook', (req, res) => {
  const { job_id, status, output, error, session_id } = req.body;

  console.log(`Job ${job_id} status: ${status}`);

  if (status === 'completed') {
    console.log('Output:', output);
    console.log('Session:', session_id);

    // Process the completed job
    processCompletedJob(job_id, output, session_id);
  } else if (status === 'failed') {
    console.error('Job failed:', error);

    // Handle failure
    handleJobFailure(job_id, error);
  } else if (status === 'cancelled') {
    console.log('Job was cancelled');
  }

  res.status(200).send('OK');
});

app.listen(3000, () => {
  console.log('Webhook server listening on port 3000');
});
```

### Webhook Handler (Go)

```go
package main

import (
    "encoding/json"
    "log"
    "net/http"
)

type WebhookPayload struct {
    JobID     string `json:"job_id"`
    Status    string `json:"status"`
    Output    string `json:"output"`
    Error     string `json:"error"`
    SessionID string `json:"session_id"`
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
    var payload WebhookPayload

    if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
        http.Error(w, "Invalid payload", http.StatusBadRequest)
        return
    }

    log.Printf("Received webhook for job %s: %s", payload.JobID, payload.Status)

    switch payload.Status {
    case "completed":
        log.Printf("Job completed with output length: %d", len(payload.Output))
        log.Printf("Session ID: %s", payload.SessionID)
        // Process completed job

    case "failed":
        log.Printf("Job failed with error: %s", payload.Error)
        // Handle failure

    case "cancelled":
        log.Printf("Job was cancelled")
    }

    w.WriteHeader(http.StatusOK)
}

func main() {
    http.HandleFunc("/webhook", webhookHandler)
    log.Fatal(http.ListenAndServe(":8081", nil))
}
```

### Webhook Handler (Python/Flask)

```python
from flask import Flask, request, jsonify
import logging

app = Flask(__name__)
logging.basicConfig(level=logging.INFO)

@app.route('/webhook', methods=['POST'])
def webhook():
    payload = request.get_json()

    job_id = payload.get('job_id')
    status = payload.get('status')
    output = payload.get('output')
    error = payload.get('error')
    session_id = payload.get('session_id')

    logging.info(f"Job {job_id} status: {status}")

    if status == 'completed':
        logging.info(f"Output length: {len(output)}")
        logging.info(f"Session: {session_id}")
        # Process completed job
        process_completed_job(job_id, output, session_id)

    elif status == 'failed':
        logging.error(f"Job failed: {error}")
        # Handle failure
        handle_failure(job_id, error)

    elif status == 'cancelled':
        logging.info("Job was cancelled")

    return jsonify({"status": "ok"}), 200

if __name__ == '__main__':
    app.run(port=8081)
```

---

## Resource Limits

### Memory and CPU Limits

Control container resources:

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Analyze this large codebase",
    "workspace": "/home/user/large-project",
    "claude": {
      "model": "opus",
      "dangerously_skip_permissions": true
    },
    "podman": {
      "timeout": "15m",
      "memory": "4g",
      "cpus": "4",
      "cpu_shares": 2048
    }
  }'
```

### Timeout Configuration

Set different timeout levels:

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Quick code review",
    "workspace": "/home/user/project",
    "claude": {
      "dangerously_skip_permissions": true
    },
    "podman": {
      "timeout": "2m",
      "memory": "512m",
      "cpus": "1"
    }
  }'
```

### Additional Volume Mounts

Mount extra directories:

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Compare files between these directories",
    "workspace": "/home/user/project",
    "claude": {
      "dangerously_skip_permissions": true
    },
    "podman": {
      "volumes": [
        "/home/user/reference:/reference:ro",
        "/home/user/cache:/cache:rw"
      ]
    }
  }'
```

---

## Advanced Claude Options

### Custom System Prompt

Customize Claude's behavior:

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Review this code for security vulnerabilities",
    "workspace": "/home/user/webapp",
    "claude": {
      "model": "sonnet",
      "system_prompt": "You are a security expert specializing in web application security. Focus on OWASP Top 10 vulnerabilities and provide specific remediation advice.",
      "dangerously_skip_permissions": true
    }
  }'
```

### Tool Restrictions

Limit which tools Claude can use:

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Analyze the code structure",
    "workspace": "/home/user/project",
    "claude": {
      "allowed_tools": ["Read", "Grep", "Glob"],
      "dangerously_skip_permissions": true
    }
  }'
```

### Budget Control

Set spending limits:

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Comprehensive code review and refactoring",
    "workspace": "/home/user/project",
    "claude": {
      "model": "opus",
      "max_budget_usd": 5.0,
      "dangerously_skip_permissions": true
    }
  }'
```

### JSON Schema Output

Request structured output:

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Analyze this code and provide a security report",
    "workspace": "/home/user/project",
    "claude": {
      "output_format": "json",
      "json_schema": "{\"type\":\"object\",\"properties\":{\"vulnerabilities\":{\"type\":\"array\",\"items\":{\"type\":\"object\",\"properties\":{\"severity\":{\"type\":\"string\"},\"description\":{\"type\":\"string\"},\"file\":{\"type\":\"string\"},\"line\":{\"type\":\"number\"}}}},\"summary\":{\"type\":\"string\"}},\"required\":[\"vulnerabilities\",\"summary\"]}",
      "dangerously_skip_permissions": true
    }
  }'
```

### Fork Session

Create a new branch from existing session:

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Try a different approach to the same problem",
    "workspace": "/home/user/project",
    "claude": {
      "session_id": "sess-550e8400-e29b-41d4-a716-446655440000",
      "resume": true,
      "fork_session": true,
      "dangerously_skip_permissions": true
    }
  }'
```

---

## Error Handling

### Handle Missing Prompt

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "workspace": "/home/user/project"
  }'
```

**Response:** `400 Bad Request`
```json
{
  "status": "error",
  "error": "Invalid request: Key: 'RunRequest.Prompt' Error:Field validation for 'Prompt' failed on the 'required' tag"
}
```

### Handle Claude Not Configured

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Hello"
  }'
```

**Response:** `503 Service Unavailable`
```json
{
  "status": "error",
  "error": "Claude not configured. Run 'make claude-setup' first"
}
```

### Handle Invalid Session

```bash
curl -X DELETE http://localhost:8080/sessions/invalid-session-id
```

**Response:** `400 Bad Request`
```json
{
  "success": false,
  "error": "invalid session ID"
}
```

### Robust Error Handling (Python)

```python
import requests
import time

def execute_claude(prompt, workspace=None, max_retries=3):
    url = "http://localhost:8080/run"
    payload = {
        "prompt": prompt,
        "claude": {
            "dangerously_skip_permissions": True
        }
    }

    if workspace:
        payload["workspace"] = workspace

    for attempt in range(max_retries):
        try:
            response = requests.post(url, json=payload, timeout=300)

            if response.status_code == 200:
                result = response.json()
                return result.get("output"), result.get("session_id")

            elif response.status_code == 503:
                print("Claude not configured")
                return None, None

            elif response.status_code == 400:
                error = response.json().get("error")
                print(f"Bad request: {error}")
                return None, None

            elif response.status_code == 500:
                print(f"Server error (attempt {attempt + 1}/{max_retries})")
                if attempt < max_retries - 1:
                    time.sleep(5 * (attempt + 1))
                    continue
                return None, None

        except requests.exceptions.Timeout:
            print(f"Timeout (attempt {attempt + 1}/{max_retries})")
            if attempt < max_retries - 1:
                time.sleep(5)
                continue
            return None, None

        except requests.exceptions.ConnectionError:
            print(f"Connection error (attempt {attempt + 1}/{max_retries})")
            if attempt < max_retries - 1:
                time.sleep(10)
                continue
            return None, None

    return None, None

# Usage
output, session_id = execute_claude(
    "Write a hello world program",
    workspace="/home/user/project"
)

if output:
    print("Success!")
    print(output)
    print(f"Session: {session_id}")
else:
    print("Failed to execute")
```

---

## Integration Examples

### CI/CD Pipeline (GitHub Actions)

```yaml
name: AI Code Review

on:
  pull_request:
    types: [opened, synchronize]

jobs:
  ai-review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: AI Code Review
        run: |
          RESULT=$(curl -X POST http://stromboli:8080/run \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${{ secrets.STROMBOLI_TOKEN }}" \
            -d '{
              "prompt": "Review this pull request for bugs, security issues, and code quality. Focus on changes in the current commit.",
              "workspace": "'${GITHUB_WORKSPACE}'",
              "claude": {
                "model": "sonnet",
                "system_prompt": "You are a senior software engineer doing code review. Be constructive and specific.",
                "dangerously_skip_permissions": true
              }
            }')

          OUTPUT=$(echo $RESULT | jq -r '.output')

          # Post as PR comment
          gh pr comment ${{ github.event.pull_request.number }} \
            --body "## AI Code Review\n\n${OUTPUT}"
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### n8n Workflow Integration

```json
{
  "nodes": [
    {
      "name": "Webhook",
      "type": "n8n-nodes-base.webhook",
      "position": [250, 300],
      "parameters": {
        "path": "code-review",
        "responseMode": "responseNode"
      }
    },
    {
      "name": "Stromboli Async",
      "type": "n8n-nodes-base.httpRequest",
      "position": [450, 300],
      "parameters": {
        "method": "POST",
        "url": "http://stromboli:8080/run/async",
        "authentication": "genericCredentialType",
        "options": {},
        "bodyParameters": {
          "parameters": [
            {
              "name": "prompt",
              "value": "={{$json[\"body\"][\"prompt\"]}}"
            },
            {
              "name": "workspace",
              "value": "={{$json[\"body\"][\"workspace\"]}}"
            },
            {
              "name": "webhook_url",
              "value": "http://n8n:5678/webhook/stromboli-result"
            }
          ]
        }
      }
    },
    {
      "name": "Return Job ID",
      "type": "n8n-nodes-base.respondToWebhook",
      "position": [650, 300],
      "parameters": {
        "respondWith": "json",
        "responseBody": "={{$json}}"
      }
    }
  ]
}
```

### Docker Compose Setup

```yaml
version: '3.8'

services:
  stromboli:
    image: stromboli:latest
    ports:
      - "8080:8080"
    environment:
      - STROMBOLI_AUTH_ENABLED=true
      - STROMBOLI_API_TOKENS=${STROMBOLI_TOKENS}
    volumes:
      - /var/run/podman/podman.sock:/var/run/podman/podman.sock
      - ./workspaces:/workspaces:rw
      - ./.claude-secrets:/app/.claude-secrets:ro
    restart: unless-stopped
    networks:
      - app-network

  webhook-receiver:
    image: webhook-receiver:latest
    ports:
      - "8081:8081"
    environment:
      - STROMBOLI_URL=http://stromboli:8080
    networks:
      - app-network

networks:
  app-network:
    driver: bridge
```

### Monitoring with Prometheus

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'stromboli'
    static_configs:
      - targets: ['stromboli:8080']
    metrics_path: '/metrics'
    scrape_interval: 15s
```

Query examples:
```promql
# Request rate
rate(stromboli_api_requests_total[5m])

# Success rate
rate(stromboli_api_requests_total{status="200"}[5m]) /
rate(stromboli_api_requests_total[5m])

# Job completion time
histogram_quantile(0.95, stromboli_job_duration_seconds_bucket)
```

---

## Best Practices

### Always Set Resource Limits

```bash
{
  "podman": {
    "timeout": "10m",
    "memory": "2g",
    "cpus": "2"
  }
}
```

### Use Sessions for Multi-Turn Conversations

```bash
# First turn
RESPONSE=$(curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{"prompt": "Create project structure"}')

SESSION_ID=$(echo $RESPONSE | jq -r '.session_id')

# Subsequent turns
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d "{
    \"prompt\": \"Add tests\",
    \"claude\": {
      \"session_id\": \"$SESSION_ID\",
      \"resume\": true
    }
  }"
```

### Clean Up Sessions Regularly

```bash
# Get all sessions
SESSIONS=$(curl -s http://localhost:8080/sessions | jq -r '.sessions[]')

# Delete old sessions
for session in $SESSIONS; do
  curl -X DELETE http://localhost:8080/sessions/$session
done
```

### Use Async for Long Tasks

```bash
# Don't use sync /run for tasks >30 seconds
# Use async instead
curl -X POST http://localhost:8080/run/async \
  -d '{"prompt": "Long task..."}'
```

### Implement Webhook Fallback

```python
# Start async job with webhook
response = requests.post('http://localhost:8080/run/async', json={
    "prompt": "Task",
    "webhook_url": "http://myapp/webhook"
})

job_id = response.json()['job_id']

# Poll as fallback (in case webhook fails)
time.sleep(60)
result = requests.get(f'http://localhost:8080/jobs/{job_id}')
```

---

For more details, see:
- [API Documentation](API.md)
- [Architecture](ARCHITECTURE.md)
- [Project README](../README.md)
