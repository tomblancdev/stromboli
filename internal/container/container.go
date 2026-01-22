package container

import (
	"context"
	"errors"
	"os/exec"
	"strings"

	strerrors "github.com/tomblanc/stromboli/internal/errors"
)

// Mount represents a volume mount
type Mount struct {
	Source   string
	Target   string
	ReadOnly bool
}

// Spec defines container configuration
type Spec struct {
	Name   string
	Image  string
	Mounts []Mount
	Env    map[string]string
	Cmd    []string
}

// Manager handles Podman container operations
type Manager struct{}

// NewManager creates a new container manager
func NewManager() *Manager {
	return &Manager{}
}

// Create creates a new container (does not start it)
func (m *Manager) Create(ctx context.Context, spec Spec) (string, error) {
	args := []string{"create", "--name", spec.Name}

	for _, mount := range spec.Mounts {
		mountStr := mount.Source + ":" + mount.Target
		if mount.ReadOnly {
			mountStr += ":ro"
		}
		args = append(args, "-v", mountStr)
	}

	for key, val := range spec.Env {
		args = append(args, "-e", key+"="+val)
	}

	args = append(args, spec.Image)
	args = append(args, spec.Cmd...)

	cmd := exec.CommandContext(ctx, "podman", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", errors.Join(strerrors.ErrCommandFailed, errors.New(string(output)))
	}

	return strings.TrimSpace(string(output)), nil
}

// Start starts an existing container
func (m *Manager) Start(ctx context.Context, nameOrID string) error {
	cmd := exec.CommandContext(ctx, "podman", "start", nameOrID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Join(strerrors.ErrCommandFailed, errors.New(string(output)))
	}
	return nil
}

// Stop stops a running container
func (m *Manager) Stop(ctx context.Context, nameOrID string) error {
	cmd := exec.CommandContext(ctx, "podman", "stop", nameOrID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Join(strerrors.ErrCommandFailed, errors.New(string(output)))
	}
	return nil
}

// Remove removes a container
func (m *Manager) Remove(ctx context.Context, nameOrID string) error {
	cmd := exec.CommandContext(ctx, "podman", "rm", nameOrID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Join(strerrors.ErrCommandFailed, errors.New(string(output)))
	}
	return nil
}

// Run creates and starts a container (shorthand)
func (m *Manager) Run(ctx context.Context, spec Spec) (string, error) {
	id, err := m.Create(ctx, spec)
	if err != nil {
		return "", err
	}

	if err := m.Start(ctx, id); err != nil {
		m.Remove(ctx, id)
		return "", err
	}

	return id, nil
}

// VolumeExists checks if a volume exists
func (m *Manager) VolumeExists(ctx context.Context, name string) (bool, error) {
	cmd := exec.CommandContext(ctx, "podman", "volume", "inspect", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "no such volume") {
			return false, nil
		}
		return false, errors.Join(strerrors.ErrCommandFailed, errors.New(string(output)))
	}
	return true, nil
}

// VolumeCreate creates a new volume
func (m *Manager) VolumeCreate(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "podman", "volume", "create", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Join(strerrors.ErrCommandFailed, errors.New(string(output)))
	}
	return nil
}

// VolumeRemove removes a volume
func (m *Manager) VolumeRemove(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "podman", "volume", "rm", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Join(strerrors.ErrCommandFailed, errors.New(string(output)))
	}
	return nil
}
