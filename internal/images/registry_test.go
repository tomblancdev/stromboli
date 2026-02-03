package images

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockExecutor implements a simple executor for testing
type MockExecutor struct {
	RunFunc func(ctx context.Context, args []string) ([]byte, error)
	calls   [][]string
}

func (m *MockExecutor) Run(ctx context.Context, args []string) ([]byte, error) {
	m.calls = append(m.calls, args)
	if m.RunFunc != nil {
		return m.RunFunc(ctx, args)
	}
	return nil, nil
}

func (m *MockExecutor) GetCalls() [][]string {
	return m.calls
}

func (m *MockExecutor) Reset() {
	m.calls = nil
}

func TestNewRegistry(t *testing.T) {
	exec := &MockExecutor{}
	r := NewRegistry(exec)

	assert.NotNil(t, r)
	assert.Equal(t, exec, r.executor)
}

func TestRegistry_List_Success(t *testing.T) {
	exec := &MockExecutor{
		RunFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return []byte(`{"Id":"sha256:abc123","Repository":"python","Tag":"3.12-slim","Size":125000000,"Created":1704067200}
{"Id":"sha256:def456","Repository":"node","Tag":"20-bookworm","Size":180000000,"Created":1704153600}
{"Id":"sha256:ghi789","Repository":"alpine","Tag":"3.19","Size":7000000,"Created":1704240000}`), nil
		},
	}
	r := NewRegistry(exec)

	images, err := r.List(context.Background())

	require.NoError(t, err)
	require.Len(t, images, 3)

	// Images should be sorted by compatibility rank
	// Python and node should come before alpine (incompatible)
	assert.Equal(t, RankGlibcBased, images[0].CompatibilityRank)
	assert.Equal(t, RankGlibcBased, images[1].CompatibilityRank)
	assert.Equal(t, RankIncompatible, images[2].CompatibilityRank)

	// Should call: podman images --format "{{json .}}"
	call := exec.GetCalls()[0]
	assert.Equal(t, []string{"podman", "images", "--format", "{{json .}}"}, call)
}

func TestRegistry_List_SkipsDanglingImages(t *testing.T) {
	exec := &MockExecutor{
		RunFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return []byte(`{"Id":"sha256:abc123","Repository":"python","Tag":"3.12","Size":125000000,"Created":1704067200}
{"Id":"sha256:dangling","Repository":"<none>","Tag":"<none>","Size":50000000,"Created":1704000000}`), nil
		},
	}
	r := NewRegistry(exec)

	images, err := r.List(context.Background())

	require.NoError(t, err)
	require.Len(t, images, 1)
	assert.Equal(t, "python", images[0].Repository)
}

func TestRegistry_List_Empty(t *testing.T) {
	exec := &MockExecutor{
		RunFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return []byte(""), nil
		},
	}
	r := NewRegistry(exec)

	images, err := r.List(context.Background())

	require.NoError(t, err)
	assert.Empty(t, images)
}

func TestRegistry_List_PodmanError(t *testing.T) {
	exec := &MockExecutor{
		RunFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return nil, errors.New("podman not running")
		},
	}
	r := NewRegistry(exec)

	_, err := r.List(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list images")
}

func TestRegistry_Inspect_Success(t *testing.T) {
	exec := &MockExecutor{
		RunFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return []byte(`[{
				"Id": "sha256:abc123",
				"Created": "2024-01-01T12:00:00.000000000Z",
				"Size": 125000000,
				"RepoTags": ["python:3.12-slim"],
				"Config": {
					"Labels": {
						"ai.stromboli.compatible": "true",
						"ai.stromboli.tools": "python, pip",
						"ai.stromboli.description": "Python development image"
					}
				}
			}]`), nil
		},
	}
	r := NewRegistry(exec)

	info, err := r.Inspect(context.Background(), "python:3.12-slim")

	require.NoError(t, err)
	assert.Equal(t, "sha256:abc123", info.ID)
	assert.Equal(t, "python", info.Repository)
	assert.Equal(t, "3.12-slim", info.Tag)
	assert.True(t, info.Compatible)
	assert.Equal(t, RankStromboliCompatible, info.CompatibilityRank)
	assert.Equal(t, []string{"python", "pip"}, info.Tools)
	assert.Equal(t, "Python development image", info.Description)

	// Should call: podman inspect python:3.12-slim --type image
	call := exec.GetCalls()[0]
	assert.Equal(t, []string{"podman", "inspect", "python:3.12-slim", "--type", "image"}, call)
}

