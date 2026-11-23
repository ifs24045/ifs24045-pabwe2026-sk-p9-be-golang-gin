package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"app/internal/config"
	"app/internal/database"
	"app/internal/features/ai"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type stubSearch struct{}

func (stubSearch) Search(_ context.Context, _ string) ([]ai.SearchSource, error) {
	return []ai.SearchSource{{Title: "Go", URL: "https://go.dev", Snippet: "Bahasa Go"}}, nil
}

type stubChat struct{}

func (stubChat) Complete(_ context.Context, _, _ string) (string, error) {
	return `{"title":"Judul AI","category":"Teknologi","excerpt":"Ringkas","content":"Isi artikel AI"}`, nil
}

func setup(t *testing.T) *gin.Engine {
	t.Helper()
	db := database.NewTestDB()
	cfg := &config.Config{
		AppPort:       "8081",
		AppKey:        "test-secret-key",
		AdminUsername: "admin",
		AdminPassword: "admin",
	}
	assert.NoError(t, database.Seed(db, cfg))
	return NewRouter(cfg, db, stubSearch{}, stubChat{})
}

func call(r http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body
}

func login(t *testing.T, r http.Handler, username, password string) string {
	t.Helper()
	body := fmt.Sprintf(`{"username":"%s","password":"%s"}`, username, password)
	w := call(r, http.MethodPost, "/api/auth/login", body, "")
	assert.Equal(t, http.StatusOK, w.Code)
	return decode(t, w)["data"].(map[string]interface{})["token"].(string)
}

func TestRootRedirectsToSwagger(t *testing.T) {
	w := call(setup(t), http.MethodGet, "/", "", "")

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/swagger/index.html", w.Header().Get("Location"))
}

func TestSwaggerUIAndSpec(t *testing.T) {
	r := setup(t)

	assert.Equal(t, http.StatusOK, call(r, http.MethodGet, "/swagger/index.html", "", "").Code)
	assert.Equal(t, http.StatusOK, call(r, http.MethodGet, "/swagger/doc.json", "", "").Code)

	w := call(r, http.MethodGet, "/api.json", "", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	assert.Contains(t, w.Body.String(), "Delcom Blog API")
	assert.Contains(t, w.Body.String(), "/api/posts/ai-generate")
}

func TestNotFoundAndCORS(t *testing.T) {
	r := setup(t)

	w := call(r, http.MethodGet, "/api/tidak-ada", "", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "fail", decode(t, w)["status"])
	assert.Equal(t, "Endpoint not found", decode(t, w)["message"])

	w = call(r, http.MethodOptions, "/api/public/posts", "", "")
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestMemberFlow(t *testing.T) {
	r := setup(t)

	w := call(r, http.MethodPost, "/api/auth/register", `{"username":"budi","email":"budi@mail.com","password":"rahasia123"}`, "")
	assert.Equal(t, http.StatusCreated, w.Code)
	token := login(t, r, "budi", "rahasia123")

	// tanpa token ditolak
	assert.Equal(t, http.StatusUnauthorized, call(r, http.MethodGet, "/api/posts", "", "").Code)

	// buat artikel
	w = call(r, http.MethodPost, "/api/posts", `{"title":"Halo Dunia","content":"isi artikel","category":"Teknologi"}`, token)
	assert.Equal(t, http.StatusCreated, w.Code)
	id := strconv.Itoa(int(decode(t, w)["data"].(map[string]interface{})["id"].(float64)))

	assert.Equal(t, http.StatusOK, call(r, http.MethodGet, "/api/posts", "", token).Code)
	assert.Equal(t, http.StatusOK, call(r, http.MethodGet, "/api/posts/stats", "", token).Code)
	assert.Equal(t, http.StatusOK, call(r, http.MethodGet, "/api/posts/"+id, "", token).Code)
	assert.Equal(t, http.StatusOK, call(r, http.MethodPut, "/api/posts/"+id, `{"title":"Halo Dunia Baru","content":"isi baru"}`, token).Code)

	// belum terbit -> tidak tampil di publik; setelah toggle -> tampil
	w = call(r, http.MethodGet, "/api/public/posts", "", "")
	assert.Len(t, decode(t, w)["data"], 0)
	assert.Equal(t, http.StatusOK, call(r, http.MethodPatch, "/api/posts/"+id+"/toggle-publish", "", token).Code)
	w = call(r, http.MethodGet, "/api/public/posts", "", "")
	assert.Len(t, decode(t, w)["data"], 1)
	assert.Equal(t, http.StatusOK, call(r, http.MethodGet, "/api/public/posts/halo-dunia", "", "").Code)
	assert.Equal(t, http.StatusOK, call(r, http.MethodGet, "/api/public/categories", "", "").Code)

	// profil & logout
	assert.Equal(t, http.StatusOK, call(r, http.MethodGet, "/api/auth/profile", "", token).Code)
	assert.Equal(t, http.StatusOK, call(r, http.MethodPut, "/api/auth/profile", `{"username":"budi","email":"budi@mail.com"}`, token).Code)
	assert.Equal(t, http.StatusOK, call(r, http.MethodPost, "/api/auth/logout", "", token).Code)

	// AI generator
	w = call(r, http.MethodPost, "/api/posts/ai-generate", `{"topic":"Belajar Go","include_web_search":true}`, token)
	assert.Equal(t, http.StatusOK, w.Code)
	data := decode(t, w)["data"].(map[string]interface{})
	assert.Equal(t, "Judul AI", data["title"])
	assert.Equal(t, true, data["web_search_used"])

	// member tidak boleh masuk area admin
	assert.Equal(t, http.StatusForbidden, call(r, http.MethodGet, "/api/admin/users", "", token).Code)

	// hapus
	assert.Equal(t, http.StatusOK, call(r, http.MethodDelete, "/api/posts/"+id, "", token).Code)
	assert.Equal(t, http.StatusNotFound, call(r, http.MethodGet, "/api/posts/"+id, "", token).Code)
}

func TestAdminFlow(t *testing.T) {
	r := setup(t)
	token := login(t, r, "admin", "admin")

	w := call(r, http.MethodGet, "/api/admin/users", "", token)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Len(t, decode(t, w)["data"], 2)

	assert.Equal(t, http.StatusOK, call(r, http.MethodGet, "/api/admin/posts", "", token).Code)
	assert.Equal(t, http.StatusOK, call(r, http.MethodGet, "/api/admin/users/1/posts", "", token).Code)
	assert.Equal(t, http.StatusNotFound, call(r, http.MethodGet, "/api/admin/users/9999/posts", "", token).Code)
}