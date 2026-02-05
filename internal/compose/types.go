package compose

import "time"

// Environment specifies compose-based environment configuration
// @Description Compose environment configuration for multi-service setups
type Environment struct {
	// Type of environment ("compose")
	Type string `json:"type" example:"compose"`

	// Absolute path to the compose file
	Path string `json:"path" example:"/home/user/project/docker-compose.yml"`

	// Service name where Claude agent will run
	Service string `json:"service" example:"dev"`

	// Optional build timeout override (e.g., "15m")
	BuildTimeout string `json:"build_timeout,omitempty" example:"15m"`
}

// Config holds compose security and operational settings
type Config struct {
	// AllowPrivileged permits services with privileged: true
	AllowPrivileged bool

	// AllowHostNetwork permits services with network_mode: host
	AllowHostNetwork bool

	// AllowHostVolumes permits host volume mounts
	AllowHostVolumes bool

	// BuildTimeout is the maximum time for compose build/up (default: 10m)
	BuildTimeout time.Duration

	// HealthTimeout is the maximum time to wait for services to become healthy (default: 2m)
	HealthTimeout time.Duration

	// StackTTL is the maximum age for orphaned stacks before cleanup (default: 1h)
	StackTTL time.Duration
}

// DefaultConfig returns a Config with secure defaults
func DefaultConfig() Config {
	return Config{
		AllowPrivileged:  false,
		AllowHostNetwork: false,
		AllowHostVolumes: false,
		BuildTimeout:     10 * time.Minute,
		HealthTimeout:    2 * time.Minute,
		StackTTL:         1 * time.Hour,
	}
}

// StackState represents the lifecycle state of a compose stack
type StackState string

const (
	// StackStateStarting indicates the stack is being initialized
	StackStateStarting StackState = "starting"
	// StackStateRunning indicates the stack is fully operational
	StackStateRunning StackState = "running"
	// StackStateStopping indicates the stack is being torn down
	StackStateStopping StackState = "stopping"
)

// Stack represents a running compose stack
type Stack struct {
	// ProjectName is the compose project name (stromboli-{session_id})
	ProjectName string

	// ComposePath is the absolute path to the compose file
	ComposePath string

	// Service is the target service for Claude execution
	Service string

	// SessionID links the stack to a Stromboli session
	SessionID string

	// StartedAt records when the stack was started
	StartedAt time.Time

	// State is the current lifecycle state
	State StackState
}

// IsReady returns true if the stack is in a usable state
func (s *Stack) IsReady() bool {
	return s.State == StackStateRunning
}

// ServiceStatus represents the health status of a service
type ServiceStatus struct {
	// Name is the service name
	Name string

	// State is the container state (running, exited, etc.)
	State string

	// Health is the health check status (healthy, unhealthy, starting, none)
	Health string

	// ExitCode is the exit code if the container has exited
	ExitCode int
}

// StackStatus represents the overall status of a compose stack
type StackStatus struct {
	// ProjectName is the compose project name
	ProjectName string

	// Services contains the status of each service
	Services []ServiceStatus

	// AllHealthy is true if all services are running and healthy
	AllHealthy bool
}

// ProjectPrefix is the prefix for all Stromboli compose projects
const ProjectPrefix = "stromboli-"

// ProjectName generates a compose project name from a session ID
func ProjectName(sessionID string) string {
	return ProjectPrefix + sessionID
}
