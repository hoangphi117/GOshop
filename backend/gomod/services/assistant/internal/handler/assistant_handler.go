package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"goshop/services/assistant/internal/dto"
	"goshop/services/assistant/internal/service"
	"goshop/shared/pkg/llm"
	"goshop/shared/pkg/middleware"
	"goshop/shared/pkg/utils"
	"goshop/shared/pkg/validation"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// logger is the app's stdout logger. See Chat for why it is injected as a
// context value instead of relying on middleware.
var logger = log.Logger

// AssistantHandler serves the storefront widget.
//
// Auth is deliberately optional: a guest must be able to ask "do you ship to
// Hanoi?" before creating an account. Logged-in callers are keyed by user id,
// guests by a session id, so the quota in Task 7 can tell them apart.
type AssistantHandler struct {
	svc     service.ChatService
	enabled bool
	ttlSec  int
}

func NewAssistantHandler(svc service.ChatService, enabled bool, sessionTTLSec int) *AssistantHandler {
	if sessionTTLSec <= 0 {
		sessionTTLSec = 3600
	}
	return &AssistantHandler{svc: svc, enabled: enabled, ttlSec: sessionTTLSec}
}

// Chat godoc
// @Summary      Ask the shopping assistant
// @Description  Grounded product advice and FAQ answers. Prices and stock come from the catalog, never from the model text.
// @Tags         Assistant
// @Accept       json
// @Produce      json
// @Param        body body dto.ChatRequest true "Question and optional history"
// @Success      200 {object} utils.APIResponse{data=dto.ChatResponse}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      429 {object} utils.ErrorResponse "Daily quota reached"
// @Failure      503 {object} utils.ErrorResponse "Assistant not configured or provider unavailable"
// @Router       /api/v1/assistant/chat [post]
func (h *AssistantHandler) Chat(c *gin.Context) {
	if !h.enabled {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeServiceUnavailable, "assistant is not configured"))
		return
	}

	var req dto.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ResponseValidator(c, validation.HandleValidationErrors(err))
		return
	}

	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = uuid.NewString()
	}
	// The app logger writes to stdout but is not attached to the request
	// context by any middleware, so without this the service's tool/round logs
	// go nowhere. One line makes the agent loop debuggable in `docker logs`.
	ctx := logger.WithContext(c.Request.Context())
	answer, err := h.svc.Ask(ctx, callerKey(c, sessionID), sessionID, toLLMHistory(req.History), req.Message)
	if err != nil {
		utils.ResponseError(c, err)
		return
	}

	utils.ResponseSuccess(c, http.StatusOK, "Assistant replied", dto.ChatResponse{
		SessionID:       sessionID,
		Answer:          answer.Answer,
		Products:        toDTOProducts(answer),
		FallbackToHuman: answer.FallbackToHuman,
		Model:           answer.Model,
		Usage: &dto.Usage{
			PromptTokens:     answer.PromptTokens,
			CompletionTokens: answer.CompletionTokens,
			CachedTokens:     answer.CachedTokens,
			ToolCalls:        answer.ToolCalls,
		},
	})
}

// streamEvent is one SSE frame. Text deltas, the final products, and errors
// share one shape so the client needs a single switch.
type streamEvent struct {
	Type            string        `json:"type"` // "delta" | "done" | "error"
	Delta           string        `json:"delta,omitempty"`
	SessionID       string        `json:"session_id,omitempty"`
	Products        []dto.Product `json:"products,omitempty"`
	FallbackToHuman bool          `json:"fallback_to_human,omitempty"`
	Error           string        `json:"error,omitempty"`
	Code            string        `json:"code,omitempty"`
}

