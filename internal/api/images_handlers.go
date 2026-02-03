package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"stromboli/internal/images"
)

// ImagesHandler handles image-related API endpoints
type ImagesHandler struct {
	registry *images.Registry
}

// NewImagesHandler creates a new images handler
func NewImagesHandler(registry *images.Registry) *ImagesHandler {
	return &ImagesHandler{
		registry: registry,
	}
}

// List returns all local images sorted by compatibility
// @Summary List images
// @Description Returns all local container images sorted by compatibility rank. Images with rank 1-2 are verified compatible, rank 3 is standard glibc (compatible), rank 4 is incompatible (Alpine/musl).
// @Tags images
// @Produce json
// @Success 200 {object} ImagesListResponse "List of images (empty array if none exist)"
// @Failure 500 {object} ImagesListResponse "Internal server error"
// @Router /images [get]
func (h *ImagesHandler) List(c *gin.Context) {
	imagesList, err := h.registry.List(c.Request.Context())
	if err != nil {
		slog.Error("Failed to list images", "error", err)
		c.JSON(http.StatusInternalServerError, ImagesListResponse{
			Error: "failed to list images",
		})
		return
	}

	// Convert to response format
	response := make([]ImageInfoResponse, 0, len(imagesList))
	for _, img := range imagesList {
		response = append(response, ImageInfoResponse{
			ID:                img.ID,
			Repository:        img.Repository,
			Tag:               img.Tag,
			Size:              img.Size,
			Created:           img.Created.Format("2006-01-02T15:04:05Z"),
			CompatibilityRank: img.CompatibilityRank,
			Compatible:        img.Compatible,
			Tools:             img.Tools,
			HasClaudeCLI:      img.HasClaudeCLI,
			Description:       img.Description,
		})
	}

	c.JSON(http.StatusOK, ImagesListResponse{
		Images: response,
	})
}

// Inspect returns detailed information about a specific image
// @Summary Inspect image
// @Description Returns detailed information about a specific container image including all labels and compatibility information.
// @Tags images
// @Produce json
// @Param name path string true "Image name with optional tag" example(python:3.12-slim)
// @Success 200 {object} ImageDetailResponse "Image details"
// @Failure 400 {object} ImageDetailResponse "Invalid image name"
// @Failure 404 {object} ImageDetailResponse "Image not found"
// @Failure 500 {object} ImageDetailResponse "Internal server error"
// @Router /images/{name} [get]
func (h *ImagesHandler) Inspect(c *gin.Context) {
	name := c.Param("name")

	info, err := h.registry.Inspect(c.Request.Context(), name)
	if err != nil {
		if errors.Is(err, images.ErrImageNotFound) {
			c.JSON(http.StatusNotFound, ImageDetailResponse{
				Error: "image not found",
			})
			return
		}
		if errors.Is(err, images.ErrValidation) {
			c.JSON(http.StatusBadRequest, ImageDetailResponse{
				Error: err.Error(),
			})
			return
		}
		slog.Error("Failed to inspect image", "name", name, "error", err)
		c.JSON(http.StatusInternalServerError, ImageDetailResponse{
			Error: "internal server error",
		})
		return
	}

	c.JSON(http.StatusOK, ImageDetailResponse{
		ID:                info.ID,
		Repository:        info.Repository,
		Tag:               info.Tag,
		Size:              info.Size,
		Created:           info.Created.Format("2006-01-02T15:04:05Z"),
		Labels:            info.Labels,
		CompatibilityRank: info.CompatibilityRank,
		RankDescription:   images.RankDescription(info.CompatibilityRank),
		Compatible:        info.Compatible,
		Tools:             info.Tools,
		HasClaudeCLI:      info.HasClaudeCLI,
		Description:       info.Description,
	})
}

// Search searches registries for images
// @Summary Search images
// @Description Searches container registries for images matching the query. Returns results from Docker Hub and other configured registries.
// @Tags images
// @Produce json
// @Param q query string true "Search query" example(python)
// @Param limit query int false "Maximum number of results (default 25, max 100)" example(10)
// @Success 200 {object} ImageSearchResponse "Search results"
// @Failure 400 {object} ImageSearchResponse "Invalid request (missing query)"
// @Failure 502 {object} ImageSearchResponse "Registry search failed"
// @Failure 500 {object} ImageSearchResponse "Internal server error"
// @Router /images/search [get]
func (h *ImagesHandler) Search(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, ImageSearchResponse{
			Error: "query parameter 'q' is required",
		})
		return
	}

	// Parse optional limit parameter
	var opts *images.SearchOptions
	if limitStr := c.Query("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 1 {
			c.JSON(http.StatusBadRequest, ImageSearchResponse{
				Error: "invalid limit parameter",
			})
			return
		}
		opts = &images.SearchOptions{Limit: limit}
	}

	results, err := h.registry.Search(c.Request.Context(), query, opts)
	if err != nil {
		if errors.Is(err, images.ErrValidation) {
			c.JSON(http.StatusBadRequest, ImageSearchResponse{
				Error: err.Error(),
			})
			return
		}
		if errors.Is(err, images.ErrSearchFailed) {
			slog.Error("Registry search failed", "query", query, "error", err)
			c.JSON(http.StatusBadGateway, ImageSearchResponse{
				Error: "registry search failed",
			})
			return
		}
		slog.Error("Failed to search images", "query", query, "error", err)
		c.JSON(http.StatusInternalServerError, ImageSearchResponse{
			Error: "internal server error",
		})
		return
	}

	// Convert to response format
	response := make([]SearchResultResponse, 0, len(results))
	for _, r := range results {
		response = append(response, SearchResultResponse{
			Index:       r.Index,
			Name:        r.Name,
			Description: r.Description,
			Stars:       r.Stars,
			Official:    r.Official,
			Automated:   r.Automated,
		})
	}

	c.JSON(http.StatusOK, ImageSearchResponse{
		Results: response,
	})
}

// Pull pulls an image from a registry
// @Summary Pull image
// @Description Pulls a container image from a registry. This operation may take some time for large images.
// @Tags images
// @Accept json
// @Produce json
// @Param request body ImagePullRequest true "Pull request"
// @Success 200 {object} ImagePullResponse "Image pulled successfully"
// @Failure 400 {object} ImagePullResponse "Invalid request"
// @Failure 500 {object} ImagePullResponse "Pull failed"
// @Router /images/pull [post]
func (h *ImagesHandler) Pull(c *gin.Context) {
	var req ImagePullRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ImagePullResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	opts := &images.PullOptions{
		Quiet:    req.Quiet,
		Platform: req.Platform,
	}

	imageID, err := h.registry.Pull(c.Request.Context(), req.Image, opts)
	if err != nil {
		if errors.Is(err, images.ErrValidation) {
			c.JSON(http.StatusBadRequest, ImagePullResponse{
				Success: false,
				Image:   req.Image,
				Error:   err.Error(),
			})
			return
		}
		if errors.Is(err, images.ErrPullFailed) {
			slog.Error("Failed to pull image", "image", req.Image, "error", err)
			c.JSON(http.StatusInternalServerError, ImagePullResponse{
				Success: false,
				Image:   req.Image,
				Error:   "failed to pull image",
			})
			return
		}
		slog.Error("Unexpected error pulling image", "image", req.Image, "error", err)
		c.JSON(http.StatusInternalServerError, ImagePullResponse{
			Success: false,
			Image:   req.Image,
			Error:   "internal server error",
		})
		return
	}

	c.JSON(http.StatusOK, ImagePullResponse{
		Success: true,
		ImageID: imageID,
		Image:   req.Image,
	})
}
