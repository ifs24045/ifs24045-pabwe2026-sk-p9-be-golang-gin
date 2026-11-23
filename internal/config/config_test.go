package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var allKeys = []string{
	"APP_ENV", "APP_PORT", "APP_KEY",
	"ADMIN_USERNAME", "ADMIN_PASSWORD",
	"DB_CONNECTION", "DB_HOST", "DB_PORT", "DB_DATABASE",
	"DB_USERNAME", "DB_PASSWORD", "DB_SSLMODE",
	"SEARCH_ENGINE_ENDPOINT", "OPENAI_ENDPOINT", "OPENAI_KEY", "OPENAI_MODEL",
}

func TestGetEnv(t *testing.T) {
	t.Setenv("TEST_CFG_KEY", "value")
	assert.Equal(t, "value", getEnv("TEST_CFG_KEY", "fallback"))

	t.Setenv("TEST_CFG_EMPTY", "")
	assert.Equal(t, "fallback", getEnv("TEST_CFG_EMPTY", "fallback"))

	assert.Equal(t, "fallback", getEnv("TEST_CFG_NOT_SET_XYZ", "fallback"))
}

func TestLoadConfigDefaults(t *testing.T) {
	for _, k := range allKeys {
		t.Setenv(k, "")
	}

	cfg := LoadConfig()

	assert.Equal(t, "development", cfg.AppEnv)
	assert.Equal(t, "8081", cfg.AppPort)
	assert.NotEmpty(t, cfg.AppKey)
	assert.Equal(t, "admin", cfg.AdminUsername)
	assert.Equal(t, "admin", cfg.AdminPassword)
	assert.Equal(t, "pgsql", cfg.DBConnection)
	assert.Equal(t, "127.0.0.1", cfg.DBHost)
	assert.Equal(t, "5432", cfg.DBPort)
	assert.Equal(t, "pabwe_p9_blog", cfg.DBDatabase)
	assert.Equal(t, "postgres", cfg.DBUsername)
	assert.Equal(t, "postgres", cfg.DBPassword)
	assert.Equal(t, "disable", cfg.DBSSLMode)
	assert.Equal(t, "https://delcom.org/api/v1/search", cfg.SearchEngineEndpoint)
	assert.Equal(t, "https://delcom.org/api/v1", cfg.OpenAIEndpoint)
	assert.Equal(t, "", cfg.OpenAIKey)
	assert.Equal(t, "", cfg.OpenAIModel)
}

func TestLoadConfigFromEnv(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_PORT", "9000")
	t.Setenv("APP_KEY", "kunci-rahasia")
	t.Setenv("ADMIN_USERNAME", "root")
	t.Setenv("ADMIN_PASSWORD", "toor")
	t.Setenv("DB_CONNECTION", "pgsql")
	t.Setenv("DB_HOST", "db.local")
	t.Setenv("DB_PORT", "6543")
	t.Setenv("DB_DATABASE", "blogdb")
	t.Setenv("DB_USERNAME", "user")
	t.Setenv("DB_PASSWORD", "pass")
	t.Setenv("DB_SSLMODE", "require")
	t.Setenv("SEARCH_ENGINE_ENDPOINT", "http://search.local")
	t.Setenv("OPENAI_ENDPOINT", "http://ai.local")
	t.Setenv("OPENAI_KEY", "sk-test")
	t.Setenv("OPENAI_MODEL", "model-x")

	cfg := LoadConfig()

	assert.Equal(t, "production", cfg.AppEnv)
	assert.Equal(t, "9000", cfg.AppPort)
	assert.Equal(t, "kunci-rahasia", cfg.AppKey)
	assert.Equal(t, "root", cfg.AdminUsername)
	assert.Equal(t, "toor", cfg.AdminPassword)
	assert.Equal(t, "db.local", cfg.DBHost)
	assert.Equal(t, "6543", cfg.DBPort)
	assert.Equal(t, "blogdb", cfg.DBDatabase)
	assert.Equal(t, "user", cfg.DBUsername)
	assert.Equal(t, "pass", cfg.DBPassword)
	assert.Equal(t, "require", cfg.DBSSLMode)
	assert.Equal(t, "http://search.local", cfg.SearchEngineEndpoint)
	assert.Equal(t, "http://ai.local", cfg.OpenAIEndpoint)
	assert.Equal(t, "sk-test", cfg.OpenAIKey)
	assert.Equal(t, "model-x", cfg.OpenAIModel)
}

func TestDSN(t *testing.T) {
	cfg := &Config{
		DBHost:     "127.0.0.1",
		DBUsername: "postgres",
		DBPassword: "secret",
		DBDatabase: "pabwe_p9_blog",
		DBPort:     "5432",
		DBSSLMode:  "disable",
	}

	expected := "host=127.0.0.1 user=postgres password=secret dbname=pabwe_p9_blog port=5432 sslmode=disable TimeZone=Asia/Jakarta"
	assert.Equal(t, expected, cfg.DSN())
}