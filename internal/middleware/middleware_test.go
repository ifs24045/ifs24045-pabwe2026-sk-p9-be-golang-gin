package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

const secret = "test-secret"

func init() {
	gin.SetMode(gin.TestMode)
}

func makeToken(t *testing.T, key string, method jwt.SigningMethod, claims jwt.MapClaims) string {
	t.Helper()
	signed, err := jwt.NewWithClaims(method, claims).SignedString([]byte(key))
	assert.NoError(t, err)
	return signed
}

func validClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"user_id":  7,
		"username": "budi",
		"role":     "member",
		"exp":      time.Now().Add(time.Hour).Unix(),
	}
}

func serve(r http.Handler, authorization string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func authRouter() *gin.Engine {
	r := gin.New()
	r.Use(Auth(secret))
	r.GET("/me", func(c *gin.Context) {
		u, ok := GetUser(c)
		c.JSON(http.StatusOK, gin.H{"ok": ok, "id": u.ID, "username": u.Username, "role": u.Role})
	})
	return r
}

func TestCORSHeadersOnGet(t *testing.T) {
	r := gin.New()
	r.Use(CORS())
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ping", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "pong", w.Body.String())
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "PATCH")
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Authorization")
}

func TestCORSPreflight(t *testing.T) {
	r := gin.New()
	r.Use(CORS())
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodOptions, "/ping", nil))

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestAuthRejectsInvalidRequests(t *testing.T) {
	expired := validClaims()
	expired["exp"] = time.Now().Add(-time.Hour).Unix()

	noUserID := jwt.MapClaims{"username": "budi", "role": "member", "exp": time.Now().Add(time.Hour).Unix()}

	cases := map[string]string{
		"tanpa header":     "",
		"skema bukan bearer": "Basic abc123",
		"token kosong":     "Bearer ",
		"token acak":       "Bearer bukan.token.valid",
		"secret salah":     "Bearer " + makeToken(t, "secret-lain", jwt.SigningMethodHS256, validClaims()),
		"kedaluwarsa":      "Bearer " + makeToken(t, secret, jwt.SigningMethodHS256, expired),
		"algoritma HS512":  "Bearer " + makeToken(t, secret, jwt.SigningMethodHS512, validClaims()),
		"tanpa user_id":    "Bearer " + makeToken(t, secret, jwt.SigningMethodHS256, noUserID),
	}

	for name, header := range cases {
		t.Run(name, func(t *testing.T) {
			w := serve(authRouter(), header)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.Contains(t, w.Body.String(), `"status":"fail"`)
		})
	}
}

func TestAuthSuccessInjectsUser(t *testing.T) {
	token := makeToken(t, secret, jwt.SigningMethodHS256, validClaims())

	w := serve(authRouter(), "Bearer "+token)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, true, body["ok"])
	assert.Equal(t, float64(7), body["id"])
	assert.Equal(t, "budi", body["username"])
	assert.Equal(t, "member", body["role"])
}

func TestGetUser(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	_, ok := GetUser(c)
	assert.False(t, ok)

	c.Set(UserContextKey, "bukan AuthUser")
	_, ok = GetUser(c)
	assert.False(t, ok)

	c.Set(UserContextKey, AuthUser{ID: 1, Username: "a", Role: "admin"})
	user, ok := GetUser(c)
	assert.True(t, ok)
	assert.Equal(t, uint(1), user.ID)
	assert.Equal(t, "admin", user.Role)
}

func TestRequireRole(t *testing.T) {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if role := c.GetHeader("X-Role"); role != "" {
			c.Set(UserContextKey, AuthUser{ID: 1, Role: role})
		}
		c.Next()
	})
	r.Use(RequireRole("admin", "editor"))
	r.GET("/admin", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	do := func(role string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/admin", nil)
		if role != "" {
			req.Header.Set("X-Role", role)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	assert.Equal(t, http.StatusUnauthorized, do("").Code)
	assert.Equal(t, http.StatusForbidden, do("member").Code)
	assert.Equal(t, http.StatusOK, do("admin").Code)
	assert.Equal(t, http.StatusOK, do("editor").Code)
}

func TestRateLimiterBlocksAfterLimit(t *testing.T) {
	r := gin.New()
	r.Use(RateLimiterWith(2, time.Minute))
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	codes := []int{}
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ping", nil))
		codes = append(codes, w.Code)
	}

	assert.Equal(t, []int{http.StatusOK, http.StatusOK, http.StatusTooManyRequests}, codes)
}

func TestRateLimiterResetsAfterWindow(t *testing.T) {
	r := gin.New()
	r.Use(RateLimiterWith(1, 20*time.Millisecond))
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	hit := func() int {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ping", nil))
		return w.Code
	}

	assert.Equal(t, http.StatusOK, hit())
	assert.Equal(t, http.StatusTooManyRequests, hit())
	time.Sleep(40 * time.Millisecond)
	assert.Equal(t, http.StatusOK, hit())
}

func TestRateLimiterDefault(t *testing.T) {
	r := gin.New()
	r.Use(RateLimiter())
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ping", nil))

	assert.Equal(t, http.StatusOK, w.Code)
}