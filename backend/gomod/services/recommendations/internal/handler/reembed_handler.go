package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	recssvc "goshop/services/recommendations/internal/service"
	"goshop/shared/pkg/utils"
)

// ReembedHandler lets the catalog refresh a product's vector the moment it is
// published, instead of waiting for the next boot backfill.
//
// Without it a product created or renamed after the service started has no
// vector — it is invisible to every vector shelf until the process restarts,
// which on a long-lived deployment means it is simply never recommended.
type ReembedHandler struct {
	embedding *recssvc.EmbeddingService
	// personalized is optional and used only to flush the shelf cache after a
	// publish. A stale shelf is a minute of staleness, but a newly published
	// product that no shelf will show until the TTL expires is the one visible
	// symptom worth removing.
	personalized recssvc.PersonalizedService
}

func NewReembedHandler(embedding *recssvc.EmbeddingService, personalized recssvc.PersonalizedService) *ReembedHandler {
	return &ReembedHandler{embedding: embedding, personalized: personalized}
}

// invalidateAfterEmbed drops the cached shelves after a publish.
//
// It runs after a successful embed, including the "text unchanged" case,
// because a price or stock edit also reaches this path and both feed the card a
// shelf renders. Failure is silent on purpose: the TTL is the fallback, and a
// cache flush must never turn a successful publish into an error response.
func (h *ReembedHandler) invalidateAfterEmbed() {
	if h.personalized == nil {
		return
	}
	h.personalized.Invalidate()
}

// Embed godoc
// @Summary      Embed one product
// @Description  Recomputes the product's embedding from its current catalog text. Idempotent: unchanged text is skipped by content hash.
// @Tags         Recommendations
// @Produce      json
// @Param        uuid path string true "Product UUID"
// @Success      200 {object} map[string]any
// @Failure      400 {object} utils.ErrorResponse
// @Router       /internal/embeddings/{uuid} [post]
func (h *ReembedHandler) Embed(ctx *gin.Context) {
	if h.embedding == nil {
		// No EMBEDDER_URL: there is no vector to refresh. A 503 tells the
		// catalog to stop retrying instead of logging a bug on its side.
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeServiceUnavailable, "embedder is not configured"))
		return
	}
	written, err := h.embedding.EmbedOne(ctx.Request.Context(), ctx.Param("uuid"))
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}
	h.invalidateAfterEmbed()
	// written is 0 when the text was unchanged; the catalog gets the same
	// answer either way, because the goal is a current vector, not a write.
	utils.ResponseSuccess(ctx, http.StatusOK, "Product embedding is current", map[string]any{
		"product_uuid": ctx.Param("uuid"),
		"embedded":     written == 1,
	})
}
