package compose

// CommandBuilder builds podman compose commands with a fluent API
type CommandBuilder struct {
	file        string
	project     string
	command     string   // up, down, exec, ps, etc.
	service     string   // service name for exec
	execArgs    []string // command args for exec
	detached    bool
	build       bool
	removeVolumes bool
	noTTY       bool     // -T flag for exec (disable pseudo-tty)
	formatJSON  bool     // --format json for ps
}

// NewCommandBuilder creates a new compose command builder
func NewCommandBuilder() *CommandBuilder {
	return &CommandBuilder{}
}

// WithFile sets the compose file path (-f)
func (b *CommandBuilder) WithFile(path string) *CommandBuilder {
	b.file = path
	return b
}

// WithProject sets the project name (-p)
func (b *CommandBuilder) WithProject(name string) *CommandBuilder {
	b.project = name
	return b
}

// Up configures the builder for "up" command
func (b *CommandBuilder) Up() *CommandBuilder {
	b.command = "up"
	return b
}

// Down configures the builder for "down" command
func (b *CommandBuilder) Down() *CommandBuilder {
	b.command = "down"
	return b
}

// Exec configures the builder for "exec" command
func (b *CommandBuilder) Exec(service string, args ...string) *CommandBuilder {
	b.command = "exec"
	b.service = service
	b.execArgs = args
	b.noTTY = true // Default to no TTY for non-interactive use
	return b
}

// Ps configures the builder for "ps" command
func (b *CommandBuilder) Ps() *CommandBuilder {
	b.command = "ps"
	return b
}

// Detached adds the -d flag (for up)
func (b *CommandBuilder) Detached() *CommandBuilder {
	b.detached = true
	return b
}

// WithBuild adds the --build flag (for up)
func (b *CommandBuilder) WithBuild() *CommandBuilder {
	b.build = true
	return b
}

// RemoveVolumes adds the -v flag (for down)
func (b *CommandBuilder) RemoveVolumes() *CommandBuilder {
	b.removeVolumes = true
	return b
}

// WithTTY enables TTY allocation for exec (removes -T flag)
func (b *CommandBuilder) WithTTY() *CommandBuilder {
	b.noTTY = false
	return b
}

// FormatJSON adds --format json (for ps)
func (b *CommandBuilder) FormatJSON() *CommandBuilder {
	b.formatJSON = true
	return b
}

// Build generates the final podman compose command
func (b *CommandBuilder) Build() []string {
	args := []string{"podman", "compose"}

	// Add file if specified
	if b.file != "" {
		args = append(args, "-f", b.file)
	}

	// Add project name if specified
	if b.project != "" {
		args = append(args, "-p", b.project)
	}

	// Add command
	if b.command != "" {
		args = append(args, b.command)
	}

	// Add command-specific flags
	switch b.command {
	case "up":
		if b.detached {
			args = append(args, "-d")
		}
		if b.build {
			args = append(args, "--build")
		}
	case "down":
		if b.removeVolumes {
			args = append(args, "-v")
		}
	case "exec":
		if b.noTTY {
			args = append(args, "-T")
		}
		if b.service != "" {
			args = append(args, b.service)
		}
		args = append(args, b.execArgs...)
	case "ps":
		if b.formatJSON {
			args = append(args, "--format", "json")
		}
	}

	return args
}