func TestRegistry_Inspect_WithClaudeCLI(t *testing.T) {
	exec := &MockExecutor{
		RunFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return []byte(`[{
				"Id": "sha256:abc123",
				"Created": "2024-01-01T12:00:00.000000000Z",
				"Size": 250000000,
				"RepoTags": ["stromboli-agent:latest"],
				"Config": {
					"Labels": {
						"ai.stromboli.claude-cli": "true"
					}
				}
			}]`), nil
		},
	}
	r := NewRegistry(exec)

	info, err := r.Inspect(context.Background(), "stromboli-agent:latest")

	require.NoError(t, err)
	assert.True(t, info.HasClaudeCLI)
	assert.Equal(t, RankClaudeCLI, info.CompatibilityRank)
}

func TestRegistry_Inspect_EmptyName(t *testing.T) {
	exec := &MockExecutor{}
	r := NewRegistry(exec)

	_, err := r.Inspect(context.Background(), "")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidation)
	assert.Contains(t, err.Error(), "image name cannot be empty")
}

func TestRegistry_Inspect_NotFound(t *testing.T) {
	exec := &MockExecutor{
		RunFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return []byte("Error: image not known"), errors.New("exit status 125")
		},
	}
	r := NewRegistry(exec)

	_, err := r.Inspect(context.Background(), "nonexistent:latest")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrImageNotFound)
}

func TestRegistry_Search_Success(t *testing.T) {
	exec := &MockExecutor{
		RunFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return []byte(`[
				{"Index":"docker.io","Name":"python","Description":"Python is an interpreted programming language","Stars":8500,"Official":"[OK]","Automated":""},
				{"Index":"docker.io","Name":"circleci/python","Description":"Python builds","Stars":100,"Official":"","Automated":"[OK]"}
			]`), nil
		},
	}
	r := NewRegistry(exec)

	results, err := r.Search(context.Background(), "python", nil)

	require.NoError(t, err)
	require.Len(t, results, 2)

	assert.Equal(t, "docker.io", results[0].Index)
	assert.Equal(t, "python", results[0].Name)
	assert.Equal(t, 8500, results[0].Stars)
	assert.True(t, results[0].Official)
	assert.False(t, results[0].Automated)

	assert.Equal(t, "circleci/python", results[1].Name)
	assert.False(t, results[1].Official)
	assert.True(t, results[1].Automated)

	// Should call: podman search docker.io/python --format json --limit 25
	call := exec.GetCalls()[0]
	assert.Equal(t, []string{"podman", "search", "docker.io/python", "--format", "json", "--limit", "25"}, call)
}

func TestRegistry_Search_WithOptions(t *testing.T) {
	exec := &MockExecutor{
		RunFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return []byte(`[]`), nil
		},
	}
	r := NewRegistry(exec)

	_, err := r.Search(context.Background(), "node", &SearchOptions{
		Limit:   10,
		NoTrunc: true,
	})

	require.NoError(t, err)

	call := exec.GetCalls()[0]
	assert.Contains(t, call, "--limit")
	assert.Contains(t, call, "10")
	assert.Contains(t, call, "--no-trunc")
}

func TestRegistry_Search_LimitCapped(t *testing.T) {
	exec := &MockExecutor{
		RunFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return []byte(`[]`), nil
		},
	}
	r := NewRegistry(exec)

	_, err := r.Search(context.Background(), "test", &SearchOptions{
		Limit: 500, // Exceeds max
	})

	require.NoError(t, err)

	call := exec.GetCalls()[0]
	// Should be capped to MaxSearchLimit (100)
	assert.Contains(t, call, "100")
}

func TestRegistry_Search_EmptyQuery(t *testing.T) {
	exec := &MockExecutor{}
	r := NewRegistry(exec)

	_, err := r.Search(context.Background(), "", nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidation)
	assert.Contains(t, err.Error(), "search query cannot be empty")
}

