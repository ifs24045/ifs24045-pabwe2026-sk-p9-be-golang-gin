package server

import (
	"net/http"

	"app/docs"
	"app/internal/config"
	"app/internal/features/ai"
	"app/internal/features/auth"
	"app/internal/features/post"
	"app/internal/middleware"
	"app/internal/response"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"
)

// NewRouter merangkai seluruh rute API, Swagger UI, dan handler 404.
func NewRouter(cfg *config.Config, db *gorm.DB, search ai.SearchClient, chat ai.ChatClient) *gin.Engine {
	docs.SwaggerInfo.Host = "localhost:" + cfg.AppPort

	authHandler := auth.NewHandler(auth.NewService(auth.NewRepository(db), cfg.AppKey))
	postHandler := post.NewHandler(post.NewService(post.NewRepository(db)))
	aiHandler := ai.NewHandler(ai.NewService(search, chat))

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), middleware.CORS())

	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/swagger/index.html")
	})
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.GET("/api.json", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(docs.SwaggerInfo.ReadDoc()))
	})

	r.NoRoute(func(c *gin.Context) {
		response.Fail(c, http.StatusNotFound, "Endpoint not found")
	})

	api := r.Group("/api", middleware.RateLimiter())

	// ----- Publik -----
	public := api.Group("/public")
	public.GET("/categories", postHandler.Categories)
	public.GET("/posts", postHandler.ListPublic)
	public.GET("/posts/:slug", postHandler.GetPublic)

	api.POST("/auth/register", authHandler.Register)
	api.POST("/auth/login", authHandler.Login)

	// ----- Perlu login -----
	protected := api.Group("", middleware.Auth(cfg.AppKey))
	protected.POST("/auth/logout", authHandler.Logout)
	protected.GET("/auth/profile", authHandler.Profile)
	protected.PUT("/auth/profile", authHandler.UpdateProfile)

	protected.GET("/posts", postHandler.ListMine)
	protected.POST("/posts", postHandler.Create)
	protected.GET("/posts/stats", postHandler.Stats)
	protected.POST("/posts/ai-generate", aiHandler.Generate)
	protected.GET("/posts/:id", postHandler.Get)
	protected.PUT("/posts/:id", postHandler.Update)
	protected.PATCH("/posts/:id/toggle-publish", postHandler.TogglePublish)
	protected.DELETE("/posts/:id", postHandler.Delete)

	// ----- Admin -----
	admin := protected.Group("/admin", middleware.RequireRole("admin"))
	admin.GET("/users", postHandler.AdminUsers)
	admin.GET("/posts", postHandler.AdminPosts)
	admin.GET("/users/:id/posts", postHandler.AdminUserPosts)

	return r
}