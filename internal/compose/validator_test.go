package compose

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileValidator_ValidatePath(t *testing.T) {
	// Create a temp directory with a valid compose file
	tmpDir := t.TempDir()
	validFile := filepath.Join(tmpDir, "docker-compose.yml")
	err := os.WriteFile(validFile, []byte("services:\n  web:\n    image: nginx\n"), 0644)
	require.NoError(t, err)

	// Create a non-canonical path (with trailing slash) to test Clean behavior
	nonCanonicalPath := validFile + "/."

	tests := []struct {
		name      string
		path      string
		wantError error
	}{
		{
			name:      "valid absolute path",
			path:      validFile,
			wantError: nil,
		},
		{
			name:      "relative path",
			path:      "docker-compose.yml",
			wantError: ErrInvalidComposePath,
		},
		{
			name:      "path with traversal",
			path:      "/home/user/../other/docker-compose.yml",
			wantError: ErrInvalidComposePath,
		},
		{
			name:      "non-canonical path",
			path:      nonCanonicalPath,
			wantError: ErrInvalidComposePath,
		},
		{
			name:      "wrong extension txt",
			path:      filepath.Join(tmpDir, "compose.txt"),
			wantError: ErrInvalidComposePath,
		},
		{
			name:      "wrong extension json",
			path:      filepath.Join(tmpDir, "compose.json"),
			wantError: ErrInvalidComposePath,
		},
		{
			name:      "non-existent file",
			path:      filepath.Join(tmpDir, "nonexistent.yml"),
			wantError: ErrComposeFileNotFound,
		},
		{
			name:      "directory path",
			path:      tmpDir,
			wantError: ErrInvalidComposePath,
		},
	}

	validator := NewFileValidator(DefaultConfig())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.path)
			if tt.wantError != nil {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantError), "expected %v, got %v", tt.wantError, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFileValidator_PrivilegedBlocked(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	content := `services:
  web:
    image: nginx
    privileged: true
`
	err := os.WriteFile(composePath, []byte(content), 0644)
	require.NoError(t, err)

	validator := NewFileValidator(Config{
		AllowPrivileged:  false,
		AllowHostNetwork: false,
		AllowHostVolumes: false,
	})

	err = validator.Validate(composePath)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrPrivilegedNotAllowed))
	assert.Contains(t, err.Error(), "web")
}

func TestFileValidator_PrivilegedAllowed(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	content := `services:
  web:
    image: nginx
    privileged: true
`
	err := os.WriteFile(composePath, []byte(content), 0644)
	require.NoError(t, err)

	validator := NewFileValidator(Config{
		AllowPrivileged:  true,
		AllowHostNetwork: false,
		AllowHostVolumes: false,
	})

	err = validator.Validate(composePath)
	assert.NoError(t, err)
}

func TestFileValidator_DangerousCapabilities(t *testing.T) {
	tests := []struct {
		name      string
		capAdd    string
		wantError bool
	}{
		{
			name:      "cap_add ALL blocked",
			capAdd:    "ALL",
			wantError: true,
		},
		{
			name:      "cap_add SYS_ADMIN blocked",
			capAdd:    "SYS_ADMIN",
			wantError: true,
		},
		{
			name:      "cap_add NET_ADMIN allowed",
			capAdd:    "NET_ADMIN",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			composePath := filepath.Join(tmpDir, "docker-compose.yml")

			content := `services:
  web:
    image: nginx
    cap_add:
      - ` + tt.capAdd + `
`
			err := os.WriteFile(composePath, []byte(content), 0644)
			require.NoError(t, err)

			validator := NewFileValidator(Config{
				AllowPrivileged: false,
			})

			err = validator.Validate(composePath)
			if tt.wantError {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, ErrPrivilegedNotAllowed))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFileValidator_HostNetworkBlocked(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	content := `services:
  web:
    image: nginx
    network_mode: host
`
	err := os.WriteFile(composePath, []byte(content), 0644)
	require.NoError(t, err)

	validator := NewFileValidator(Config{
		AllowPrivileged:  false,
		AllowHostNetwork: false,
		AllowHostVolumes: false,
	})

	err = validator.Validate(composePath)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrHostNetworkNotAllowed))
}

func TestFileValidator_HostNetworkAllowed(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	content := `services:
  web:
    image: nginx
    network_mode: host
`
	err := os.WriteFile(composePath, []byte(content), 0644)
	require.NoError(t, err)

	validator := NewFileValidator(Config{
		AllowPrivileged:  false,
		AllowHostNetwork: true,
		AllowHostVolumes: false,
	})

	err = validator.Validate(composePath)
	assert.NoError(t, err)
}

