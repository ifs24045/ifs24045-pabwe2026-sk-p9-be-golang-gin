package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"app/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// UserContextKey adalah kunci penyimpanan data user di gin.Context.
const UserContextKey = "user"

// AuthUser adalah data pengguna hasil pembacaan token JWT.
type AuthUser struct {
	ID       uint
	Username string
	Role     string
}

// GetUser mengambil data user yang disuntikkan oleh middleware Auth.
func GetUser(c *gin.Context) (AuthUser, bool) {
	value, exists := c.Get(UserContextKey)
	if !exists {
		return AuthUser{}, false
	}
	user, ok := value.(AuthUser)
	return user, ok
}

// CORS memberi izin akses lintas-domain bagi frontend React.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// Auth memverifikasi Bearer Token JWT dan menyuntikkan AuthUser ke context.
func Auth(secretKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		parts := strings.SplitN(c.GetHeader("Authorization"), " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			response.AbortFail(c, http.StatusUnauthorized, "Token autentikasi tidak ditemukan")
			return
		}

		token, err := jwt.Parse(
			strings.TrimSpace(parts[1]),
			func(_ *jwt.Token) (interface{}, error) { return []byte(secretKey), nil },
			jwt.WithValidMethods([]string{"HS256"}),
		)
		if err != nil || !token.Valid {
			response.AbortFail(c, http.StatusUnauthorized, "Token tidak valid atau sudah kedaluwarsa")
			return
		}

		claims, _ := token.Claims.(jwt.MapClaims)
		id, _ := claims["user_id"].(float64)
		username, _ := claims["username"].(string)
		role, _ := claims["role"].(string)
		if id == 0 {
			response.AbortFail(c, http.StatusUnauthorized, "Token tidak valid")
			return
		}

		c.Set(UserContextKey, AuthUser{ID: uint(id), Username: username, Role: role})
		c.Next()
	}
}

// RequireRole hanya meloloskan pengguna dengan salah satu role yang diizinkan.
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := GetUser(c)
		if !ok {
			response.AbortFail(c, http.StatusUnauthorized, "Anda belum login")
			return
		}

		for _, role := range roles {
			if user.Role == role {
				c.Next()
				return
			}
		}
		response.AbortFail(c, http.StatusForbidden, "Anda tidak memiliki hak akses")
	}
}

type visitor struct {
	count   int
	resetAt time.Time
}

// RateLimiter membatasi 100 request per menit untuk setiap alamat IP.
func RateLimiter() gin.HandlerFunc {
	return RateLimiterWith(100, time.Minute)
}

// RateLimiterWith membatasi jumlah request per jendela waktu untuk setiap IP.
func RateLimiterWith(limit int, window time.Duration) gin.HandlerFunc {
	var mu sync.Mutex
	visitors := map[string]*visitor{}

	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		mu.Lock()
		v, ok := visitors[ip]
		if !ok || now.After(v.resetAt) {
			v = &visitor{resetAt: now.Add(window)}
			visitors[ip] = v
		}
		v.count++
		exceeded := v.count > limit
		mu.Unlock()

		if exceeded {
			response.AbortFail(c, http.StatusTooManyRequests, "Terlalu banyak permintaan, coba lagi nanti")
			return
		}
		c.Next()
	}
}