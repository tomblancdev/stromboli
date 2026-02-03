package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"stromboli/internal/images"
)

// mockImagesExecutor implements images.Executor for testing
type mockImagesExecutor struct {
	runFunc func(ctx context.Context, args []string) ([]byte, error)
}

func (m *mockImagesExecutor) Run(ctx context.Context, args []string) ([]byte, error) {
	if m.runFunc != nil {
		return m.runFunc(ctx, args)
	}
	return nil, nil
}

func setupImagesRouter(registry *images.Registry) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewImagesHandler(registry)

	router.GET("/images", handler.List)
	router.GET("/images/search", handler.Search)
	router.GET("/images/:name", handler.Inspect)
	router.POST("/images/pull", handler.Pull)

	return router
}

func TestNewImagesHandler(t *testing.T) {
	exec := &mockImagesExecutor{}
	registry := images.NewRegistry(exec)
	handler := NewImagesHandler(registry)

	// Just verify handler is created (functional correctness proven by other tests)
	assert.NotNil(t, handler)
}

func TestImagesHandler_List_Success(t *testing.T) {
	exec := &mockImagesExecutor{
		runFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return []byte(`{"Id":"sha256:abc123","Repository":"python","Tag":"3.12-slim","Size":125000000,"Created":1704067200}
{"Id":"sha256:def456","Repository":"node","Tag":"20-bookworm","Size":180000000,"Created":1704153600}`), nil
		},
	}
	registry := images.NewRegistry(exec)
	router := setupImagesRouter(registry)

	req, _ := http.NewRequest("GET", "/images", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response ImagesListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Len(t, response.Images, 2)
	assert.Equal(t, "node", response.Images[0].Repository)    // Alphabetically first
	assert.Equal(t, "python", response.Images[1].Repository)  // Alphabetically second
}

func TestImagesHandler_List_Empty(t *testing.T) {
	exec := &mockImagesExecutor{
		runFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return []byte(``), nil
		},
	}
	registry := images.NewRegistry(exec)
	router := setupImagesRouter(registry)

	req, _ := http.NewRequest("GET", "/images", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response ImagesListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Empty(t, response.Images)
}

func TestImagesHandler_List_Error(t *testing.T) {
	exec := &mockImagesExecutor{
		runFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return nil, errors.New("podman not running")
		},
	}
	registry := images.NewRegistry(exec)
	router := setupImagesRouter(registry)

	req, _ := http.NewRequest("GET", "/images", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response.Error, "failed to list images")
}

func TestImagesHandler_Inspect_Success(t *testing.T) {
	exec := &mockImagesExecutor{
		runFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return []byte(`[{
				"Id": "sha256:abc123",
				"Created": "2024-01-01T12:00:00.000000000Z",
				"Size": 125000000,
				"RepoTags": ["python:3.12-slim"],
				"Config": {
					"Labels": {
						"ai.stromboli.compatible": "true",
						"ai.stromboli.tools": "python, pip"
					}
				}
			}]`), nil
		},
	}
	registry := images.NewRegistry(exec)
	router := setupImagesRouter(registry)

	req, _ := http.NewRequest("GET", "/images/python:3.12-slim", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response ImageDetailResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "sha256:abc123", response.ID)
	assert.Equal(t, "python", response.Repository)
	assert.Equal(t, "3.12-slim", response.Tag)
	assert.True(t, response.Compatible)
	assert.Equal(t, images.RankStromboliCompatible, response.CompatibilityRank)
	assert.Contains(t, response.RankDescription, "Stromboli verified")
}

func TestImagesHandler_Inspect_NotFound(t *testing.T) {
	exec := &mockImagesExecutor{
		runFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return []byte("Error: image not known"), errors.New("exit status 125")
		},
	}
	registry := images.NewRegistry(exec)
	router := setupImagesRouter(registry)

	req, _ := http.NewRequest("GET", "/images/nonexistent:latest", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "image not found", response.Error)
}

func TestImagesHandler_Inspect_InvalidName(t *testing.T) {
	exec := &mockImagesExecutor{}
	registry := images.NewRegistry(exec)
	router := setupImagesRouter(registry)

	tests := []struct {
		name      string
		imageName string
	}{
		{"special chars", "invalid!image"},
		{"consecutive colons", "image::tag"},
		{"starts with dash", "-invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/images/"+tt.imageName, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var response ErrorResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response.Error, "invalid image name")
		})
	}
}

func TestImagesHandler_Search_Success(t *testing.T) {
	exec := &mockImagesExecutor{
		runFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return []byte(`[
				{"Index":"docker.io","Name":"python","Description":"Python is an interpreted programming language","Stars":8500,"Official":"[OK]","Automated":""},
				{"Index":"docker.io","Name":"circleci/python","Description":"Python builds","Stars":100,"Official":"","Automated":"[OK]"}
			]`), nil
		},
	}
	registry := images.NewRegistry(exec)
	router := setupImagesRouter(registry)

	req, _ := http.NewRequest("GET", "/images/search?q=python", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response ImageSearchResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Len(t, response.Results, 2)
	assert.Equal(t, "python", response.Results[0].Name)
	assert.True(t, response.Results[0].Official)
	assert.Equal(t, 8500, response.Results[0].Stars)
}

