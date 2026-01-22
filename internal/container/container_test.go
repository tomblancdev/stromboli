//go:build integration

package container

import (
	"context"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func skipIfNoPodman(t *testing.T) {
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not available, skipping test")
	}
}

func TestCreate_SimpleContainer(t *testing.T) {
	skipIfNoPodman(t)
	ctx := context.Background()
	manager := NewManager()

	spec := Spec{
		Name:  "test-stromboli-create",
		Image: "docker.io/alpine:latest",
		Cmd:   []string{"echo", "hello"},
	}

	id, err := manager.Create(ctx, spec)
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	// Cleanup
	manager.Remove(ctx, spec.Name)
}

func TestRun_SimpleContainer(t *testing.T) {
	skipIfNoPodman(t)
	ctx := context.Background()
	manager := NewManager()

	spec := Spec{
		Name:  "test-stromboli-run",
		Image: "docker.io/alpine:latest",
		Cmd:   []string{"echo", "hello"},
	}

	id, err := manager.Run(ctx, spec)
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	// Cleanup
	manager.Stop(ctx, spec.Name)
	manager.Remove(ctx, spec.Name)
}

func TestStop_RunningContainer(t *testing.T) {
	skipIfNoPodman(t)
	ctx := context.Background()
	manager := NewManager()

	spec := Spec{
		Name:  "test-stromboli-stop",
		Image: "docker.io/alpine:latest",
		Cmd:   []string{"sleep", "60"},
	}

	_, err := manager.Run(ctx, spec)
	require.NoError(t, err)

	err = manager.Stop(ctx, spec.Name)
	assert.NoError(t, err)

	// Cleanup
	manager.Remove(ctx, spec.Name)
}

func TestRemove_StoppedContainer(t *testing.T) {
	skipIfNoPodman(t)
	ctx := context.Background()
	manager := NewManager()

	spec := Spec{
		Name:  "test-stromboli-remove",
		Image: "docker.io/alpine:latest",
		Cmd:   []string{"echo", "hello"},
	}

	manager.Create(ctx, spec)
	err := manager.Remove(ctx, spec.Name)
	assert.NoError(t, err)
}

func TestCreate_ContextCancellation(t *testing.T) {
	skipIfNoPodman(t)
	manager := NewManager()

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	spec := Spec{
		Name:  "test-stromboli-cancel",
		Image: "docker.io/alpine:latest",
		Cmd:   []string{"sleep", "60"},
	}

	_, err := manager.Create(ctx, spec)
	assert.Error(t, err, "expected error with cancelled context")
}