func TestFileValidator_IPCHostBlocked(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	content := `services:
  web:
    image: nginx
    ipc: host
`
	err := os.WriteFile(composePath, []byte(content), 0644)
	require.NoError(t, err)

	validator := NewFileValidator(Config{AllowPrivileged: false})
	err = validator.Validate(composePath)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrPrivilegedNotAllowed))
	assert.Contains(t, err.Error(), "ipc: host")
}

func TestFileValidator_PIDHostBlocked(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	content := `services:
  web:
    image: nginx
    pid: host
`
	err := os.WriteFile(composePath, []byte(content), 0644)
	require.NoError(t, err)

	validator := NewFileValidator(Config{AllowPrivileged: false})
	err = validator.Validate(composePath)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrPrivilegedNotAllowed))
	assert.Contains(t, err.Error(), "pid: host")
}

func TestFileValidator_SecurityOptBlocked(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name: "seccomp unconfined colon",
			content: `services:
  web:
    image: nginx
    security_opt:
      - seccomp:unconfined
`,
		},
		{
			name: "seccomp unconfined equals",
			content: `services:
  web:
    image: nginx
    security_opt:
      - seccomp=unconfined
`,
		},
		{
			name: "apparmor unconfined",
			content: `services:
  web:
    image: nginx
    security_opt:
      - apparmor:unconfined
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			composePath := filepath.Join(tmpDir, "docker-compose.yml")

			err := os.WriteFile(composePath, []byte(tt.content), 0644)
			require.NoError(t, err)

			validator := NewFileValidator(Config{AllowPrivileged: false})
			err = validator.Validate(composePath)
			assert.Error(t, err)
			assert.True(t, errors.Is(err, ErrPrivilegedNotAllowed))
			assert.Contains(t, err.Error(), "security_opt")
		})
	}
}

func TestFileValidator_DevicesBlocked(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	content := `services:
  web:
    image: nginx
    devices:
      - /dev/sda:/dev/xvda
`
	err := os.WriteFile(composePath, []byte(content), 0644)
	require.NoError(t, err)

	validator := NewFileValidator(Config{AllowPrivileged: false})
	err = validator.Validate(composePath)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrPrivilegedNotAllowed))
	assert.Contains(t, err.Error(), "device")
}

func TestFileValidator_UserNSModeHostBlocked(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	content := `services:
  web:
    image: nginx
    userns_mode: host
`
	err := os.WriteFile(composePath, []byte(content), 0644)
	require.NoError(t, err)

	validator := NewFileValidator(Config{AllowPrivileged: false})
	err = validator.Validate(composePath)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrPrivilegedNotAllowed))
	assert.Contains(t, err.Error(), "userns_mode: host")
}

func TestFileValidator_SysctlsBlocked(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name: "kernel sysctl",
			content: `services:
  web:
    image: nginx
    sysctls:
      kernel.msgmax: 65536
`,
		},
		{
			name: "net sysctl",
			content: `services:
  web:
    image: nginx
    sysctls:
      net.core.somaxconn: 1024
`,
		},
		{
			name: "fs sysctl",
			content: `services:
  web:
    image: nginx
    sysctls:
      fs.file-max: 65536
`,
		},
		{
			name: "vm sysctl",
			content: `services:
  web:
    image: nginx
    sysctls:
      vm.swappiness: 60
`,
		},
		{
			name: "debug sysctl",
			content: `services:
  web:
    image: nginx
    sysctls:
      debug.exception-trace: 1
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			composePath := filepath.Join(tmpDir, "docker-compose.yml")

			err := os.WriteFile(composePath, []byte(tt.content), 0644)
			require.NoError(t, err)

			validator := NewFileValidator(Config{AllowPrivileged: false})
			err = validator.Validate(composePath)
			assert.Error(t, err)
			assert.True(t, errors.Is(err, ErrPrivilegedNotAllowed))
			assert.Contains(t, err.Error(), "sysctl")
		})
	}
}

func TestFileValidator_DangerousFieldsAllowedWhenPrivileged(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	// A compose file with many dangerous settings
	content := `services:
  web:
    image: nginx
    privileged: true
    ipc: host
    pid: host
    security_opt:
      - seccomp:unconfined
    devices:
      - /dev/sda:/dev/xvda
    sysctls:
      kernel.msgmax: 65536
`
	err := os.WriteFile(composePath, []byte(content), 0644)
	require.NoError(t, err)

	// Should pass when AllowPrivileged is true
	validator := NewFileValidator(Config{AllowPrivileged: true})
	err = validator.Validate(composePath)
	assert.NoError(t, err)
}

