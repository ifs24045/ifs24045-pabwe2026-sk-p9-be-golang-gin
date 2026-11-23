package auth

import (
	"strings"

	"app/internal/database"

	"gorm.io/gorm"
)

// Repository mendefinisikan akses data pengguna.
type Repository interface {
	FindByID(id uint) (*database.User, error)
	FindByIdentity(identity string) (*database.User, error)
	ExistsUsername(username string, excludeID uint) (bool, error)
	ExistsEmail(email string, excludeID uint) (bool, error)
	Create(user *database.User) error
	Update(user *database.User) error
}

type repository struct {
	db *gorm.DB
}

// NewRepository membuat Repository berbasis GORM.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) FindByID(id uint) (*database.User, error) {
	var user database.User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByIdentity mencari pengguna berdasarkan username atau email.
func (r *repository) FindByIdentity(identity string) (*database.User, error) {
	var user database.User
	err := r.db.Where("username = ? OR email = ?", identity, strings.ToLower(identity)).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *repository) ExistsUsername(username string, excludeID uint) (bool, error) {
	var count int64
	err := r.db.Model(&database.User{}).Where("username = ? AND id <> ?", username, excludeID).Count(&count).Error
	return count > 0, err
}

func (r *repository) ExistsEmail(email string, excludeID uint) (bool, error) {
	var count int64
	err := r.db.Model(&database.User{}).Where("email = ? AND id <> ?", email, excludeID).Count(&count).Error
	return count > 0, err
}

func (r *repository) Create(user *database.User) error {
	return r.db.Create(user).Error
}

func (r *repository) Update(user *database.User) error {
	return r.db.Save(user).Error
}