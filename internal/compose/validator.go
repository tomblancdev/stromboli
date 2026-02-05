package compose

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Validation errors
var (
	ErrComposeFileNotFound    = fmt.Errorf("compose file not found")
	ErrInvalidComposePath     = fmt.Errorf("invalid compose file path")
	ErrPrivilegedNotAllowed   = fmt.Errorf("privileged containers not allowed")
	ErrHostNetworkNotAllowed  = fmt.Errorf("host network mode not allowed")
	ErrHostVolumeNotAllowed   = fmt.Errorf("host volume mounts not allowed")
	ErrServiceNotFound        = fmt.Errorf("service not found in compose file")
	ErrInvalidComposeFile     = fmt.Errorf("invalid compose file")
)

// FileValidator validates compose files for security compliance
type FileValidator struct {
	config Config
}

// NewFileValidator creates a new FileValidator with the given config
func NewFileValidator(cfg Config) *FileValidator {
	return &FileValidator{config: cfg}
}

// Validate checks if a compose file is valid and compliant with security settings.
// It returns nil if the file is valid, or an error describing the issue.
func (v *FileValidator) Validate(path string) error {
	// Validate path
	if err := v.validatePath(path); err != nil {
		return err
	}

	// Parse and validate compose file contents
	if err := v.validateContents(path); err != nil {
		return err
	}

	return nil
}

// ValidateWithService validates a compose file and checks that the specified service exists
func (v *FileValidator) ValidateWithService(path, service string) error {
	// First validate the file itself
	if err := v.Validate(path); err != nil {
		return err
	}

	// Then check the service exists
	compose, err := v.parseComposeFile(path)
	if err != nil {
		return err
	}

	if _, ok := compose.Services[service]; !ok {
		return fmt.Errorf("%w: %s", ErrServiceNotFound, service)
	}

	return nil
}

// validatePath checks that the path is valid and secure
func (v *FileValidator) validatePath(path string) error {
	// Must be absolute path
	if !filepath.IsAbs(path) {
		return fmt.Errorf("%w: path must be absolute", ErrInvalidComposePath)
	}

	// Clean the path and verify it equals the original (catches traversal attempts)
	cleanPath := filepath.Clean(path)
	if cleanPath != path {
		return fmt.Errorf("%w: path contains traversal sequence or is not canonical", ErrInvalidComposePath)
	}

	// Must be a YAML file
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".yml" && ext != ".yaml" {
		return fmt.Errorf("%w: must be .yml or .yaml file", ErrInvalidComposePath)
	}

	// Check file exists first (before resolving symlinks)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("%w: %s", ErrComposeFileNotFound, path)
	}
	if err != nil {
		return fmt.Errorf("%w: cannot access file: %v", ErrInvalidComposePath, err)
	}

	// Must be a regular file (not a directory)
	if info.IsDir() {
		return fmt.Errorf("%w: path is a directory", ErrInvalidComposePath)
	}

	// Resolve symlinks and verify the real path doesn't escape
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("%w: cannot resolve symlinks: %v", ErrInvalidComposePath, err)
	}

	// Verify the resolved path is still a yaml file (symlink attack prevention)
	realExt := strings.ToLower(filepath.Ext(realPath))
	if realExt != ".yml" && realExt != ".yaml" {
		return fmt.Errorf("%w: symlink target must be .yml or .yaml file", ErrInvalidComposePath)
	}

	// Re-check the resolved path is a regular file
	realInfo, err := os.Stat(realPath)
	if err != nil {
		return fmt.Errorf("%w: cannot access resolved path: %v", ErrInvalidComposePath, err)
	}
	if realInfo.IsDir() {
		return fmt.Errorf("%w: resolved path is a directory", ErrInvalidComposePath)
	}

	return nil
}

// validateContents parses and validates the compose file contents for security
func (v *FileValidator) validateContents(path string) error {
	compose, err := v.parseComposeFile(path)
	if err != nil {
		return err
	}

	// Validate each service
	for name, service := range compose.Services {
		if err := v.validateService(name, service); err != nil {
			return err
		}
	}

	return nil
}

// composeFile represents the structure of a compose file for validation
type composeFile struct {
	Version  string                    `yaml:"version"`
	Services map[string]composeService `yaml:"services"`
}

