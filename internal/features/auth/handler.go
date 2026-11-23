package auth

import (
	"errors"
	"net/http"

	"app/internal/middleware"
	"app/internal/response"

	"github.com/gin-gonic/gin"
)

// Handler menangani endpoint /api/auth/*.
type Handler struct {
	service *Service
}

// NewHandler membuat Handler autentikasi.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// respondError memetakan error domain ke kode HTTP dan format respons standar.
func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrUsernameTaken), errors.Is(err, ErrEmailTaken):
		response.Fail(c, http.StatusConflict, err.Error())
	case errors.Is(err, ErrInvalidCredentials):
		response.Fail(c, http.StatusUnauthorized, err.Error())
	case errors.Is(err, ErrUserNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, "Terjadi kesalahan pada server")
	}
}

// currentUser mengambil user dari context hasil middleware Auth.
func currentUser(c *gin.Context) (middleware.AuthUser, bool) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, "Anda belum login")
	}
	return user, ok
}

// Register godoc
// @Summary Registrasi akun penulis baru
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body RegisterRequest true "Data registrasi"
// @Success 201 {object} response.Body
// @Router /api/auth/register [post]
func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Data registrasi tidak valid")
		return
	}

	user, err := h.service.Register(req)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusCreated, "Registrasi berhasil", ToUserResponse(user))
}

// Login godoc
// @Summary Login dan terima token JWT
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body LoginRequest true "Kredensial"
// @Success 200 {object} response.Body
// @Router /api/auth/login [post]
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Username dan password wajib diisi")
		return
	}

	result, err := h.service.Login(req)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Login berhasil", result)
}

// Logout godoc
// @Summary Logout sesi aktif
// @Tags Auth
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.Body
// @Router /api/auth/logout [post]
func (h *Handler) Logout(c *gin.Context) {
	response.Success(c, http.StatusOK, "Logout berhasil", nil)
}

// Profile godoc
// @Summary Ambil profil pengguna yang sedang login
// @Tags Auth
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.Body
// @Router /api/auth/profile [get]
func (h *Handler) Profile(c *gin.Context) {
	authUser, ok := currentUser(c)
	if !ok {
		return
	}

	user, err := h.service.GetProfile(authUser.ID)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Profil berhasil diambil", ToUserResponse(user))
}

// UpdateProfile godoc
// @Summary Perbarui profil pengguna yang sedang login
// @Tags Auth
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body UpdateProfileRequest true "Data profil"
// @Success 200 {object} response.Body
// @Router /api/auth/profile [put]
func (h *Handler) UpdateProfile(c *gin.Context) {
	authUser, ok := currentUser(c)
	if !ok {
		return
	}

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Data profil tidak valid")
		return
	}

	user, err := h.service.UpdateProfile(authUser.ID, req)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Profil berhasil diperbarui", ToUserResponse(user))
}