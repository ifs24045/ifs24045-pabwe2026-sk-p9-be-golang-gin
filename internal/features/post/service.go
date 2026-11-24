package post

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"app/internal/database"

	"gorm.io/gorm"
)

var (
	ErrPostNotFound    = errors.New("Artikel tidak ditemukan")
	ErrUserNotFound    = errors.New("Pengguna tidak ditemukan")
	ErrInvalidCategory = errors.New("Kategori tidak valid")
	ErrPostForbidden   = errors.New("Anda tidak memiliki akses ke artikel ini")
)

// Service berisi logika bisnis artikel.
type Service struct {
	repo Repository
}

// NewService membuat Service artikel.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// ---------- fungsi bantu ----------

// ReadingTime menghitung estimasi waktu baca dalam menit (200 kata per menit, minimal 1).
func ReadingTime(content string) int {
	words := len(strings.Fields(content))
	minutes := (words + 199) / 200
	if minutes < 1 {
		return 1
	}
	return minutes
}

// slugify mengubah judul menjadi slug URL (huruf kecil, angka, dan tanda hubung).
func slugify(title string) string {
	var b strings.Builder
	lastDash := true
	for _, r := range strings.ToLower(title) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			b.WriteRune('-')
			lastDash = true
		}
	}

	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return "artikel"
	}
	return slug
}

func normalizeCategory(category string) (string, error) {
	category = strings.TrimSpace(category)
	if category == "" {
		return "Umum", nil
	}
	for _, c := range Categories {
		if strings.EqualFold(c, category) {
			return c, nil
		}
	}
	return "", ErrInvalidCategory
}

func makeExcerpt(content string) string {
	runes := []rune(strings.TrimSpace(content))
	if len(runes) <= 160 {
		return string(runes)
	}
	return string(runes[:160]) + "..."
}

func lookupError(err error, notFound error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return notFound
	}
	return err
}

// toPublicResponses sama seperti toResponses, tetapi menyembunyikan data internal (user_id).
func toPublicResponses(posts []database.Post) []PostResponse {
	responses := make([]PostResponse, 0, len(posts))
	for i := range posts {
		resp := ToPostResponse(&posts[i])
		resp.UserID = 0
		responses = append(responses, resp)
	}
	return responses
}

// applyPublish menerapkan status terbit dari permintaan (nil = tidak diubah).
func applyPublish(post *database.Post, want *bool) {
	if want == nil {
		return
	}
	if !*want {
		post.IsPublished = false
		post.PublishedAt = nil
		return
	}
	if !post.IsPublished {
		now := time.Now()
		post.IsPublished = true
		post.PublishedAt = &now
	}
}

func toResponses(posts []database.Post) []PostResponse {
	responses := make([]PostResponse, 0, len(posts))
	for i := range posts {
		responses = append(responses, ToPostResponse(&posts[i]))
	}
	return responses
}

// prepare merapikan dan memvalidasi input artikel.
func prepare(req PostRequest) (database.Post, error) {
	category, err := normalizeCategory(req.Category)
	if err != nil {
		return database.Post{}, err
	}

	content := strings.TrimSpace(req.Content)
	excerpt := strings.TrimSpace(req.Excerpt)
	if excerpt == "" {
		excerpt = makeExcerpt(content)
	}

	return database.Post{
		Title:          strings.TrimSpace(req.Title),
		Category:       category,
		Excerpt:        excerpt,
		Content:        content,
		CoverImagePath: strings.TrimSpace(req.CoverImagePath),
	}, nil
}

