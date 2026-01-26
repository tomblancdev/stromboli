package version

// Build-time variables (set via ldflags)
var (
	// Version is the semantic version (e.g., "0.1.4")
	Version = "dev"

	// Commit is the git commit SHA
	Commit = "unknown"

	// BuildTime is the build timestamp
	BuildTime = "unknown"
)

// Info returns version information
type Info struct {
	Version   string `json:"version" example:"0.1.4"`
	Commit    string `json:"commit" example:"ee4ca0c"`
	BuildTime string `json:"build_time,omitempty" example:"2026-01-26T12:00:00Z"`
}

// Get returns the current version info
func Get() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildTime: BuildTime,
	}
}

// String returns a formatted version string
func String() string {
	if Commit == "unknown" || Commit == "" {
		return Version
	}
	return Version + " (" + Commit[:min(7, len(Commit))] + ")"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