func TestImagesHandler_Search_WithLimit(t *testing.T) {
	exec := &mockImagesExecutor{
		runFunc: func(ctx context.Context, args []string) ([]byte, error) {
			// Verify limit is passed
			for i, arg := range args {
				if arg == "--limit" && i+1 < len(args) {
					assert.Equal(t, "10", args[i+1])
				}
			}
			return []byte(`[]`), nil
		},
	}
	registry := images.NewRegistry(exec)
	router := setupImagesRouter(registry)

	req, _ := http.NewRequest("GET", "/images/search?q=test&limit=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestImagesHandler_Search_WithNoTrunc(t *testing.T) {
	var capturedArgs []string
	exec := &mockImagesExecutor{
		runFunc: func(ctx context.Context, args []string) ([]byte, error) {
			capturedArgs = args
			return []byte(`[]`), nil
		},
	}
	registry := images.NewRegistry(exec)
	router := setupImagesRouter(registry)

	req, _ := http.NewRequest("GET", "/images/search?q=python&no_trunc=true", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, capturedArgs, "--no-trunc")
}

func TestImagesHandler_Search_MissingQuery(t *testing.T) {
	exec := &mockImagesExecutor{}
	registry := images.NewRegistry(exec)
	router := setupImagesRouter(registry)

	req, _ := http.NewRequest("GET", "/images/search", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response.Error, "query parameter 'q' is required")
}

func TestImagesHandler_Search_InvalidLimit(t *testing.T) {
	exec := &mockImagesExecutor{}
	registry := images.NewRegistry(exec)
	router := setupImagesRouter(registry)

	tests := []struct {
		name  string
		limit string
	}{
		{"non-numeric", "abc"},
		{"zero", "0"},
		{"negative", "-5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/images/search?q=test&limit="+tt.limit, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var response ErrorResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response.Error, "invalid limit parameter")
		})
	}
}

func TestImagesHandler_Search_RegistryError(t *testing.T) {
	exec := &mockImagesExecutor{
		runFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return nil, errors.New("network error")
		},
	}
	registry := images.NewRegistry(exec)
	router := setupImagesRouter(registry)

	req, _ := http.NewRequest("GET", "/images/search?q=python", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response.Error, "registry search failed")
}

func TestImagesHandler_Pull_Success(t *testing.T) {
	exec := &mockImagesExecutor{
		runFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return []byte(`sha256:abc123def456`), nil
		},
	}
	registry := images.NewRegistry(exec)
	router := setupImagesRouter(registry)

	body := `{"image": "python:3.12-slim"}`
	req, _ := http.NewRequest("POST", "/images/pull", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response ImagePullResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, "sha256:abc123def456", response.ImageID)
	assert.Equal(t, "python:3.12-slim", response.Image)
}

func TestImagesHandler_Pull_WithOptions(t *testing.T) {
	var capturedArgs []string
	exec := &mockImagesExecutor{
		runFunc: func(ctx context.Context, args []string) ([]byte, error) {
			capturedArgs = args
			return []byte(`sha256:abc123`), nil
		},
	}
	registry := images.NewRegistry(exec)
	router := setupImagesRouter(registry)

	body := `{"image": "python:3.12", "platform": "linux/amd64", "quiet": true}`
	req, _ := http.NewRequest("POST", "/images/pull", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, capturedArgs, "--quiet")
	assert.Contains(t, capturedArgs, "--platform")
	assert.Contains(t, capturedArgs, "linux/amd64")
}

func TestImagesHandler_Pull_InvalidRequest(t *testing.T) {
	exec := &mockImagesExecutor{}
	registry := images.NewRegistry(exec)
	router := setupImagesRouter(registry)

	tests := []struct {
		name string
		body string
	}{
		{"missing image", `{}`},
		{"invalid json", `{invalid}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("POST", "/images/pull", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestImagesHandler_Pull_Error(t *testing.T) {
	exec := &mockImagesExecutor{
		runFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return []byte("Error: unable to pull image"), errors.New("exit status 125")
		},
	}
	registry := images.NewRegistry(exec)
	router := setupImagesRouter(registry)

	body := `{"image": "nonexistent/image:latest"}`
	req, _ := http.NewRequest("POST", "/images/pull", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response.Error, "failed to pull image")
}

func TestImagesHandler_List_CompatibilitySorting(t *testing.T) {
	exec := &mockImagesExecutor{
		runFunc: func(ctx context.Context, args []string) ([]byte, error) {
			// Return images in random order
			return []byte(`{"Id":"sha256:alpine","Repository":"alpine","Tag":"3.19","Size":7000000,"Created":1704240000}
{"Id":"sha256:python","Repository":"python","Tag":"3.12-slim","Size":125000000,"Created":1704067200}
{"Id":"sha256:node","Repository":"node","Tag":"20-alpine","Size":120000000,"Created":1704153600}`), nil
		},
	}
	registry := images.NewRegistry(exec)
	router := setupImagesRouter(registry)

	req, _ := http.NewRequest("GET", "/images", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response ImagesListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Len(t, response.Images, 3)

	// python:3.12-slim should be first (RankGlibcBased = 3)
	// alpine and node-alpine should be last (RankIncompatible = 4)
	assert.Equal(t, "python", response.Images[0].Repository)
	assert.Equal(t, images.RankGlibcBased, response.Images[0].CompatibilityRank)

	// Alpine images should be sorted by name
	assert.Equal(t, images.RankIncompatible, response.Images[1].CompatibilityRank)
	assert.Equal(t, images.RankIncompatible, response.Images[2].CompatibilityRank)
}