// uniqueSlug membuat slug unik; jika sudah ada ditambah akhiran -2, -3, dst.
func (s *Service) uniqueSlug(title string) (string, error) {
	base := slugify(title)
	slug := base
	for i := 2; ; i++ {
		exists, err := s.repo.SlugExists(slug)
		if err != nil {
			return "", err
		}
		if !exists {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

// getOwned mengambil artikel milik penulis; artikel orang lain ditolak (403).
func (s *Service) getOwned(userID, id uint) (*database.Post, error) {
	post, err := s.repo.FindByID(id)
	if err != nil {
		return nil, lookupError(err, ErrPostNotFound)
	}
	if post.UserID != userID {
		return nil, ErrPostForbidden
	}
	return post, nil
}

// ---------- publik ----------

// Categories mengembalikan daftar kategori artikel.
func (s *Service) Categories() []string {
	return Categories
}

// ListPublic mengambil artikel terbit dengan filter pencarian dan kategori.
func (s *Service) ListPublic(filter ListFilter) ([]PostResponse, error) {
	posts, err := s.repo.ListPublished(filter)
	if err != nil {
		return nil, err
	}
	return toPublicResponses(posts), nil
}

// GetPublic mengambil satu artikel terbit berdasarkan slug.
func (s *Service) GetPublic(slug string) (*PostResponse, error) {
	post, err := s.repo.FindPublishedBySlug(slug)
	if err != nil {
		return nil, lookupError(err, ErrPostNotFound)
	}
	resp := ToPostResponse(post)
	resp.UserID = 0
	return &resp, nil
}

// ---------- penulis ----------

// ListMine mengambil seluruh artikel milik penulis (terbit dan draf).
func (s *Service) ListMine(userID uint) ([]PostResponse, error) {
	posts, err := s.repo.ListByUser(userID)
	if err != nil {
		return nil, err
	}
	return toResponses(posts), nil
}

// GetMine mengambil satu artikel milik penulis berdasarkan ID.
func (s *Service) GetMine(userID, id uint) (*PostResponse, error) {
	post, err := s.getOwned(userID, id)
	if err != nil {
		return nil, err
	}
	resp := ToPostResponse(post)
	return &resp, nil
}

// Create membuat artikel baru (draf) dengan slug unik.
func (s *Service) Create(userID uint, req PostRequest) (*PostResponse, error) {
	post, err := prepare(req)
	if err != nil {
		return nil, err
	}

	slug, err := s.uniqueSlug(post.Title)
	if err != nil {
		return nil, err
	}
	post.UserID = userID
	post.Slug = slug
	applyPublish(&post, req.IsPublished)

	if err := s.repo.Create(&post); err != nil {
		return nil, err
	}
	resp := ToPostResponse(&post)
	return &resp, nil
}

// Update memperbarui judul, kategori, excerpt, konten, dan sampul. Slug tidak berubah.
func (s *Service) Update(userID, id uint, req PostRequest) (*PostResponse, error) {
	post, err := s.getOwned(userID, id)
	if err != nil {
		return nil, err
	}

	data, err := prepare(req)
	if err != nil {
		return nil, err
	}
	post.Title = data.Title
	post.Category = data.Category
	post.Excerpt = data.Excerpt
	post.Content = data.Content
	post.CoverImagePath = data.CoverImagePath
	post.CoverImagePath = data.CoverImagePath
	applyPublish(post, req.IsPublished)

	if err := s.repo.Update(post); err != nil {
		return nil, err
	}
	resp := ToPostResponse(post)
	return &resp, nil
}

// TogglePublish membalik status Draf <-> Terbit beserta published_at.
func (s *Service) TogglePublish(userID, id uint) (*PostResponse, error) {
	post, err := s.getOwned(userID, id)
	if err != nil {
		return nil, err
	}

	if post.IsPublished {
		post.IsPublished = false
		post.PublishedAt = nil
	} else {
		now := time.Now()
		post.IsPublished = true
		post.PublishedAt = &now
	}

	if err := s.repo.Update(post); err != nil {
		return nil, err
	}
	resp := ToPostResponse(post)
	return &resp, nil
}

// Delete menghapus artikel milik penulis.
func (s *Service) Delete(userID, id uint) error {
	post, err := s.getOwned(userID, id)
	if err != nil {
		return err
	}
	return s.repo.Delete(post.ID)
}

// Stats mengembalikan ringkasan 3 kartu statistik penulis.
func (s *Service) Stats(userID uint) (*StatsResponse, error) {
	count, err := s.repo.CountByUser(userID)
	if err != nil {
		return nil, err
	}
	draft := count.Total - count.Published
	return &StatsResponse{
		Total:         count.Total,
		Published:     count.Published,
		Draft:         draft,
		TotalArticles: count.Total,
		TotalSelesai:  count.Published,
		BelumSelesai:  draft,
	}, nil
}

// ---------- admin ----------

// AdminListUsers mengambil seluruh pengguna beserta total artikel dan total terbit.
func (s *Service) AdminListUsers() ([]AdminUserResponse, error) {
	users, err := s.repo.ListUsers()
	if err != nil {
		return nil, err
	}
	counts, err := s.repo.CountPerUser()
	if err != nil {
		return nil, err
	}

	result := make([]AdminUserResponse, 0, len(users))
	for _, u := range users {
		count := counts[u.ID]
		result = append(result, AdminUserResponse{
			ID:             u.ID,
			Username:       u.Username,
			Email:          u.Email,
			Role:           u.Role,
			TotalPosts:     count.Total,
			TotalPublished: count.Published,
			CreatedAt:      u.CreatedAt,
		})
	}
	return result, nil
}

// AdminListPosts mengambil arsip seluruh artikel dari semua penulis.
func (s *Service) AdminListPosts() ([]PostResponse, error) {
	posts, err := s.repo.ListAll()
	if err != nil {
		return nil, err
	}
	return toResponses(posts), nil
}

// AdminListUserPosts mengambil seluruh artikel milik pengguna tertentu.
// Pengguna yang tidak ada menghasilkan daftar kosong, bukan error.
func (s *Service) AdminListUserPosts(userID uint) ([]PostResponse, error) {
	posts, err := s.repo.ListByUser(userID)
	if err != nil {
		return nil, err
	}
	return toResponses(posts), nil
}