func TestFileValidator_HostVolumesBlocked(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name: "absolute path short syntax",
			content: `services:
  web:
    image: nginx
    volumes:
      - /host/path:/container/path
`,
		},
		{
			name: "relative path short syntax",
			content: `services:
  web:
    image: nginx
    volumes:
      - ./data:/data
`,
		},
		{
			name: "long syntax bind mount",
			content: `services:
  web:
    image: nginx
    volumes:
      - type: bind
        source: /host/path
        target: /container/path
`,
		},
		{
			name: "tilde expansion",
			content: `services:
  web:
    image: nginx
    volumes:
      - ~/data:/data
`,
		},
		{
			name: "environment variable",
			content: `services:
  web:
    image: nginx
    volumes:
      - $HOME/data:/data
`,
		},
		{
			name: "environment variable braces",
			content: `services:
  web:
    image: nginx
    volumes:
      - ${PWD}/data:/data
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			composePath := filepath.Join(tmpDir, "docker-compose.yml")

			err := os.WriteFile(composePath, []byte(tt.content), 0644)
			require.NoError(t, err)

			validator := NewFileValidator(Config{
				AllowPrivileged:  false,
				AllowHostNetwork: false,
				AllowHostVolumes: false,
			})

			err = validator.Validate(composePath)
			assert.Error(t, err)
			assert.True(t, errors.Is(err, ErrHostVolumeNotAllowed))
		})
	}
}

func TestFileValidator_HostVolumesAllowed(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	content := `services:
  web:
    image: nginx
    volumes:
      - /host/path:/container/path
`
	err := os.WriteFile(composePath, []byte(content), 0644)
	require.NoError(t, err)

	validator := NewFileValidator(Config{
		AllowPrivileged:  false,
		AllowHostNetwork: false,
		AllowHostVolumes: true,
	})

	err = validator.Validate(composePath)
	assert.NoError(t, err)
}

func TestFileValidator_NamedVolumesAllowed(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	// Named volumes (without leading / or .) should be allowed
	content := `services:
  db:
    image: postgres
    volumes:
      - pgdata:/var/lib/postgresql/data
volumes:
  pgdata:
`
	err := os.WriteFile(composePath, []byte(content), 0644)
	require.NoError(t, err)

	validator := NewFileValidator(Config{
		AllowPrivileged:  false,
		AllowHostNetwork: false,
		AllowHostVolumes: false,
	})

	err = validator.Validate(composePath)
	assert.NoError(t, err)
}

func TestFileValidator_ValidateWithService(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	content := `services:
  web:
    image: nginx
  api:
    image: node
  db:
    image: postgres
`
	err := os.WriteFile(composePath, []byte(content), 0644)
	require.NoError(t, err)

	validator := NewFileValidator(DefaultConfig())

	tests := []struct {
		name      string
		service   string
		wantError error
	}{
		{
			name:      "existing service web",
			service:   "web",
			wantError: nil,
		},
		{
			name:      "existing service api",
			service:   "api",
			wantError: nil,
		},
		{
			name:      "non-existing service",
			service:   "cache",
			wantError: ErrServiceNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateWithService(composePath, tt.service)
			if tt.wantError != nil {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantError))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFileValidator_GetServices(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	content := `services:
  web:
    image: nginx
  api:
    image: node
  db:
    image: postgres
`
	err := os.WriteFile(composePath, []byte(content), 0644)
	require.NoError(t, err)

	validator := NewFileValidator(DefaultConfig())
	services, err := validator.GetServices(composePath)
	require.NoError(t, err)

	assert.Len(t, services, 3)
	assert.Contains(t, services, "web")
	assert.Contains(t, services, "api")
	assert.Contains(t, services, "db")
}

func TestFileValidator_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	content := `this is not: valid: yaml: [[[`
	err := os.WriteFile(composePath, []byte(content), 0644)
	require.NoError(t, err)

	validator := NewFileValidator(DefaultConfig())
	err = validator.Validate(composePath)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidComposeFile))
}

func TestFileValidator_NoServices(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	content := `version: "3.8"
networks:
  default:
`
	err := os.WriteFile(composePath, []byte(content), 0644)
	require.NoError(t, err)

	validator := NewFileValidator(DefaultConfig())
	err = validator.Validate(composePath)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidComposeFile))
}

func TestFileValidator_ComplexValidCompose(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	// A complex but valid compose file
	content := `version: "3.8"
services:
  dev:
    build:
      context: .
      dockerfile: Dockerfile.dev
    image: myapp:dev
    working_dir: /app
    command: npm run dev
    environment:
      - NODE_ENV=development
    ports:
      - "3000:3000"
    volumes:
      - node_modules:/app/node_modules
    depends_on:
      - db
      - redis

  db:
    image: postgres:15
    environment:
      POSTGRES_PASSWORD: secret
    volumes:
      - pgdata:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    volumes:
      - redis_data:/data

volumes:
  node_modules:
  pgdata:
  redis_data:

networks:
  default:
    driver: bridge
`
	err := os.WriteFile(composePath, []byte(content), 0644)
	require.NoError(t, err)

	validator := NewFileValidator(DefaultConfig())
	err = validator.Validate(composePath)
	assert.NoError(t, err)

	// Validate specific service exists
	err = validator.ValidateWithService(composePath, "dev")
	assert.NoError(t, err)
}

func TestFileValidator_YamlExtension(t *testing.T) {
	tmpDir := t.TempDir()

	// Test .yaml extension
	yamlFile := filepath.Join(tmpDir, "compose.yaml")
	content := `services:
  web:
    image: nginx
`
	err := os.WriteFile(yamlFile, []byte(content), 0644)
	require.NoError(t, err)

	validator := NewFileValidator(DefaultConfig())
	err = validator.Validate(yamlFile)
	assert.NoError(t, err)
}

func TestFileValidator_FileSizeLimit(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	// Create a file that's too large (> 1MB)
	// We'll create a valid YAML header followed by lots of data
	largeContent := "services:\n  web:\n    image: nginx\n    labels:\n"
	// Add enough labels to exceed 1MB
	for i := 0; i < 100000; i++ {
		largeContent += "      - label" + string(rune(i)) + "=value\n"
	}

	err := os.WriteFile(composePath, []byte(largeContent), 0644)
	require.NoError(t, err)

	// Check file is actually large enough
	info, err := os.Stat(composePath)
	require.NoError(t, err)
	require.Greater(t, info.Size(), int64(maxComposeFileSize))

	validator := NewFileValidator(DefaultConfig())
	err = validator.Validate(composePath)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidComposeFile))
	assert.Contains(t, err.Error(), "file too large")
}

func TestFileValidator_SymlinkToValidFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create actual compose file
	realFile := filepath.Join(tmpDir, "real-compose.yml")
	content := `services:
  web:
    image: nginx
`
	err := os.WriteFile(realFile, []byte(content), 0644)
	require.NoError(t, err)

	// Create symlink to it
	symlinkPath := filepath.Join(tmpDir, "link-compose.yml")
	err = os.Symlink(realFile, symlinkPath)
	require.NoError(t, err)

	validator := NewFileValidator(DefaultConfig())
	err = validator.Validate(symlinkPath)
	assert.NoError(t, err)
}

func TestFileValidator_SymlinkToNonYaml(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a non-yaml file
	realFile := filepath.Join(tmpDir, "script.sh")
	content := `#!/bin/bash
echo "hello"
`
	err := os.WriteFile(realFile, []byte(content), 0644)
	require.NoError(t, err)

	// Create symlink with yaml extension pointing to non-yaml file
	symlinkPath := filepath.Join(tmpDir, "fake-compose.yml")
	err = os.Symlink(realFile, symlinkPath)
	require.NoError(t, err)

	validator := NewFileValidator(DefaultConfig())
	err = validator.Validate(symlinkPath)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidComposePath))
	assert.Contains(t, err.Error(), "symlink target must be .yml or .yaml")
}

func TestIsHostPath(t *testing.T) {
	tests := []struct {
		path   string
		isHost bool
	}{
		{"/var/lib/data", true},
		{"./data", true},
		{"../data", true},
		{"~/data", true},
		{"$HOME/data", true},
		{"${PWD}/data", true},
		// Windows paths - detected by second character being ':'
		// Note: In docker-compose, these are handled specially and would
		// be split differently, but our isHostPath function detects them
		{"C:", true},  // Just the drive letter
		{"D:", true},
		{"namedvolume", false},
		{"my_data", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isHostPath(tt.path)
			assert.Equal(t, tt.isHost, result, "isHostPath(%q)", tt.path)
		})
	}
}
