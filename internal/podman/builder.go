package podman

import "fmt"

// CommandBuilder builds podman run commands with a fluent API
type CommandBuilder struct {
	name        string
	image       string
	env         map[string]string
	volumes     []string
	secrets     []string // podman secrets (--secret flag)
	tmpfs       []string // tmpfs mounts for isolation
	workdir     string
	interactive bool
	command     []string
	memory      string // memory limit (e.g., "512m", "1g")
	cpus        string // CPU limit (e.g., "0.5", "2")
	cpuShares   int    // CPU shares (relative weight)
	user        string // user to run as (e.g., "1000:1000")
	keepID      bool   // use --userns=keep-id for host user mapping
}

// NewCommand creates a new podman command builder
func NewCommand() *CommandBuilder {
	return &CommandBuilder{
		env: make(map[string]string),
	}
}

// WithName sets the container name
func (b *CommandBuilder) WithName(name string) *CommandBuilder {
	b.name = name
	return b
}

// WithImage sets the container image
func (b *CommandBuilder) WithImage(image string) *CommandBuilder {
	b.image = image
	return b
}

// WithEnv adds an environment variable
func (b *CommandBuilder) WithEnv(key, value string) *CommandBuilder {
	b.env[key] = value
	return b
}

// WithVolume adds a volume mount
func (b *CommandBuilder) WithVolume(hostPath, containerPath string) *CommandBuilder {
	b.volumes = append(b.volumes, hostPath+":"+containerPath)
	return b
}

// WithVolumeRaw adds a raw volume string (for complex mounts like :ro, :Z, etc.)
func (b *CommandBuilder) WithVolumeRaw(volume string) *CommandBuilder {
	b.volumes = append(b.volumes, volume)
	return b
}

// WithVolumeReadOnly adds a read-only volume mount
func (b *CommandBuilder) WithVolumeReadOnly(hostPath, containerPath string) *CommandBuilder {
	b.volumes = append(b.volumes, hostPath+":"+containerPath+":ro")
	return b
}

// WithVolumeChown adds a volume mount with :U flag
// The :U flag tells Podman to adjust ownership for the container user
// This allows the container user to write to the mounted directory
func (b *CommandBuilder) WithVolumeChown(hostPath, containerPath string) *CommandBuilder {
	b.volumes = append(b.volumes, hostPath+":"+containerPath+":U")
	return b
}

// WithSecret adds a podman secret to the container
// The secret must be created beforehand with `podman secret create`
// Secret is available at /run/secrets/<name> inside the container
func (b *CommandBuilder) WithSecret(name string) *CommandBuilder {
	b.secrets = append(b.secrets, name)
	return b
}

// WithSecretTarget adds a podman secret mounted at a custom target path
// The secret must be created beforehand with `podman secret create`
// Example: WithSecretTarget("claude-credentials", "/home/user/.claude/.credentials.json")
func (b *CommandBuilder) WithSecretTarget(name, targetPath string) *CommandBuilder {
	b.secrets = append(b.secrets, fmt.Sprintf("%s,target=%s", name, targetPath))
	return b
}

// WithSecretFile is deprecated - use WithSecret instead
// This mounts a file read-only but has permission issues with rootless podman
func (b *CommandBuilder) WithSecretFile(hostPath, containerPath string) *CommandBuilder {
	b.volumes = append(b.volumes, hostPath+":"+containerPath+":ro")
	return b
}

// WithTmpfs adds a tmpfs mount (ephemeral in-memory storage)
// Useful for session isolation - data is lost when container stops
func (b *CommandBuilder) WithTmpfs(containerPath string) *CommandBuilder {
	b.tmpfs = append(b.tmpfs, containerPath)
	return b
}

// WithTmpfsSized adds a tmpfs mount with size limit
func (b *CommandBuilder) WithTmpfsSized(containerPath, size string) *CommandBuilder {
	b.tmpfs = append(b.tmpfs, containerPath+":size="+size)
	return b
}

// WithWorkdir sets the working directory
func (b *CommandBuilder) WithWorkdir(dir string) *CommandBuilder {
	b.workdir = dir
	return b
}

// WithInteractive enables interactive mode (-it)
func (b *CommandBuilder) WithInteractive() *CommandBuilder {
	b.interactive = true
	return b
}

// WithCommand sets the command to run in the container
func (b *CommandBuilder) WithCommand(cmd []string) *CommandBuilder {
	b.command = cmd
	return b
}

// WithMemory sets the memory limit (e.g., "512m", "1g")
func (b *CommandBuilder) WithMemory(limit string) *CommandBuilder {
	b.memory = limit
	return b
}

// WithCPUs sets the CPU limit (e.g., "0.5", "2")
func (b *CommandBuilder) WithCPUs(cpus string) *CommandBuilder {
	b.cpus = cpus
	return b
}

// WithCPUShares sets the CPU shares (relative weight, default 1024)
func (b *CommandBuilder) WithCPUShares(shares int) *CommandBuilder {
	b.cpuShares = shares
	return b
}

// WithUser sets the user to run the container as (e.g., "1000:1000")
func (b *CommandBuilder) WithUser(user string) *CommandBuilder {
	b.user = user
	return b
}

// WithKeepID enables --userns=keep-id for host user ID mapping
// This maps the host user to the same UID inside the container
func (b *CommandBuilder) WithKeepID() *CommandBuilder {
	b.keepID = true
	return b
}

// Build generates the final podman command
func (b *CommandBuilder) Build() []string {
	args := []string{"podman", "run", "--rm"}

	if b.keepID {
		args = append(args, "--userns=keep-id")
	}

	if b.user != "" {
		args = append(args, "--user", b.user)
	}

	if b.interactive {
		args = append(args, "-it")
	}

	if b.name != "" {
		args = append(args, "--name", b.name)
	}

	for key, value := range b.env {
		args = append(args, "-e", key+"="+value)
	}

	for _, vol := range b.volumes {
		args = append(args, "-v", vol)
	}

	for _, secret := range b.secrets {
		args = append(args, "--secret", secret)
	}

	for _, tmpfs := range b.tmpfs {
		args = append(args, "--tmpfs", tmpfs)
	}

	if b.workdir != "" {
		args = append(args, "-w", b.workdir)
	}

	// Resource limits
	if b.memory != "" {
		args = append(args, "--memory", b.memory)
	}

	if b.cpus != "" {
		args = append(args, "--cpus", b.cpus)
	}

	if b.cpuShares > 0 {
		args = append(args, "--cpu-shares", fmt.Sprintf("%d", b.cpuShares))
	}

	if b.image != "" {
		args = append(args, b.image)
	}

	if len(b.command) > 0 {
		args = append(args, b.command...)
	}

	return args
}
