package ai

import (
	"errors"
	"net/http"

	"app/internal/response"

	"github.com/gin-gonic/gin"
)

// Handler menangani endpoint /api/posts/ai-generate.
type Handler struct {
	service *Service
}

// NewHandler membuat Handler AI.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrAIUnavailable), errors.Is(err, ErrInvalidAIResponse):
		response.Error(c, http.StatusBadGateway, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, "Terjadi kesalahan pada server")
	}
}

// Generate godoc
// @Summary Hasilkan draf artikel dengan AI + Search Engine
// @Tags AI
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body GenerateRequest true "Topik, kategori, dan opsi penelusuran web"
// @Success 200 {object} response.Body
// @Router /api/posts/ai-generate [post]
func (h *Handler) Generate(c *gin.Context) {
	var req GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Topik artikel wajib diisi (minimal 3 karakter)")
		return
	}

	result, err := h.service.GeneratePost(c.Request.Context(), req)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Draf artikel berhasil dibuat", result)
}