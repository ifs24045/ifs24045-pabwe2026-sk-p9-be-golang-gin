package database

import (
	"strings"
	"testing"

	"app/internal/config"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

func testConfig() *config.Config {
	return &config.Config{AdminUsername: "admin", AdminPassword: "admin"}
}

func TestConnectError(t *testing.T) {
	cfg := &config.Config{
		DBHost:     "127.0.0.1",
		DBPort:     "1",
		DBUsername: "x",
		DBPassword: "x",
		DBDatabase: "x",
		DBSSLMode:  "disable",
	}

	_, err := Connect(cfg)

	assert.Error(t, err)
}

func TestNewTestDBMigratesAndIsIsolated(t *testing.T) {
	db1 := NewTestDB()
	db2 := NewTestDB()

	assert.True(t, db1.Migrator().HasTable(&User{}))
	assert.True(t, db1.Migrator().HasTable(&Post{}))

	assert.NoError(t, db1.Create(&User{Username: "a", Email: "a@x.com", Password: "p", Role: "member"}).Error)

	var c1, c2 int64
	db1.Model(&User{}).Count(&c1)
	db2.Model(&User{}).Count(&c2)
	assert.Equal(t, int64(1), c1)
	assert.Equal(t, int64(0), c2)
}

func TestMigrateError(t *testing.T) {
	db := NewTestDB()
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()

	assert.Error(t, Migrate(db))
}

func TestSeedSuccessAndIdempotent(t *testing.T) {
	db := NewTestDB()

	assert.NoError(t, Seed(db, testConfig()))
	assert.NoError(t, Seed(db, testConfig())) // kedua kali: user sudah ada

	var count int64
	db.Model(&User{}).Count(&count)
	assert.Equal(t, int64(2), count)

	var admin, member User
	assert.NoError(t, db.Where("username = ?", "admin").First(&admin).Error)
	assert.NoError(t, db.Where("username = ?", "member").First(&member).Error)
	assert.Equal(t, "admin", admin.Role)
	assert.Equal(t, "member", member.Role)
	assert.NotEqual(t, "admin", admin.Password)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte("admin")))
}

func TestSeedCountError(t *testing.T) {
	db := NewTestDB()
	assert.NoError(t, db.Migrator().DropTable(&Post{}, &User{}))

	assert.Error(t, Seed(db, testConfig()))
}

func TestSeedHashError(t *testing.T) {
	db := NewTestDB()
	cfg := &config.Config{AdminUsername: "admin", AdminPassword: strings.Repeat("a", 73)}

	assert.Error(t, Seed(db, cfg))
}

func TestSeedCreateError(t *testing.T) {
	db := NewTestDB()
	// email sudah dipakai user lain -> unique index membuat Create gagal
	assert.NoError(t, db.Create(&User{Username: "other", Email: "admin@delcom.org", Password: "p", Role: "member"}).Error)

	assert.Error(t, Seed(db, testConfig()))
}