package runner

import (
	"os/exec"
	"testing"
)

// skipIfNoPodman skips the test if podman is not available
func skipIfNoPodman(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not available, skipping test")
	}
}
