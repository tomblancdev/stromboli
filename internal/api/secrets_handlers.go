package api

import (
	"errors"
	"log/slog"
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
// @Description Returns metadata for all available Podman secrets that can be injected into agents. Secret values are never returned - only IDs, names, and creation times.
// @Tags secrets
// @Produce json
// @Success 200 {object} SecretsListResponse "List of secrets (empty array if none exist)"
// @Failure 500 {object} SecretsListResponse "Internal server error"
// @Router /secrets [get]
func (h *SecretsHandler) List(c *gin.Context) {
	secretsList, err := h.registry.List(c.Request.Context())
	if err != nil {
		slog.Error("Failed to list secrets", "error", err)
		c.JSON(http.StatusInternalServerError, SecretsListResponse{
			Error: "failed to list secrets",
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
// @Description Creates a new Podman secret that can be injected into agents. Secret names must be alphanumeric (with dashes and underscores), max 253 characters. Values are limited to 1MB.
// @Tags secrets
// @Accept json
// @Produce json
// @Param request body CreateSecretRequest true "Create secret request"
// @Success 201 {object} CreateSecretResponse "Secret created successfully"
// @Failure 400 {object} CreateSecretResponse "Invalid request (missing/invalid name or value)"
// @Failure 409 {object} CreateSecretResponse "Secret with this name already exists"
// @Failure 500 {object} CreateSecretResponse "Internal server error"
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

	// Create the secret - let Podman handle existence check atomically
	// This avoids TOCTOU race conditions
	if err := h.registry.Create(c.Request.Context(), req.Name, req.Value); err != nil {
		if errors.Is(err, secrets.ErrSecretAlreadyExists) {
			c.JSON(http.StatusConflict, CreateSecretResponse{
				Success: false,
				Name:    req.Name,
				Error:   "secret already exists",
			})
			return
		}
		slog.Error("Failed to create secret", "name", req.Name, "error", err)
		c.JSON(http.StatusInternalServerError, CreateSecretResponse{
			Success: false,
			Error:   "failed to create secret",
		})
		return
	}

	c.JSON(http.StatusCreated, CreateSecretResponse{
		Success: true,
		Name:    req.Name,
	})
}

// Get returns metadata about a specific secret
// @Summary Get secret metadata
// @Description Returns metadata about a specific Podman secret. For security, the actual secret value is never returned - only the ID, name, and creation time.
// @Tags secrets
// @Produce json
// @Param name path string true "Secret name" example(github-token)
// @Success 200 {object} SecretInfoResponse "Secret metadata"
// @Failure 404 {object} ErrorResponse "Secret not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /secrets/{name} [get]
func (h *SecretsHandler) Get(c *gin.Context) {
	name := c.Param("name")

	info, err := h.registry.Inspect(c.Request.Context(), name)
	if err != nil {
		if errors.Is(err, secrets.ErrSecretNotFound) {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error: "secret not found",
			})
			return
		}
		slog.Error("Failed to inspect secret", "name", name, "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "internal server error",
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
// @Description Permanently deletes a Podman secret. This action cannot be undone. Secrets currently in use by running containers may cause those containers to fail.
// @Tags secrets
// @Produce json
// @Param name path string true "Secret name" example(github-token)
// @Success 200 {object} DeleteSecretResponse "Secret deleted successfully"
// @Failure 404 {object} DeleteSecretResponse "Secret not found"
// @Failure 500 {object} DeleteSecretResponse "Internal server error"
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
		slog.Error("Failed to delete secret", "name", name, "error", err)
		c.JSON(http.StatusInternalServerError, DeleteSecretResponse{
			Success: false,
			Name:    name,
			Error:   "failed to delete secret",
		})
		return
	}

	c.JSON(http.StatusOK, DeleteSecretResponse{
		Success: true,
		Name:    name,
	})
}
