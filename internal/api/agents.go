package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"stromboli/internal/agent"
)

// AgentsHandler exposes the persistent-agent endpoints. Constructed in
// cmd/stromboli/main.go with a fully-configured agent.Manager and an
// ArgvBuilder that knows how to assemble a podman + claude command for the
// running deployment.
type AgentsHandler struct {
	manager *agent.Manager
	build   agent.ArgvBuilder
}

// NewAgentsHandler wires the manager + argv builder into one handler.
func NewAgentsHandler(manager *agent.Manager, build agent.ArgvBuilder) *AgentsHandler {
	return &AgentsHandler{manager: manager, build: build}
}

// CreateAgentResponse is the body of POST /agents.
// @Description Created agent metadata
type CreateAgentResponse struct {
	agent.Snapshot
}

// SendAgentRequest is the body of POST /agents/{id}/send.
// @Description Append a prompt to a running agent's stream-json stdin
type SendAgentRequest struct {
	Prompt string `json:"prompt" binding:"required" example:"Tell me about that alert"`
}

// SendAgentResponse is the body of POST /agents/{id}/send.
// @Description Acknowledgement that a turn has started; output streams via /stream
type SendAgentResponse struct {
	TurnID string `json:"turn_id" example:"turn-abc123"`
}

// errorBody is the payload of any non-2xx agent response. Sentinel-only —
// the agent package returns errors typed enough to map to HTTP codes.
type errorBody struct {
	Error string `json:"error"`
}

// Create starts a new agent.
// @Summary Spawn a persistent agent
// @Description Launches a long-lived Claude container with stream-json I/O.
//   The returned agent stays alive across many /send turns until DELETE or idle timeout.
// @Tags agents
// @Accept json
// @Produce json
// @Param request body agent.CreateRequest true "Agent configuration"
// @Success 201 {object} CreateAgentResponse
// @Failure 400 {object} errorBody "Invalid request"
// @Failure 500 {object} errorBody "Spawn failed"
// @Router /agents [post]
func (h *AgentsHandler) Create(c *gin.Context) {
	var req agent.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorBody{Error: err.Error()})
		return
	}
	snap, err := h.manager.Create(c.Request.Context(), req, h.build)
	if err != nil {
		status, body := mapAgentError(err)
		c.JSON(status, body)
		return
	}
	c.JSON(http.StatusCreated, CreateAgentResponse{Snapshot: snap})
}

// List returns every agent currently registered.
// @Summary List persistent agents
// @Tags agents
// @Produce json
// @Success 200 {array} agent.Snapshot
// @Router /agents [get]
func (h *AgentsHandler) List(c *gin.Context) {
	c.JSON(http.StatusOK, h.manager.List())
}

// Get returns one agent's metadata.
// @Summary Get persistent agent
// @Tags agents
// @Produce json
// @Param id path string true "Agent ID"
// @Success 200 {object} agent.Snapshot
// @Failure 404 {object} errorBody
// @Router /agents/{id} [get]
func (h *AgentsHandler) Get(c *gin.Context) {
	snap, err := h.manager.Get(c.Param("id"))
	if err != nil {
		status, body := mapAgentError(err)
		c.JSON(status, body)
		return
	}
	c.JSON(http.StatusOK, snap)
}

// Send appends a prompt to a running agent.
// @Summary Send a turn to a persistent agent
// @Tags agents
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Param request body SendAgentRequest true "Prompt"
// @Success 202 {object} SendAgentResponse
// @Failure 400 {object} errorBody
// @Failure 404 {object} errorBody
// @Failure 409 {object} errorBody "Turn already in progress"
// @Failure 410 {object} errorBody "Agent has exited"
// @Router /agents/{id}/send [post]
func (h *AgentsHandler) Send(c *gin.Context) {
	var req SendAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorBody{Error: err.Error()})
		return
	}
	res, err := h.manager.Send(c.Param("id"), req.Prompt)
	if err != nil {
		status, body := mapAgentError(err)
		c.JSON(status, body)
		return
	}
	c.JSON(http.StatusAccepted, SendAgentResponse{TurnID: res.TurnID})
}

// Stream opens an SSE feed of the agent's output events.
// @Summary Stream a persistent agent's events (SSE)
// @Tags agents
// @Produce text/event-stream
// @Param id path string true "Agent ID"
// @Success 200 {string} string "SSE stream of agent.Event objects"
// @Failure 404 {object} errorBody
// @Router /agents/{id}/stream [get]
func (h *AgentsHandler) Stream(c *gin.Context) {
	events, cancel, err := h.manager.Subscribe(c.Param("id"))
	if err != nil {
		status, body := mapAgentError(err)
		c.JSON(status, body)
		return
	}
	defer cancel()

	// SSE headers — the standard set; gin doesn't have a built-in helper
	// for this, but it's three lines so we just write them directly.
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no") // disable proxy buffering
	c.Writer.WriteHeader(http.StatusOK)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		// No flusher means the response can't actually stream; bail with
		// a meaningful error instead of silently buffering forever.
		_, _ = io.WriteString(c.Writer, "event: error\ndata: streaming not supported\n\n")
		return
	}
	flusher.Flush()

	clientGone := c.Request.Context().Done()
	for {
		select {
		case <-clientGone:
			return
		case ev, open := <-events:
			if !open {
				// Subscriber chan closed — agent exited or was deleted.
				_, _ = io.WriteString(c.Writer, "event: closed\ndata: {}\n\n")
				flusher.Flush()
				return
			}
			payload, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", ev.Type, payload); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// Delete tears an agent down.
// @Summary Stop a persistent agent
// @Tags agents
// @Param id path string true "Agent ID"
// @Success 204 "No Content"
// @Failure 404 {object} errorBody
// @Router /agents/{id} [delete]
func (h *AgentsHandler) Delete(c *gin.Context) {
	if err := h.manager.Stop(c.Param("id")); err != nil {
		status, body := mapAgentError(err)
		c.JSON(status, body)
		return
	}
	c.Status(http.StatusNoContent)
}

// StopAll is exposed for graceful server shutdown — main.go calls it from the
// shutdown path so we don't leak podman containers on SIGTERM.
func (h *AgentsHandler) StopAll(_ context.Context) {
	if h == nil || h.manager == nil {
		return
	}
	h.manager.StopAll()
}

// mapAgentError translates an agent package sentinel into an HTTP response.
func mapAgentError(err error) (int, errorBody) {
	switch {
	case errors.Is(err, agent.ErrAgentNotFound):
		return http.StatusNotFound, errorBody{Error: err.Error()}
	case errors.Is(err, agent.ErrAgentBusy):
		return http.StatusConflict, errorBody{Error: err.Error()}
	case errors.Is(err, agent.ErrAgentExited):
		// 410 Gone fits better than 404 for a known-but-dead agent.
		return http.StatusGone, errorBody{Error: err.Error()}
	case errors.Is(err, agent.ErrInvalidRequest):
		return http.StatusBadRequest, errorBody{Error: err.Error()}
	case errors.Is(err, agent.ErrAgentNotReady):
		return http.StatusServiceUnavailable, errorBody{Error: err.Error()}
	default:
		return http.StatusInternalServerError, errorBody{Error: err.Error()}
	}
}
