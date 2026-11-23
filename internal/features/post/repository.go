package post

import (
	"strings"

	"app/internal/database"

	"gorm.io/gorm"
)

// Repository mendefinisikan akses data artikel dan pengguna.
type Repository interface {
	Create(post *database.Post) error
	Update(post *database.Post) error
	Delete(id uint) error
	FindByID(id uint) (*database.Post, error)
	FindPublishedBySlug(slug string) (*database.Post, error)
	SlugExists(slug string) (bool, error)
	ListPublished(filter ListFilter) ([]database.Post, error)
	ListByUser(userID uint) ([]database.Post, error)
	ListAll() ([]database.Post, error)
	CountByUser(userID uint) (PostCount, error)
	CountPerUser() (map[uint]PostCount, error)
	ListUsers() ([]database.User, error)
	FindUser(id uint) (*database.User, error)
}

type repository struct {
	db *gorm.DB
}

// NewRepository membuat Repository berbasis GORM.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(post *database.Post) error {
	return r.db.Create(post).Error
}

func (r *repository) Update(post *database.Post) error {
	return r.db.Omit("User").Save(post).Error
}

func (r *repository) Delete(id uint) error {
	return r.db.Delete(&database.Post{}, id).Error
}

func (r *repository) FindByID(id uint) (*database.Post, error) {
	var post database.Post
	if err := r.db.Preload("User").First(&post, id).Error; err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *repository) FindPublishedBySlug(slug string) (*database.Post, error) {
	var post database.Post
	err := r.db.Preload("User").Where("slug = ? AND is_published = ?", slug, true).First(&post).Error
	if err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *repository) SlugExists(slug string) (bool, error) {
	var count int64
	err := r.db.Model(&database.Post{}).Where("slug = ?", slug).Count(&count).Error
	return count > 0, err
}

// ListPublished mengambil artikel terbit dengan pencarian kata kunci dan filter kategori.
func (r *repository) ListPublished(filter ListFilter) ([]database.Post, error) {
	query := r.db.Preload("User").Where("is_published = ?", true)

	if keyword := strings.ToLower(strings.TrimSpace(filter.Query)); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("(LOWER(title) LIKE ? OR LOWER(excerpt) LIKE ? OR LOWER(content) LIKE ?)", like, like, like)
	}
	if category := strings.TrimSpace(filter.Category); category != "" {
		query = query.Where("category = ?", category)
	}

	var posts []database.Post
	if err := query.Order("published_at DESC, id DESC").Find(&posts).Error; err != nil {
		return nil, err
	}
	return posts, nil
}

func (r *repository) ListByUser(userID uint) ([]database.Post, error) {
	var posts []database.Post
	err := r.db.Preload("User").Where("user_id = ?", userID).Order("created_at DESC, id DESC").Find(&posts).Error
	if err != nil {
		return nil, err
	}
	return posts, nil
}

func (r *repository) ListAll() ([]database.Post, error) {
	var posts []database.Post
	if err := r.db.Preload("User").Order("created_at DESC, id DESC").Find(&posts).Error; err != nil {
		return nil, err
	}
	return posts, nil
}

func (r *repository) CountByUser(userID uint) (PostCount, error) {
	var posts []database.Post
	if err := r.db.Select([]string{"is_published"}).Where("user_id = ?", userID).Find(&posts).Error; err != nil {
		return PostCount{}, err
	}

	var count PostCount
	for _, p := range posts {
		count.Total++
		if p.IsPublished {
			count.Published++
		}
	}
	return count, nil
}

func (r *repository) CountPerUser() (map[uint]PostCount, error) {
	var posts []database.Post
	if err := r.db.Select([]string{"user_id", "is_published"}).Find(&posts).Error; err != nil {
		return nil, err
	}

	counts := map[uint]PostCount{}
	for _, p := range posts {
		count := counts[p.UserID]
		count.Total++
		if p.IsPublished {
			count.Published++
		}
		counts[p.UserID] = count
	}
	return counts, nil
}

func (r *repository) ListUsers() ([]database.User, error) {
	var users []database.User
	if err := r.db.Order("id ASC").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (r *repository) FindUser(id uint) (*database.User, error) {
	var user database.User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}