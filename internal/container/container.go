package container

import (
	"errors"
	"os/exec"
	"strings"
)

var ErrContainerNotFound = errors.New("container not found")
var ErrCommandFailed = errors.New("podman command failed")

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
func (m *Manager) Create(spec Spec) (string, error) {
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

	cmd := exec.Command("podman", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", errors.Join(ErrCommandFailed, errors.New(string(output)))
	}

	return strings.TrimSpace(string(output)), nil
}

// Start starts an existing container
func (m *Manager) Start(nameOrID string) error {
	cmd := exec.Command("podman", "start", nameOrID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Join(ErrCommandFailed, errors.New(string(output)))
	}
	return nil
}

// Stop stops a running container
func (m *Manager) Stop(nameOrID string) error {
	cmd := exec.Command("podman", "stop", nameOrID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Join(ErrCommandFailed, errors.New(string(output)))
	}
	return nil
}

// Remove removes a container
func (m *Manager) Remove(nameOrID string) error {
	cmd := exec.Command("podman", "rm", nameOrID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Join(ErrCommandFailed, errors.New(string(output)))
	}
	return nil
}

// Run creates and starts a container (shorthand)
func (m *Manager) Run(spec Spec) (string, error) {
	id, err := m.Create(spec)
	if err != nil {
		return "", err
	}

	if err := m.Start(id); err != nil {
		m.Remove(id)
		return "", err
	}

	return id, nil
}

// VolumeExists checks if a volume exists
func (m *Manager) VolumeExists(name string) (bool, error) {
	cmd := exec.Command("podman", "volume", "inspect", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "no such volume") {
			return false, nil
		}
		return false, errors.Join(ErrCommandFailed, errors.New(string(output)))
	}
	return true, nil
}

// VolumeCreate creates a new volume
func (m *Manager) VolumeCreate(name string) error {
	cmd := exec.Command("podman", "volume", "create", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Join(ErrCommandFailed, errors.New(string(output)))
	}
	return nil
}

// VolumeRemove removes a volume
func (m *Manager) VolumeRemove(name string) error {
	cmd := exec.Command("podman", "volume", "rm", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Join(ErrCommandFailed, errors.New(string(output)))
	}
	return nil
}
