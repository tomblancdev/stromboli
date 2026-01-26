package runner

import (
	"fmt"
	"regexp"
)

// Environment variable name validation
// Must start with letter or underscore, contain only alphanumeric and underscore
var envVarNameRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// Maximum number of secrets that can be injected (prevent DoS)
const maxSecretsEnv = 50

// Dangerous environment variables that could affect container security
var dangerousEnvVars = map[string]bool{
	"LD_PRELOAD":      true,
	"LD_LIBRARY_PATH": true,
}

// ValidateSecretsEnv validates the secrets_env map for security and correctness
func ValidateSecretsEnv(secretsEnv map[string]string) error {
	if len(secretsEnv) > maxSecretsEnv {
		return fmt.Errorf("too many secrets requested (%d), maximum is %d", len(secretsEnv), maxSecretsEnv)
	}

	for envVar, secretName := range secretsEnv {
		// Validate environment variable name format
		if !envVarNameRegex.MatchString(envVar) {
			return fmt.Errorf("invalid environment variable name %q: must start with letter or underscore, contain only alphanumeric and underscore", envVar)
		}

		// Check for dangerous environment variables
		if dangerousEnvVars[envVar] {
			return fmt.Errorf("environment variable %q is not allowed for security reasons", envVar)
		}

		// Validate secret name is not empty
		if secretName == "" {
			return fmt.Errorf("secret name for environment variable %q cannot be empty", envVar)
		}

		// Validate secret name length (Podman limit)
		if len(secretName) > 253 {
			return fmt.Errorf("secret name %q exceeds maximum length of 253 characters", secretName)
		}
	}

	return nil
}
