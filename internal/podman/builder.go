package podman

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
// The :U flag tells Podman to recursively chown the volume to match the container user
// This is essential for rootless containers writing to mounted volumes
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

// Build generates the final podman command
func (b *CommandBuilder) Build() []string {
	args := []string{"podman", "run", "--rm"}

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

	if b.image != "" {
		args = append(args, b.image)
	}

	if len(b.command) > 0 {
		args = append(args, b.command...)
	}

	return args
}
