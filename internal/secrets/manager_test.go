package secrets

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewManager(t *testing.T) {
	m := NewManager("")
	assert.Equal(t, DefaultSecretName, m.SecretName())

	m = NewManager("custom-secret")
	assert.Equal(t, "custom-secret", m.SecretName())
}
