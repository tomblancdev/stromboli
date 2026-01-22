# Resource Limits

Stromboli allows you to configure resource limits for the containers running Claude Code agents. This helps prevent resource exhaustion and ensures fair resource allocation across multiple concurrent executions.

## Default Resource Limits

As of v1.0, Stromboli applies default resource limits to all containers unless explicitly overridden. This provides a safe baseline for execution without requiring manual configuration.

### Default Values

| Resource | Default Value | Description |
|----------|---------------|-------------|
| Memory | 512m | Maximum memory allocation per container |
| CPUs | 1 | Number of CPU cores allocated per container |
| Timeout | 30m | Maximum execution time before timeout |

### Environment Variable Configuration

You can override the default values using environment variables:

```bash
# Set default memory limit (e.g., "512m", "1g", "2g")
export STROMBOLI_DEFAULT_MEMORY="1g"

# Set default CPU limit (e.g., "0.5", "1", "2", "4")
export STROMBOLI_DEFAULT_CPUS="2"

# Set default timeout (e.g., "5m", "30m", "1h", "2h")
export STROMBOLI_DEFAULT_TIMEOUT="1h"
```

These environment variables are read at startup and applied to all requests that don't specify explicit resource limits.

## Per-Request Resource Limits

You can override the defaults on a per-request basis by specifying resource limits in the request payload:

### Example: Override Memory and CPUs

```json
{
  "prompt": "Analyze this large codebase",
  "podman": {
    "memory": "2g",
    "cpus": "4"
  }
}
```

In this example:
- Memory is set to 2GB (overrides default of 512m)
- CPUs is set to 4 cores (overrides default of 1)
- Timeout uses the default of 30m (not specified in request)

### Example: Override Only Timeout

```json
{
  "prompt": "Run long-running integration tests",
  "podman": {
    "timeout": "2h"
  }
}
```

In this example:
- Timeout is set to 2 hours (overrides default of 30m)
- Memory uses the default of 512m (not specified in request)
- CPUs uses the default of 1 (not specified in request)

### Example: Use All Defaults

```json
{
  "prompt": "Simple task",
  "podman": {}
}
```

In this example, all default values are applied:
- Memory: 512m
- CPUs: 1
- Timeout: 30m

## Resource Limit Formats

### Memory Format

Memory limits use Podman's memory format:
- `512m` - 512 megabytes
- `1g` - 1 gigabyte
- `2g` - 2 gigabytes

### CPU Format

CPU limits specify the number of CPU cores:
- `0.5` - Half a CPU core
- `1` - One CPU core
- `2` - Two CPU cores
- `4` - Four CPU cores

### Timeout Format

Timeout uses Go's duration format:
- `5m` - 5 minutes
- `30m` - 30 minutes
- `1h` - 1 hour
- `2h` - 2 hours
- `1h30m` - 1 hour and 30 minutes

## Behavior

### Application Order

1. Stromboli applies defaults from environment variables (or hardcoded defaults if not set)
2. Per-request values override defaults for that specific execution
3. If a resource is not specified in the request, the default is used

### Backward Compatibility

The resource defaults feature is backward compatible:
- Existing code that uses `NewPodmanRunner()` continues to work (no defaults applied)
- New code can use `NewPodmanRunnerWithDefaults()` to enable defaults
- The main application uses defaults automatically
- API requests can omit resource limits and get sensible defaults

### Why Defaults?

Before v1.0, resource limits had to be specified for every request. This had several drawbacks:

1. **Manual Configuration**: Every client had to remember to set limits
2. **No Safety Net**: Forgot to set limits? Risk resource exhaustion
3. **Inconsistent Limits**: Different clients might use wildly different values

With defaults:

1. **Automatic Protection**: Every container gets sensible limits by default
2. **Consistency**: All executions use the same baseline unless overridden
3. **Flexibility**: Can still override when needed for specific workloads
4. **Production-Ready**: Safe defaults for production deployments

## Best Practices

### Development Environment

For local development, the defaults are appropriate:

```bash
# Use defaults (512m memory, 1 CPU, 30m timeout)
# No environment variables needed
```

### Production Environment

For production, tune defaults based on your workload:

```bash
# Higher memory for complex tasks
export STROMBOLI_DEFAULT_MEMORY="1g"

# More CPUs for parallel processing
export STROMBOLI_DEFAULT_CPUS="2"

# Longer timeout for long-running jobs
export STROMBOLI_DEFAULT_TIMEOUT="1h"
```

### Special Workloads

For specific workloads that need different limits, override per-request:

```bash
# Large codebase analysis
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Analyze entire codebase",
    "podman": {
      "memory": "4g",
      "cpus": "8",
      "timeout": "2h"
    }
  }'
```

```bash
# Quick query
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Quick question",
    "podman": {
      "memory": "256m",
      "timeout": "5m"
    }
  }'
```

## Monitoring

Watch resource usage to tune your defaults:

```bash
# Monitor container resource usage
podman stats

# Check for OOM kills
journalctl -u stromboli | grep -i "out of memory"

# Monitor timeout errors
journalctl -u stromboli | grep -i timeout
```

## Troubleshooting

### Out of Memory Errors

If containers are running out of memory:

1. Increase default memory: `export STROMBOLI_DEFAULT_MEMORY="1g"`
2. Or override per-request: `"memory": "2g"`

### CPU Throttling

If tasks are slow due to CPU limits:

1. Increase default CPUs: `export STROMBOLI_DEFAULT_CPUS="2"`
2. Or override per-request: `"cpus": "4"`

### Timeout Errors

If executions are timing out:

1. Increase default timeout: `export STROMBOLI_DEFAULT_TIMEOUT="1h"`
2. Or override per-request: `"timeout": "2h"`

## See Also

- [API Documentation](API.md) - Full API reference
- [Architecture](ARCHITECTURE.md) - System architecture overview
- [Examples](EXAMPLES.md) - More usage examples
