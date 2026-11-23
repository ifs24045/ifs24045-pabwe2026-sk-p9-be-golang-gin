package database

import (
	"fmt"
	"sync/atomic"
	"time"

	"app/internal/config"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// User adalah model tabel users.
type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"size:255;uniqueIndex;not null" json:"username"`
	Email     string    `gorm:"size:255;uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"type:text;not null" json:"-"`
	Role      string    `gorm:"size:50;not null;default:member" json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Post adalah model tabel posts.
type Post struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	UserID         uint       `gorm:"not null;index" json:"user_id"`
	User           *User      `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"user,omitempty"`
	Title          string     `gorm:"size:255;not null" json:"title"`
	Slug           string     `gorm:"size:255;uniqueIndex;not null" json:"slug"`
	Category       string     `gorm:"size:100;not null;default:Umum" json:"category"`
	Excerpt        string     `gorm:"type:text" json:"excerpt"`
	Content        string     `gorm:"type:text" json:"content"`
	CoverImagePath string     `gorm:"type:text" json:"cover_image_path"`
	IsPublished    bool       `gorm:"not null;default:false" json:"is_published"`
	PublishedAt    *time.Time `json:"published_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// Connect membuka koneksi PostgreSQL untuk runtime aplikasi.
func Connect(cfg *config.Config) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{})
}

var memCounter uint64

// NewTestDB membuat database SQLite in-memory baru (terisolasi per pemanggilan)
// yang sudah dimigrasi. Dipakai oleh unit test seluruh paket.
func NewTestDB() *gorm.DB {
	n := atomic.AddUint64(&memCounter, 1)
	dsn := fmt.Sprintf("file:memdb_%d_%d?mode=memory&cache=shared&_pragma=foreign_keys(1)", time.Now().UnixNano(), n)
	db, _ := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	_ = Migrate(db)
	return db
}

// Migrate menjalankan AutoMigrate untuk seluruh model.
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&User{}, &Post{})
}

// Seed membuat akun bawaan admin dan member jika belum ada.
func Seed(db *gorm.DB, cfg *config.Config) error {
	if err := seedUser(db, cfg.AdminUsername, "admin@delcom.org", cfg.AdminPassword, "admin"); err != nil {
		return err
	}
	return seedUser(db, "member", "member@delcom.org", "member", "member")
}

func seedUser(db *gorm.DB, username, email, password, role string) error {
	var count int64
	if err := db.Model(&User{}).Where("username = ?", username).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return db.Create(&User{
		Username: username,
		Email:    email,
		Password: string(hash),
		Role:     role,
	}).Error
}