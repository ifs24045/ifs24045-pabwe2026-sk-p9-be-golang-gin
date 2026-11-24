package post

import (
	"errors"
	"net/http"
	"strconv"

	"app/internal/middleware"
	"app/internal/response"

	"github.com/gin-gonic/gin"
)

// Handler menangani endpoint /api/public/posts/*, /api/posts/*, dan /api/admin/*.
type Handler struct {
	service *Service
}

// NewHandler membuat Handler artikel.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrPostNotFound), errors.Is(err, ErrUserNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrPostForbidden):
		response.Fail(c, http.StatusForbidden, err.Error())
	case errors.Is(err, ErrInvalidCategory):
		response.Fail(c, http.StatusBadRequest, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, "Terjadi kesalahan pada server")
	}
}

func currentUserID(c *gin.Context) (uint, bool) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, "Anda belum login")
	}
	return user.ID, ok
}

func parseID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, "ID tidak valid")
		return 0, false
	}
	return uint(id), true
}

// ---------- publik ----------

// Categories godoc
// @Summary Daftar kategori artikel
// @Tags Public
// @Produce json
// @Success 200 {object} response.Body
// @Router /api/public/categories [get]
func (h *Handler) Categories(c *gin.Context) {
	response.Success(c, http.StatusOK, "Kategori berhasil diambil", h.service.Categories())
}

// ListPublic godoc
// @Summary Daftar artikel terbit (cari dan filter kategori)
// @Tags Public
// @Produce json
// @Param q query string false "Kata kunci"
// @Param category query string false "Kategori"
// @Success 200 {object} response.Body
// @Router /api/public/posts [get]
func (h *Handler) ListPublic(c *gin.Context) {
	posts, err := h.service.ListPublic(ListFilter{Query: c.Query("q"), Category: c.Query("category")})
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Artikel berhasil diambil", posts)
}

// GetPublic godoc
// @Summary Detail artikel terbit berdasarkan slug
// @Tags Public
// @Produce json
// @Param slug path string true "Slug artikel"
// @Success 200 {object} response.Body
// @Router /api/public/posts/{slug} [get]
func (h *Handler) GetPublic(c *gin.Context) {
	post, err := h.service.GetPublic(c.Param("slug"))
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Artikel berhasil diambil", post)
}

// ---------- penulis ----------

// ListMine godoc
// @Summary Daftar artikel milik penulis
// @Tags Posts
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.Body
// @Router /api/posts [get]
func (h *Handler) ListMine(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	posts, err := h.service.ListMine(userID)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Artikel berhasil diambil", posts)
}

// Create godoc
// @Summary Buat artikel baru
// @Tags Posts
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body PostRequest true "Data artikel"
// @Success 201 {object} response.Body
// @Router /api/posts [post]
func (h *Handler) Create(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	var req PostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Judul dan konten artikel wajib diisi")
		return
	}
	post, err := h.service.Create(userID, req)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusCreated, "Artikel berhasil dibuat", post)
}

// Stats godoc
// @Summary Ringkasan statistik artikel penulis
// @Tags Posts
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.Body
// @Router /api/posts/stats [get]
func (h *Handler) Stats(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	stats, err := h.service.Stats(userID)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Statistik berhasil diambil", stats)
}

// Get godoc
// @Summary Detail artikel milik penulis (untuk diedit)
// @Tags Posts
// @Security BearerAuth
// @Produce json
// @Param id path int true "ID artikel"
// @Success 200 {object} response.Body
// @Router /api/posts/{id} [get]
func (h *Handler) Get(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	post, err := h.service.GetMine(userID, id)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Artikel berhasil diambil", post)
}

// Update godoc
// @Summary Perbarui artikel milik penulis
// @Tags Posts
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "ID artikel"
// @Param body body PostRequest true "Data artikel"
// @Success 200 {object} response.Body
// @Router /api/posts/{id} [put]
func (h *Handler) Update(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req PostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Judul dan konten artikel wajib diisi")
		return
	}
	post, err := h.service.Update(userID, id, req)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Artikel berhasil diperbarui", post)
}

// TogglePublish godoc
// @Summary Ubah status Draf <-> Terbit
// @Tags Posts
// @Security BearerAuth
// @Produce json
// @Param id path int true "ID artikel"
// @Success 200 {object} response.Body
// @Router /api/posts/{id}/toggle-publish [patch]
func (h *Handler) TogglePublish(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	post, err := h.service.TogglePublish(userID, id)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Status publikasi berhasil diubah", post)
}

// Delete godoc
// @Summary Hapus artikel milik penulis
// @Tags Posts
// @Security BearerAuth
// @Produce json
// @Param id path int true "ID artikel"
// @Success 200 {object} response.Body
// @Router /api/posts/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.service.Delete(userID, id); err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Artikel berhasil dihapus", nil)
}

// ---------- admin ----------

// AdminUsers godoc
// @Summary Daftar seluruh pengguna beserta total artikel (Admin)
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.Body
// @Router /api/admin/users [get]
func (h *Handler) AdminUsers(c *gin.Context) {
	users, err := h.service.AdminListUsers()
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Pengguna berhasil diambil", users)
}

// AdminPosts godoc
// @Summary Arsip seluruh artikel dari semua penulis (Admin)
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.Body
// @Router /api/admin/posts [get]
func (h *Handler) AdminPosts(c *gin.Context) {
	posts, err := h.service.AdminListPosts()
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Artikel berhasil diambil", posts)
}

// AdminUserPosts godoc
// @Summary Artikel milik pengguna tertentu (Admin)
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param id path int true "ID pengguna"
// @Success 200 {object} response.Body
// @Router /api/admin/users/{id}/posts [get]
func (h *Handler) AdminUserPosts(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	posts, err := h.service.AdminListUserPosts(id)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Artikel berhasil diambil", posts)
}