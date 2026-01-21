package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	manager := NewManager()
	assert.NotNil(t, manager)
}

func TestCreate_SimpleContainer(t *testing.T) {
	manager := NewManager()

	spec := Spec{
		Name:  "test-stromboli-create",
		Image: "docker.io/alpine:latest",
		Cmd:   []string{"echo", "hello"},
	}

	id, err := manager.Create(spec)
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	// Cleanup
	manager.Remove(spec.Name)
}

func TestRun_SimpleContainer(t *testing.T) {
	manager := NewManager()

	spec := Spec{
		Name:  "test-stromboli-run",
		Image: "docker.io/alpine:latest",
		Cmd:   []string{"echo", "hello"},
	}

	id, err := manager.Run(spec)
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	// Cleanup
	manager.Stop(spec.Name)
	manager.Remove(spec.Name)
}

func TestStop_RunningContainer(t *testing.T) {
	manager := NewManager()

	spec := Spec{
		Name:  "test-stromboli-stop",
		Image: "docker.io/alpine:latest",
		Cmd:   []string{"sleep", "60"},
	}

	_, err := manager.Run(spec)
	require.NoError(t, err)

	err = manager.Stop(spec.Name)
	assert.NoError(t, err)

	// Cleanup
	manager.Remove(spec.Name)
}

func TestRemove_StoppedContainer(t *testing.T) {
	manager := NewManager()

	spec := Spec{
		Name:  "test-stromboli-remove",
		Image: "docker.io/alpine:latest",
		Cmd:   []string{"echo", "hello"},
	}

	manager.Create(spec)
	err := manager.Remove(spec.Name)
	assert.NoError(t, err)
}
