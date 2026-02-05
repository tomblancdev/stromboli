// Package compose provides compose-based environment orchestration for Stromboli.
//
// It enables Claude agents to run in multi-service environments defined by
// Docker/Podman Compose files. This allows agents to work alongside databases,
// caches, frontends, and other services.
//
// # Overview
//
// The compose package manages the lifecycle of compose stacks:
//   - Validate compose files for security compliance
//   - Start stacks with health check polling
//   - Execute commands in specific services
//   - Clean up stacks on session destroy or TTL expiry
//
// # Security
//
// By default, the package blocks dangerous configurations:
//   - privileged: true (requires AllowPrivileged config)
//   - network_mode: host (requires AllowHostNetwork config)
//   - Host volume mounts (requires AllowHostVolumes config)
//
// # Usage
//
//	cfg := compose.Config{
//	    AllowPrivileged:  false,
//	    AllowHostNetwork: false,
//	    AllowHostVolumes: false,
//	    BuildTimeout:     10 * time.Minute,
//	    HealthTimeout:    2 * time.Minute,
//	    StackTTL:         1 * time.Hour,
//	}
//
//	mgr := compose.NewManager(executor, cfg)
//
//	stack, err := mgr.Up(ctx, compose.Environment{
//	    Type:    "compose",
//	    Path:    "/path/to/docker-compose.yml",
//	    Service: "dev",
//	}, "session-123")
//
//	output, err := mgr.Exec(ctx, "session-123", "dev", []string{"echo", "hello"})
//
//	err = mgr.Down(ctx, "session-123")
package compose