// composeService represents a service in a compose file
type composeService struct {
	Image       string        `yaml:"image"`
	Build       interface{}   `yaml:"build"` // Can be string or map
	Privileged  bool          `yaml:"privileged"`
	NetworkMode string        `yaml:"network_mode"`
	Volumes     []interface{} `yaml:"volumes"` // Can be strings or maps
	CapAdd      []string      `yaml:"cap_add"`
}

// maxComposeFileSize is the maximum allowed size for compose files (1MB)
const maxComposeFileSize = 1 * 1024 * 1024

// parseComposeFile reads and parses a compose file
func (v *FileValidator) parseComposeFile(path string) (*composeFile, error) {
	// Check file size first to prevent DoS via large files
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("%w: cannot stat file: %v", ErrInvalidComposeFile, err)
	}
	if info.Size() > maxComposeFileSize {
		return nil, fmt.Errorf("%w: file too large (max %d bytes, got %d)", ErrInvalidComposeFile, maxComposeFileSize, info.Size())
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: cannot read file: %v", ErrInvalidComposeFile, err)
	}

	var compose composeFile
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return nil, fmt.Errorf("%w: cannot parse YAML: %v", ErrInvalidComposeFile, err)
	}

	if compose.Services == nil {
		return nil, fmt.Errorf("%w: no services defined", ErrInvalidComposeFile)
	}

	return &compose, nil
}

// validateService validates a single service configuration
func (v *FileValidator) validateService(name string, service composeService) error {
	// Check privileged mode
	if service.Privileged && !v.config.AllowPrivileged {
		return fmt.Errorf("%w: service %q has privileged: true", ErrPrivilegedNotAllowed, name)
	}

	// Check for privileged-equivalent capabilities
	if !v.config.AllowPrivileged {
		for _, cap := range service.CapAdd {
			if cap == "ALL" || cap == "SYS_ADMIN" {
				return fmt.Errorf("%w: service %q has dangerous capability: %s", ErrPrivilegedNotAllowed, name, cap)
			}
		}
	}

	// Check host network mode
	if service.NetworkMode == "host" && !v.config.AllowHostNetwork {
		return fmt.Errorf("%w: service %q has network_mode: host", ErrHostNetworkNotAllowed, name)
	}

	// Check host volumes
	if !v.config.AllowHostVolumes {
		for _, vol := range service.Volumes {
			if v.isHostVolume(vol) {
				return fmt.Errorf("%w: service %q has host volume mount", ErrHostVolumeNotAllowed, name)
			}
		}
	}

	return nil
}

// isHostVolume checks if a volume specification is a host mount
func (v *FileValidator) isHostVolume(vol interface{}) bool {
	switch volume := vol.(type) {
	case string:
		// Short syntax: "host:container" or "host:container:mode"
		// Named volumes don't have "/" in the source
		parts := strings.Split(volume, ":")
		if len(parts) >= 2 {
			source := parts[0]
			if isHostPath(source) {
				return true
			}
		}
	case map[string]interface{}:
		// Long syntax with type field
		volType, ok := volume["type"].(string)
		if ok && volType == "bind" {
			return true
		}
		// Also check source for host paths
		if source, ok := volume["source"].(string); ok {
			if isHostPath(source) {
				return true
			}
		}
	}
	return false
}

// isHostPath checks if a path is a host filesystem path
func isHostPath(source string) bool {
	// Unix absolute paths
	if strings.HasPrefix(source, "/") {
		return true
	}
	// Relative paths (current dir or parent dir)
	if strings.HasPrefix(source, ".") {
		return true
	}
	// Tilde expansion (home directory)
	if strings.HasPrefix(source, "~") {
		return true
	}
	// Environment variables that likely resolve to paths
	if strings.HasPrefix(source, "$") || strings.Contains(source, "${") {
		return true
	}
	// Windows absolute paths (C:\, D:\, etc.)
	if len(source) >= 2 && source[1] == ':' {
		return true
	}
	return false
}

// GetServices returns the list of service names from a compose file
func (v *FileValidator) GetServices(path string) ([]string, error) {
	compose, err := v.parseComposeFile(path)
	if err != nil {
		return nil, err
	}

	services := make([]string, 0, len(compose.Services))
	for name := range compose.Services {
		services = append(services, name)
	}
	return services, nil
}