func TestRegistry_Search_Empty(t *testing.T) {
	exec := &MockExecutor{
		RunFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return []byte(`[]`), nil
		},
	}
	r := NewRegistry(exec)

	results, err := r.Search(context.Background(), "nonexistent12345", nil)

	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestRegistry_Search_Error(t *testing.T) {
	exec := &MockExecutor{
		RunFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return nil, errors.New("network error")
		},
	}
	r := NewRegistry(exec)

	_, err := r.Search(context.Background(), "python", nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSearchFailed)
}

func TestRegistry_Pull_Success(t *testing.T) {
	exec := &MockExecutor{
		RunFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return []byte(`Trying to pull docker.io/library/python:3.12-slim...
Getting image source signatures
Copying blob sha256:abc123 done
Copying config sha256:def456 done
Writing manifest to image destination
sha256:final789`), nil
		},
	}
	r := NewRegistry(exec)

	imageID, err := r.Pull(context.Background(), "python:3.12-slim", nil)

	require.NoError(t, err)
	assert.Equal(t, "sha256:final789", imageID)

	// Should call: podman pull python:3.12-slim
	call := exec.GetCalls()[0]
	assert.Equal(t, []string{"podman", "pull", "python:3.12-slim"}, call)
}

func TestRegistry_Pull_WithOptions(t *testing.T) {
	exec := &MockExecutor{
		RunFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return []byte(`sha256:abc123`), nil
		},
	}
	r := NewRegistry(exec)

	_, err := r.Pull(context.Background(), "python:3.12", &PullOptions{
		Quiet:    true,
		Platform: "linux/amd64",
	})

	require.NoError(t, err)

	call := exec.GetCalls()[0]
	assert.Contains(t, call, "--quiet")
	assert.Contains(t, call, "--platform")
	assert.Contains(t, call, "linux/amd64")
}

func TestRegistry_Pull_EmptyName(t *testing.T) {
	exec := &MockExecutor{}
	r := NewRegistry(exec)

	_, err := r.Pull(context.Background(), "", nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidation)
	assert.Contains(t, err.Error(), "image name cannot be empty")
}

func TestRegistry_Pull_Error(t *testing.T) {
	exec := &MockExecutor{
		RunFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return []byte("Error: unable to pull image"), errors.New("exit status 125")
		},
	}
	r := NewRegistry(exec)

	_, err := r.Pull(context.Background(), "nonexistent/image:latest", nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPullFailed)
}

func TestNormalizeSearchQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{"simple query", "python", "docker.io/python"},
		{"query with namespace", "library/python", "docker.io/library/python"},
		{"already has docker.io", "docker.io/python", "docker.io/python"},
		{"already has ghcr.io", "ghcr.io/owner/repo", "ghcr.io/owner/repo"},
		{"already has quay.io", "quay.io/org/image", "quay.io/org/image"},
		{"localhost registry", "localhost/myimage", "localhost/myimage"},
		{"localhost with port", "localhost:5000/myimage", "docker.io/localhost:5000/myimage"}, // Port without dot, treated as namespace
		{"custom registry", "registry.example.com/image", "registry.example.com/image"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeSearchQuery(tt.query)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseRepoTag(t *testing.T) {
	tests := []struct {
		name         string
		repoTags     []string
		fallback     string
		wantRepo     string
		wantTag      string
	}{
		{
			name:     "single repo tag",
			repoTags: []string{"python:3.12-slim"},
			fallback: "fallback",
			wantRepo: "python",
			wantTag:  "3.12-slim",
		},
		{
			name:     "repo tag with registry",
			repoTags: []string{"docker.io/library/python:3.12"},
			fallback: "fallback",
			wantRepo: "docker.io/library/python",
			wantTag:  "3.12",
		},
		{
			name:     "multiple repo tags - uses first",
			repoTags: []string{"python:3.12", "python:latest"},
			fallback: "fallback",
			wantRepo: "python",
			wantTag:  "3.12",
		},
		{
			name:     "empty repo tags - uses fallback",
			repoTags: []string{},
			fallback: "myimage:v1.0",
			wantRepo: "myimage",
			wantTag:  "v1.0",
		},
		{
			name:     "fallback without tag",
			repoTags: []string{},
			fallback: "myimage",
			wantRepo: "myimage",
			wantTag:  "latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, tag := parseRepoTag(tt.repoTags, tt.fallback)
			assert.Equal(t, tt.wantRepo, repo)
			assert.Equal(t, tt.wantTag, tag)
		})
	}
}
