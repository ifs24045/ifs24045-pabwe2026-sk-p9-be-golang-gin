package post

import (
	"time"

	"app/internal/database"
)

// Categories adalah daftar kategori artikel yang diizinkan.
var Categories = []string{"Teknologi", "Pendidikan", "Gaya Hidup", "Bisnis", "Opini", "Umum"}

// PostRequest adalah payload pembuatan dan pembaruan artikel.
type PostRequest struct {
	Title          string `json:"title" binding:"required,max=255"`
	Category       string `json:"category"`
	Excerpt        string `json:"excerpt"`
	Content        string `json:"content" binding:"required"`
	CoverImagePath string `json:"cover_image_path"`
}

// ListFilter adalah filter pencarian artikel publik.
type ListFilter struct {
	Query    string
	Category string
}

// PostCount adalah jumlah artikel total dan yang terbit.
type PostCount struct {
	Total     int64
	Published int64
}

// Author adalah ringkasan penulis artikel.
type Author struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

// PostResponse adalah data artikel yang dikirim ke klien.
type PostResponse struct {
	ID             uint       `json:"id"`
	UserID         uint       `json:"user_id"`
	Author         *Author    `json:"author,omitempty"`
	Title          string     `json:"title"`
	Slug           string     `json:"slug"`
	Category       string     `json:"category"`
	Excerpt        string     `json:"excerpt"`
	Content        string     `json:"content"`
	CoverImagePath string     `json:"cover_image_path"`
	IsPublished    bool       `json:"is_published"`
	PublishedAt    *time.Time `json:"published_at"`
	ReadingTime    int        `json:"reading_time"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// StatsResponse adalah ringkasan 3 kartu statistik penulis.
type StatsResponse struct {
	Total         int64 `json:"total"`
	Published     int64 `json:"published"`
	Draft         int64 `json:"draft"`
	TotalArticles int64 `json:"total_articles"`
	TotalSelesai  int64 `json:"total_selesai"`
	BelumSelesai  int64 `json:"belum_selesai"`
}

// AdminUserResponse adalah baris direktori pengguna untuk admin.
type AdminUserResponse struct {
	ID             uint      `json:"id"`
	Username       string    `json:"username"`
	Email          string    `json:"email"`
	Role           string    `json:"role"`
	TotalPosts     int64     `json:"total_posts"`
	TotalPublished int64     `json:"total_published"`
	CreatedAt      time.Time `json:"created_at"`
}

// ToPostResponse mengubah model Post menjadi PostResponse.
func ToPostResponse(post *database.Post) PostResponse {
	resp := PostResponse{
		ID:             post.ID,
		UserID:         post.UserID,
		Title:          post.Title,
		Slug:           post.Slug,
		Category:       post.Category,
		Excerpt:        post.Excerpt,
		Content:        post.Content,
		CoverImagePath: post.CoverImagePath,
		IsPublished:    post.IsPublished,
		PublishedAt:    post.PublishedAt,
		ReadingTime:    ReadingTime(post.Content),
		CreatedAt:      post.CreatedAt,
		UpdatedAt:      post.UpdatedAt,
	}
	if post.User != nil {
		resp.Author = &Author{ID: post.User.ID, Username: post.User.Username}
	}
	return resp
}