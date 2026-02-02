package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"stromboli/internal/secrets"
)

// SecretsHandler handles secrets-related API endpoints
type SecretsHandler struct {
	registry *secrets.Registry
}

// NewSecretsHandler creates a new secrets handler
func NewSecretsHandler(registry *secrets.Registry) *SecretsHandler {
	return &SecretsHandler{
		registry: registry,
	}
}

// List returns all available Podman secrets
// @Summary List secrets
// @Description Returns all available Podman secrets that can be injected into agents
// @Tags secrets
// @Produce json
// @Success 200 {object} SecretsListResponse
// @Failure 500 {object} SecretsListResponse
// @Router /secrets [get]
func (h *SecretsHandler) List(c *gin.Context) {
	secretsList, err := h.registry.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, SecretsListResponse{
			Error: err.Error(),
		})
		return
	}

	// Convert to response format
	response := make([]SecretInfoResponse, 0, len(secretsList))
	for _, s := range secretsList {
		response = append(response, SecretInfoResponse{
			ID:        s.ID,
			Name:      s.Name,
			CreatedAt: s.CreatedAt, // May be RFC3339 or relative time
		})
	}

	c.JSON(http.StatusOK, SecretsListResponse{
		Secrets: response,
	})
}

// Create creates a new Podman secret
// @Summary Create secret
// @Description Creates a new Podman secret that can be injected into agents
// @Tags secrets
// @Accept json
// @Produce json
// @Param request body CreateSecretRequest true "Create secret request"
// @Success 201 {object} CreateSecretResponse
// @Failure 400 {object} CreateSecretResponse
// @Failure 409 {object} CreateSecretResponse "Secret already exists"
// @Failure 500 {object} CreateSecretResponse
// @Router /secrets [post]
func (h *SecretsHandler) Create(c *gin.Context) {
	var req CreateSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, CreateSecretResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// Check if secret already exists
	exists, err := h.registry.Exists(c.Request.Context(), req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, CreateSecretResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, CreateSecretResponse{
			Success: false,
			Name:    req.Name,
			Error:   "secret already exists",
		})
		return
	}

	if err := h.registry.Create(c.Request.Context(), req.Name, req.Value); err != nil {
		c.JSON(http.StatusInternalServerError, CreateSecretResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, CreateSecretResponse{
		Success: true,
		Name:    req.Name,
	})
}

// Get returns metadata about a specific secret
// @Summary Get secret
// @Description Returns metadata about a specific Podman secret (never returns the value)
// @Tags secrets
// @Produce json
// @Param name path string true "Secret name"
// @Success 200 {object} SecretInfoResponse
// @Failure 404 {object} CreateSecretResponse "Secret not found"
// @Failure 500 {object} CreateSecretResponse
// @Router /secrets/{name} [get]
func (h *SecretsHandler) Get(c *gin.Context) {
	name := c.Param("name")

	info, err := h.registry.Inspect(c.Request.Context(), name)
	if err != nil {
		if errors.Is(err, secrets.ErrSecretNotFound) {
			c.JSON(http.StatusNotFound, CreateSecretResponse{
				Success: false,
				Error:   "secret not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, CreateSecretResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SecretInfoResponse{
		ID:        info.ID,
		Name:      info.Name,
		CreatedAt: info.CreatedAt,
	})
}

// Delete removes a Podman secret
// @Summary Delete secret
// @Description Deletes a Podman secret
// @Tags secrets
// @Produce json
// @Param name path string true "Secret name"
// @Success 200 {object} DeleteSecretResponse
// @Failure 404 {object} DeleteSecretResponse "Secret not found"
// @Failure 500 {object} DeleteSecretResponse
// @Router /secrets/{name} [delete]
func (h *SecretsHandler) Delete(c *gin.Context) {
	name := c.Param("name")

	if err := h.registry.Delete(c.Request.Context(), name); err != nil {
		if errors.Is(err, secrets.ErrSecretNotFound) {
			c.JSON(http.StatusNotFound, DeleteSecretResponse{
				Success: false,
				Name:    name,
				Error:   "secret not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, DeleteSecretResponse{
			Success: false,
			Name:    name,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, DeleteSecretResponse{
		Success: true,
		Name:    name,
	})
}