// Stream godoc
// @Summary      Ask the shopping assistant, streamed
// @Description  Same grounding as POST /chat, but the answer arrives as SSE text deltas.
// @Description  Event types: delta (text), done (session_id + products), error.
// @Description  GET with query params so the browser can use EventSource-style reading without a body.
// @Tags         Assistant
// @Produce      text/event-stream
// @Param        message      query string true  "Question"
// @Param        session_id   query string false "Session id"
// @Success      200 {string} string "text/event-stream"
// @Failure      400 {object} utils.ErrorResponse
// @Failure      429 {object} utils.ErrorResponse
// @Failure      503 {object} utils.ErrorResponse
// @Router       /api/v1/assistant/stream [get]
func (h *AssistantHandler) Stream(c *gin.Context) {
	// ponytail: GET + query params, matching the EventSource shape. Message is
	// capped at 2000 chars so a URL stays under the 8KB request-line limit in
	// every proxy; switch to POST + a fetch ReadableStream if that cap hurts.
	if !h.enabled {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeServiceUnavailable, "assistant is not configured"))
		return
	}

	message := strings.TrimSpace(c.Query("message"))
	if message == "" {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, "message is required"))
		return
	}
	if len(message) > 2000 {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, "message is too long"))
		return
	}
	sessionID, err := sessionIDFrom(c.Query("session_id"))
	if err != nil {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeBadRequest, "session_id must be a UUID"))
		return
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeServiceUnavailable, "streaming is not supported"))
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	// nginx buffers proxied responses by default, which would hold every delta
	// until the turn finished and defeat the point.
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	flusher.Flush()

	send := func(ev streamEvent) error {
		b, err := json.Marshal(ev)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", b); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	answer, err := h.svc.AskStream(logger.WithContext(c.Request.Context()), callerKey(c, sessionID), sessionID, nil, message,
		func(delta string) error { return send(streamEvent{Type: "delta", Delta: delta}) })

	// Once the first byte is out the status line is already sent, so failures
	// are reported as an error event rather than an HTTP status. A client that
	// went away gets nothing: writing would only log noise.
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		code := string(utils.ErrCodeServiceUnavailable)
		var appErr *utils.AppError
		if errors.As(err, &appErr) {
			code = string(appErr.Code)
		}
		_ = send(streamEvent{Type: "error", Code: code, Error: userFacingError(err)})
		return
	}

	_ = send(streamEvent{
		Type:            "done",
		SessionID:       sessionID,
		Products:        toDTOProducts(answer),
		FallbackToHuman: answer.FallbackToHuman,
	})
}

// userFacingError keeps provider internals out of the browser.
func userFacingError(err error) string {
	var appErr *utils.AppError
	if errors.As(err, &appErr) {
		return appErr.Message
	}
	return "The assistant is temporarily unavailable."
}

// sessionIDFrom accepts an empty query value (mint a new session) and rejects
// anything that is not a UUID, so a client cannot write arbitrary Redis keys.
func sessionIDFrom(raw string) (string, error) {
	if raw == "" {
		return uuid.NewString(), nil
	}
	if _, err := uuid.Parse(raw); err != nil {
		return "", err
	}
	return raw, nil
}

// NewSession godoc// @Summary      Start an assistant session
// @Description  Returns a session id the widget sends back with each question.
// @Tags         Assistant
// @Produce      json
// @Success      200 {object} utils.APIResponse{data=dto.SessionResponse}
// @Failure      503 {object} utils.ErrorResponse
// @Router       /api/v1/assistant/session [post]
func (h *AssistantHandler) NewSession(c *gin.Context) {
	if !h.enabled {
		utils.ResponseError(c, utils.NewError(utils.ErrCodeServiceUnavailable, "assistant is not configured"))
		return
	}
	utils.ResponseSuccess(c, http.StatusOK, "Session created", dto.SessionResponse{
		SessionID: uuid.NewString(),
		ExpiresIn: h.ttlSec,
	})
}

// callerKey is the quota identity: user id when authenticated, else session.
func callerKey(c *gin.Context, sessionID string) string {
	if id, ok := middleware.GetUserID(c); ok {
		return "user:" + itoa(id)
	}
	return "sess:" + sessionID
}

func itoa(v int32) string {
	const digits = "0123456789"
	if v == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = digits[v%10]
		v /= 10
	}
	return string(buf[i:])
}

// toLLMHistory maps client echoes to model messages. Anything outside the
// user/assistant pair is dropped: a client must not inject system or tool turns.
func toLLMHistory(turns []dto.AssistantTurn) []llm.Message {
	out := make([]llm.Message, 0, len(turns))
	for _, t := range turns {
		switch strings.ToLower(t.Role) {
		case "user":
			out = append(out, llm.Message{Role: llm.RoleUser, Content: t.Content})
		case "assistant":
			out = append(out, llm.Message{Role: llm.RoleAssistant, Content: t.Content})
		}
	}
	return out
}

func toDTOProducts(answer service.Answer) []dto.Product {
	out := make([]dto.Product, 0, len(answer.Products))
	for _, p := range answer.Products {
		out = append(out, dto.Product{
			UUID:         p.UUID,
			Name:         p.Name,
			Slug:         p.Slug,
			Price:        p.Price,
			Stock:        p.Stock,
			CategoryName: p.CategoryName,
			ImageURL:     p.ImageURL,
		})
	}
	return out
}
