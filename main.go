package main

import (
	"log"

	"app/internal/config"
	"app/internal/database"
	"app/internal/features/ai"
	"app/internal/server"

	"github.com/gin-gonic/gin"
)

// Titik masuk untuk build di root repositori (dipakai Delcom: go build -o server .).
// Isinya sama dengan cmd/server/main.go.
func main() {
	cfg := config.LoadConfig()

	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("gagal terhubung ke database: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		log.Fatalf("gagal migrasi database: %v", err)
	}
	if err := database.Seed(db, cfg); err != nil {
		log.Fatalf("gagal seeding database: %v", err)
	}

	search := ai.NewSearchClient(cfg.SearchEngineEndpoint, cfg.OpenAIKey)
	chat := ai.NewChatClient(cfg.OpenAIEndpoint, cfg.OpenAIKey, cfg.OpenAIModel)

	router := server.NewRouter(cfg, db, search, chat)
	if err := router.Run(":" + cfg.AppPort); err != nil {
		log.Fatalf("gagal menjalankan server: %v", err)
	}
}