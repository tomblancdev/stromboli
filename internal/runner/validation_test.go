package runner

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateSecretsEnv_ValidCases(t *testing.T) {
	tests := []struct {
		name       string
		secretsEnv map[string]string
	}{
		{
			name:       "empty map",
			secretsEnv: map[string]string{},
		},
		{
			name:       "single valid secret",
			secretsEnv: map[string]string{"GH_TOKEN": "github-token"},
		},
		{
			name:       "multiple valid secrets",
			secretsEnv: map[string]string{"GH_TOKEN": "github-token", "GITLAB_TOKEN": "gitlab-token"},
		},
		{
			name:       "underscore prefix",
			secretsEnv: map[string]string{"_PRIVATE": "secret"},
		},
		{
			name:       "numbers in name",
			secretsEnv: map[string]string{"API_KEY_V2": "key-v2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSecretsEnv(tt.secretsEnv)
			assert.NoError(t, err)
		})
	}
}

func TestValidateSecretsEnv_InvalidEnvVarNames(t *testing.T) {
	tests := []struct {
		name       string
		secretsEnv map[string]string
		errContain string
	}{
		{
			name:       "starts with number",
			secretsEnv: map[string]string{"1PASSWORD": "secret"},
			errContain: "invalid environment variable name",
		},
		{
			name:       "contains hyphen",
			secretsEnv: map[string]string{"GH-TOKEN": "secret"},
			errContain: "invalid environment variable name",
		},
		{
			name:       "contains space",
			secretsEnv: map[string]string{"GH TOKEN": "secret"},
			errContain: "invalid environment variable name",
		},
		{
			name:       "empty env var name",
			secretsEnv: map[string]string{"": "secret"},
			errContain: "invalid environment variable name",
		},
		{
			name:       "contains special characters",
			secretsEnv: map[string]string{"GH$TOKEN": "secret"},
			errContain: "invalid environment variable name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSecretsEnv(tt.secretsEnv)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContain)
		})
	}
}

func TestValidateSecretsEnv_DangerousEnvVars(t *testing.T) {
	tests := []struct {
		name   string
		envVar string
	}{
		{"LD_PRELOAD", "LD_PRELOAD"},
		{"LD_LIBRARY_PATH", "LD_LIBRARY_PATH"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSecretsEnv(map[string]string{tt.envVar: "malicious"})
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "not allowed for security reasons")
		})
	}
}

func TestValidateSecretsEnv_EmptySecretName(t *testing.T) {
	err := ValidateSecretsEnv(map[string]string{"GH_TOKEN": ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestValidateSecretsEnv_TooManySecrets(t *testing.T) {
	secretsEnv := make(map[string]string)
	for i := 0; i <= maxSecretsEnv; i++ {
		secretsEnv["SECRET_"+strings.Repeat("X", 5)+string(rune('A'+i%26))+string(rune('0'+i/26))] = "secret"
	}

	err := ValidateSecretsEnv(secretsEnv)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too many secrets")
}

func TestValidateSecretsEnv_SecretNameTooLong(t *testing.T) {
	longName := strings.Repeat("a", 254)
	err := ValidateSecretsEnv(map[string]string{"GH_TOKEN": longName})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum length")
}
