package post

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"app/internal/database"
	"app/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

var errBoom = errors.New("boom")

func init() {
	gin.SetMode(gin.TestMode)
}

// ---------- helper ----------

func mkUser(t *testing.T, db *gorm.DB, username string) *database.User {
	t.Helper()
	user := &database.User{Username: username, Email: username + "@mail.com", Password: "x", Role: "member"}
	assert.NoError(t, db.Create(user).Error)
	return user
}

func mkPost(t *testing.T, db *gorm.DB, userID uint, title, slug, category string, published bool) *database.Post {
	t.Helper()
	post := &database.Post{
		UserID:      userID,
		Title:       title,
		Slug:        slug,
		Category:    category,
		Excerpt:     "ringkasan " + title,
		Content:     "isi " + title,
		IsPublished: published,
	}
	if published {
		now := time.Now()
		post.PublishedAt = &now
	}
	assert.NoError(t, db.Create(post).Error)
	return post
}

// stubRepo membungkus repository asli dan dapat dipaksa gagal per method.
type stubRepo struct {
	Repository
	fail map[string]error
}

func newStubSvc(db *gorm.DB) (*Service, *stubRepo) {
	stub := &stubRepo{Repository: NewRepository(db), fail: map[string]error{}}
	return NewService(stub), stub
}

func (s *stubRepo) Create(p *database.Post) error {
	if err := s.fail["Create"]; err != nil {
		return err
	}
	return s.Repository.Create(p)
}

func (s *stubRepo) Update(p *database.Post) error {
	if err := s.fail["Update"]; err != nil {
		return err
	}
	return s.Repository.Update(p)
}

func (s *stubRepo) Delete(id uint) error {
	if err := s.fail["Delete"]; err != nil {
		return err
	}
	return s.Repository.Delete(id)
}

func (s *stubRepo) FindByID(id uint) (*database.Post, error) {
	if err := s.fail["FindByID"]; err != nil {
		return nil, err
	}
	return s.Repository.FindByID(id)
}

func (s *stubRepo) FindPublishedBySlug(slug string) (*database.Post, error) {
	if err := s.fail["FindPublishedBySlug"]; err != nil {
		return nil, err
	}
	return s.Repository.FindPublishedBySlug(slug)
}

func (s *stubRepo) SlugExists(slug string) (bool, error) {
	if err := s.fail["SlugExists"]; err != nil {
		return false, err
	}
	return s.Repository.SlugExists(slug)
}

func (s *stubRepo) ListPublished(f ListFilter) ([]database.Post, error) {
	if err := s.fail["ListPublished"]; err != nil {
		return nil, err
	}
	return s.Repository.ListPublished(f)
}

func (s *stubRepo) ListByUser(id uint) ([]database.Post, error) {
	if err := s.fail["ListByUser"]; err != nil {
		return nil, err
	}
	return s.Repository.ListByUser(id)
}

func (s *stubRepo) ListAll() ([]database.Post, error) {
	if err := s.fail["ListAll"]; err != nil {
		return nil, err
	}
	return s.Repository.ListAll()
}

func (s *stubRepo) CountByUser(id uint) (PostCount, error) {
	if err := s.fail["CountByUser"]; err != nil {
		return PostCount{}, err
	}
	return s.Repository.CountByUser(id)
}

func (s *stubRepo) CountPerUser() (map[uint]PostCount, error) {
	if err := s.fail["CountPerUser"]; err != nil {
		return nil, err
	}
	return s.Repository.CountPerUser()
}

func (s *stubRepo) ListUsers() ([]database.User, error) {
	if err := s.fail["ListUsers"]; err != nil {
		return nil, err
	}
	return s.Repository.ListUsers()
}

func (s *stubRepo) FindUser(id uint) (*database.User, error) {
	if err := s.fail["FindUser"]; err != nil {
		return nil, err
	}
	return s.Repository.FindUser(id)
}

// ---------- fungsi bantu murni ----------

func TestSlugify(t *testing.T) {
	assert.Equal(t, "halo-dunia-2026", slugify("  Halo, Dunia!!  2026 "))
	assert.Equal(t, "artikel", slugify("!!!"))
}

func TestReadingTime(t *testing.T) {
	assert.Equal(t, 1, ReadingTime(""))
	assert.Equal(t, 1, ReadingTime(strings.Repeat("kata ", 200)))
	assert.Equal(t, 2, ReadingTime(strings.Repeat("kata ", 201)))
	assert.Equal(t, 3, ReadingTime(strings.Repeat("kata ", 450)))
}

func TestNormalizeCategory(t *testing.T) {
	got, err := normalizeCategory("")
	assert.NoError(t, err)
	assert.Equal(t, "Umum", got)

	got, err = normalizeCategory("  gaya hidup ")
	assert.NoError(t, err)
	assert.Equal(t, "Gaya Hidup", got)

	_, err = normalizeCategory("Olahraga")
	assert.ErrorIs(t, err, ErrInvalidCategory)
}

func TestMakeExcerpt(t *testing.T) {
	assert.Equal(t, "pendek", makeExcerpt("  pendek "))

	long := makeExcerpt(strings.Repeat("a", 200))
	assert.Equal(t, strings.Repeat("a", 160)+"...", long)
}

func TestLookupError(t *testing.T) {
	assert.Equal(t, ErrPostNotFound, lookupError(gorm.ErrRecordNotFound, ErrPostNotFound))
	assert.Equal(t, errBoom, lookupError(errBoom, ErrPostNotFound))
}

func TestToPostResponse(t *testing.T) {
	post := &database.Post{ID: 1, UserID: 2, Title: "Judul", Content: "isi"}
	resp := ToPostResponse(post)
	assert.Nil(t, resp.Author)
	assert.Equal(t, 1, resp.ReadingTime)

	post.User = &database.User{ID: 2, Username: "budi"}
	resp = ToPostResponse(post)
	assert.Equal(t, "budi", resp.Author.Username)
}

// ---------- repository ----------

func TestRepository(t *testing.T) {
	db := database.NewTestDB()
	repo := NewRepository(db)
	a := mkUser(t, db, "andi")
	b := mkUser(t, db, "budi")

	p1 := mkPost(t, db, a.ID, "Go Dasar", "go-dasar", "Teknologi", true)
	p2 := mkPost(t, db, a.ID, "Draf Andi", "draf-andi", "Bisnis", false)
	p3 := mkPost(t, db, b.ID, "Belajar Gizi", "belajar-gizi", "Gaya Hidup", true)

	found, err := repo.FindByID(p1.ID)
	assert.NoError(t, err)
	assert.Equal(t, "andi", found.User.Username)
	_, err = repo.FindByID(9999)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	bySlug, err := repo.FindPublishedBySlug("go-dasar")
	assert.NoError(t, err)
	assert.Equal(t, p1.ID, bySlug.ID)
	_, err = repo.FindPublishedBySlug("draf-andi")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	exists, err := repo.SlugExists("go-dasar")
	assert.NoError(t, err)
	assert.True(t, exists)
	exists, err = repo.SlugExists("tidak-ada")
	assert.NoError(t, err)
	assert.False(t, exists)

	all, err := repo.ListPublished(ListFilter{})
	assert.NoError(t, err)
	assert.Len(t, all, 2)

	byTitle, err := repo.ListPublished(ListFilter{Query: "  DASAR "})
	assert.NoError(t, err)
	assert.Len(t, byTitle, 1)
	assert.Equal(t, p1.ID, byTitle[0].ID)

	byExcerpt, err := repo.ListPublished(ListFilter{Query: "ringkasan belajar"})
	assert.NoError(t, err)
	assert.Len(t, byExcerpt, 1)

	byContent, err := repo.ListPublished(ListFilter{Query: "isi go"})
	assert.NoError(t, err)
	assert.Len(t, byContent, 1)

	byCategory, err := repo.ListPublished(ListFilter{Category: " Gaya Hidup "})
	assert.NoError(t, err)
	assert.Len(t, byCategory, 1)
	assert.Equal(t, p3.ID, byCategory[0].ID)

	both, err := repo.ListPublished(ListFilter{Query: "go", Category: "Gaya Hidup"})
	assert.NoError(t, err)
	assert.Len(t, both, 0)

	mine, err := repo.ListByUser(a.ID)
	assert.NoError(t, err)
	assert.Len(t, mine, 2)

	everything, err := repo.ListAll()
	assert.NoError(t, err)
	assert.Len(t, everything, 3)

	p2.Title = "Draf Andi Baru"
	assert.NoError(t, repo.Update(p2))
	reloaded, _ := repo.FindByID(p2.ID)
	assert.Equal(t, "Draf Andi Baru", reloaded.Title)

	count, err := repo.CountByUser(a.ID)
	assert.NoError(t, err)
	assert.Equal(t, PostCount{Total: 2, Published: 1}, count)

	perUser, err := repo.CountPerUser()
	assert.NoError(t, err)
	assert.Equal(t, PostCount{Total: 2, Published: 1}, perUser[a.ID])
	assert.Equal(t, PostCount{Total: 1, Published: 1}, perUser[b.ID])

	users, err := repo.ListUsers()
	assert.NoError(t, err)
	assert.Len(t, users, 2)

	user, err := repo.FindUser(a.ID)
	assert.NoError(t, err)
	assert.Equal(t, "andi", user.Username)
	_, err = repo.FindUser(9999)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	assert.NoError(t, repo.Delete(p3.ID))
	_, err = repo.FindByID(p3.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestRepositoryErrors(t *testing.T) {
	db := database.NewTestDB()
	repo := NewRepository(db)
	assert.NoError(t, db.Migrator().DropTable(&database.Post{}, &database.User{}))

	assert.Error(t, repo.Create(&database.Post{}))
	assert.Error(t, repo.Update(&database.Post{ID: 1}))
	assert.Error(t, repo.Delete(1))

	_, err := repo.FindByID(1)
	assert.Error(t, err)
	_, err = repo.FindPublishedBySlug("x")
	assert.Error(t, err)
	_, err = repo.SlugExists("x")
	assert.Error(t, err)
	_, err = repo.ListPublished(ListFilter{})
	assert.Error(t, err)
	_, err = repo.ListByUser(1)
	assert.Error(t, err)
	_, err = repo.ListAll()
	assert.Error(t, err)
	_, err = repo.CountByUser(1)
	assert.Error(t, err)
	_, err = repo.CountPerUser()
	assert.Error(t, err)
	_, err = repo.ListUsers()
	assert.Error(t, err)
	_, err = repo.FindUser(1)
	assert.Error(t, err)
}

// ---------- service ----------

func TestServiceCreate(t *testing.T) {
	db := database.NewTestDB()
	u := mkUser(t, db, "budi")
	svc := NewService(NewRepository(db))

	first, err := svc.Create(u.ID, PostRequest{Title: "Belajar Go Gin", Content: "isi artikel", Category: "teknologi"})
	assert.NoError(t, err)
	assert.Equal(t, "belajar-go-gin", first.Slug)
	assert.Equal(t, "Teknologi", first.Category)
	assert.Equal(t, "isi artikel", first.Excerpt)
	assert.False(t, first.IsPublished)
	assert.Equal(t, 1, first.ReadingTime)
	assert.Equal(t, u.ID, first.UserID)

	second, err := svc.Create(u.ID, PostRequest{Title: "Belajar Go Gin", Content: "lain", Excerpt: "ringkas"})
	assert.NoError(t, err)
	assert.Equal(t, "belajar-go-gin-2", second.Slug)
	assert.Equal(t, "Umum", second.Category)
	assert.Equal(t, "ringkas", second.Excerpt)
}

func TestServiceCreateErrors(t *testing.T) {
	db := database.NewTestDB()
	svc, stub := newStubSvc(db)

	_, err := svc.Create(1, PostRequest{Title: "a", Content: "b", Category: "Salah"})
	assert.ErrorIs(t, err, ErrInvalidCategory)

	stub.fail["SlugExists"] = errBoom
	_, err = svc.Create(1, PostRequest{Title: "a", Content: "b"})
	assert.ErrorIs(t, err, errBoom)

	delete(stub.fail, "SlugExists")
	stub.fail["Create"] = errBoom
	_, err = svc.Create(1, PostRequest{Title: "a", Content: "b"})
	assert.ErrorIs(t, err, errBoom)
}

func TestServiceUpdate(t *testing.T) {
	db := database.NewTestDB()
	owner := mkUser(t, db, "andi")
	other := mkUser(t, db, "budi")
	svc, stub := newStubSvc(db)

	created, err := svc.Create(owner.ID, PostRequest{Title: "Judul Lama", Content: "isi lama", Category: "Bisnis"})
	assert.NoError(t, err)

	req := PostRequest{Title: "Judul Baru", Content: "isi baru", Excerpt: "ringkas baru", Category: "Opini", CoverImagePath: "/cover.png"}
	updated, err := svc.Update(owner.ID, created.ID, req)
	assert.NoError(t, err)
	assert.Equal(t, "Judul Baru", updated.Title)
	assert.Equal(t, "judul-lama", updated.Slug)
	assert.Equal(t, "Opini", updated.Category)
	assert.Equal(t, "isi baru", updated.Content)
	assert.Equal(t, "/cover.png", updated.CoverImagePath)
	assert.NotNil(t, updated.Author)

	_, err = svc.Update(other.ID, created.ID, req)
	assert.ErrorIs(t, err, ErrPostForbidden)

	_, err = svc.Update(owner.ID, 9999, req)
	assert.ErrorIs(t, err, ErrPostNotFound)

	_, err = svc.Update(owner.ID, created.ID, PostRequest{Title: "x", Content: "y", Category: "Salah"})
	assert.ErrorIs(t, err, ErrInvalidCategory)

	stub.fail["Update"] = errBoom
	_, err = svc.Update(owner.ID, created.ID, req)
	assert.ErrorIs(t, err, errBoom)
	delete(stub.fail, "Update")

	stub.fail["FindByID"] = errBoom
	_, err = svc.Update(owner.ID, created.ID, req)
	assert.ErrorIs(t, err, errBoom)
}

func TestServiceTogglePublish(t *testing.T) {
	db := database.NewTestDB()
	owner := mkUser(t, db, "andi")
	other := mkUser(t, db, "budi")
	svc, stub := newStubSvc(db)

	created, _ := svc.Create(owner.ID, PostRequest{Title: "Artikel", Content: "isi"})

	published, err := svc.TogglePublish(owner.ID, created.ID)
	assert.NoError(t, err)
	assert.True(t, published.IsPublished)
	assert.NotNil(t, published.PublishedAt)

	draft, err := svc.TogglePublish(owner.ID, created.ID)
	assert.NoError(t, err)
	assert.False(t, draft.IsPublished)
	assert.Nil(t, draft.PublishedAt)

	_, err = svc.TogglePublish(other.ID, created.ID)
	assert.ErrorIs(t, err, ErrPostForbidden)

	stub.fail["Update"] = errBoom
	_, err = svc.TogglePublish(owner.ID, created.ID)
	assert.ErrorIs(t, err, errBoom)
}

func TestServiceDelete(t *testing.T) {
	db := database.NewTestDB()
	owner := mkUser(t, db, "andi")
	other := mkUser(t, db, "budi")
	svc, stub := newStubSvc(db)

	created, _ := svc.Create(owner.ID, PostRequest{Title: "Artikel", Content: "isi"})

	assert.ErrorIs(t, svc.Delete(other.ID, created.ID), ErrPostForbidden)

	stub.fail["Delete"] = errBoom
	assert.ErrorIs(t, svc.Delete(owner.ID, created.ID), errBoom)
	delete(stub.fail, "Delete")

	assert.NoError(t, svc.Delete(owner.ID, created.ID))
	_, err := svc.GetMine(owner.ID, created.ID)
	assert.ErrorIs(t, err, ErrPostNotFound)
}

func TestServiceReadFlow(t *testing.T) {
	db := database.NewTestDB()
	a := mkUser(t, db, "andi")
	b := mkUser(t, db, "budi")
	mkUser(t, db, "citra")
	mkPost(t, db, a.ID, "Go Dasar", "go-dasar", "Teknologi", true)
	mkPost(t, db, a.ID, "Draf Andi", "draf-andi", "Bisnis", false)
	mkPost(t, db, b.ID, "Belajar Gizi", "belajar-gizi", "Gaya Hidup", true)
	svc := NewService(NewRepository(db))

	assert.Equal(t, Categories, svc.Categories())

	list, err := svc.ListPublic(ListFilter{})
	assert.NoError(t, err)
	assert.Len(t, list, 2)

	list, err = svc.ListPublic(ListFilter{Query: "DASAR"})
	assert.NoError(t, err)
	assert.Len(t, list, 1)

	pub, err := svc.GetPublic("go-dasar")
	assert.NoError(t, err)
	assert.Equal(t, "andi", pub.Author.Username)
	_, err = svc.GetPublic("draf-andi")
	assert.ErrorIs(t, err, ErrPostNotFound)

	mine, err := svc.ListMine(a.ID)
	assert.NoError(t, err)
	assert.Len(t, mine, 2)

	one, err := svc.GetMine(a.ID, mine[0].ID)
	assert.NoError(t, err)
	assert.Equal(t, mine[0].ID, one.ID)

	stats, err := svc.Stats(a.ID)
	assert.NoError(t, err)
	assert.Equal(t, StatsResponse{
		Total: 2, Published: 1, Draft: 1,
		TotalArticles: 2, TotalSelesai: 1, BelumSelesai: 1,
	}, *stats)

	users, err := svc.AdminListUsers()
	assert.NoError(t, err)
	assert.Len(t, users, 3)
	assert.Equal(t, int64(2), users[0].TotalPosts)
	assert.Equal(t, int64(1), users[0].TotalPublished)
	assert.Equal(t, int64(0), users[2].TotalPosts)

	all, err := svc.AdminListPosts()
	assert.NoError(t, err)
	assert.Len(t, all, 3)

	userPosts, err := svc.AdminListUserPosts(a.ID)
	assert.NoError(t, err)
	assert.Len(t, userPosts, 2)

	none, err := svc.AdminListUserPosts(9999)
	assert.NoError(t, err)
	assert.Empty(t, none)
}

func TestServiceReadErrors(t *testing.T) {
	db := database.NewTestDB()
	u := mkUser(t, db, "andi")
	svc, stub := newStubSvc(db)

	stub.fail["ListPublished"] = errBoom
	_, err := svc.ListPublic(ListFilter{})
	assert.ErrorIs(t, err, errBoom)

	stub.fail["FindPublishedBySlug"] = errBoom
	_, err = svc.GetPublic("x")
	assert.ErrorIs(t, err, errBoom)

	stub.fail["ListByUser"] = errBoom
	_, err = svc.ListMine(u.ID)
	assert.ErrorIs(t, err, errBoom)
	_, err = svc.AdminListUserPosts(u.ID)
	assert.ErrorIs(t, err, errBoom)

	stub.fail["FindByID"] = errBoom
	_, err = svc.GetMine(u.ID, 1)
	assert.ErrorIs(t, err, errBoom)

	stub.fail["CountByUser"] = errBoom
	_, err = svc.Stats(u.ID)
	assert.ErrorIs(t, err, errBoom)

	stub.fail["ListUsers"] = errBoom
	_, err = svc.AdminListUsers()
	assert.ErrorIs(t, err, errBoom)
	delete(stub.fail, "ListUsers")

	stub.fail["CountPerUser"] = errBoom
	_, err = svc.AdminListUsers()
	assert.ErrorIs(t, err, errBoom)

	stub.fail["ListAll"] = errBoom
	_, err = svc.AdminListPosts()
	assert.ErrorIs(t, err, errBoom)
}

// ---------- handler ----------

func testAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if id := c.GetHeader("X-User-ID"); id != "" {
			n, _ := strconv.ParseUint(id, 10, 64)
			c.Set(middleware.UserContextKey, middleware.AuthUser{ID: uint(n), Role: "member"})
		}
		c.Next()
	}
}

func handlerSetup(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	db := database.NewTestDB()
	h := NewHandler(NewService(NewRepository(db)))

	r := gin.New()
	r.Use(testAuth())
	r.GET("/public/categories", h.Categories)
	r.GET("/public/posts", h.ListPublic)
	r.GET("/public/posts/:slug", h.GetPublic)
	r.GET("/posts", h.ListMine)
	r.POST("/posts", h.Create)
	r.GET("/posts/stats", h.Stats)
	r.GET("/posts/:id", h.Get)
	r.PUT("/posts/:id", h.Update)
	r.PATCH("/posts/:id/toggle-publish", h.TogglePublish)
	r.DELETE("/posts/:id", h.Delete)
	r.GET("/admin/users", h.AdminUsers)
	r.GET("/admin/posts", h.AdminPosts)
	r.GET("/admin/users/:id/posts", h.AdminUserPosts)
	return r, db
}

func call(r http.Handler, method, path, body, userID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if userID != "" {
		req.Header.Set("X-User-ID", userID)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body
}

func uid(u *database.User) string {
	return strconv.Itoa(int(u.ID))
}

func TestHandlerAuthorFlow(t *testing.T) {
	r, db := handlerSetup(t)
	owner := mkUser(t, db, "andi")
	other := mkUser(t, db, "budi")

	assert.Equal(t, http.StatusOK, call(r, http.MethodGet, "/public/categories", "", "").Code)

	// create
	w := call(r, http.MethodPost, "/posts", `bukan json`, uid(owner))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	w = call(r, http.MethodPost, "/posts", `{"title":"","content":""}`, uid(owner))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	w = call(r, http.MethodPost, "/posts", `{"title":"Halo Dunia","content":"isi","category":"Salah"}`, uid(owner))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = call(r, http.MethodPost, "/posts", `{"title":"Halo Dunia","content":"isi artikel","category":"Teknologi"}`, uid(owner))
	assert.Equal(t, http.StatusCreated, w.Code)
	data := decode(t, w)["data"].(map[string]interface{})
	id := strconv.Itoa(int(data["id"].(float64)))
	assert.Equal(t, "halo-dunia", data["slug"])

	// belum terbit -> tidak tampil di publik
	w = call(r, http.MethodGet, "/public/posts", "", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Len(t, decode(t, w)["data"], 0)
	assert.Equal(t, http.StatusNotFound, call(r, http.MethodGet, "/public/posts/halo-dunia", "", "").Code)

	// baca milik sendiri / milik orang lain (403)
	assert.Equal(t, http.StatusOK, call(r, http.MethodGet, "/posts/"+id, "", uid(owner)).Code)
	assert.Equal(t, http.StatusForbidden, call(r, http.MethodGet, "/posts/"+id, "", uid(other)).Code)
	assert.Equal(t, http.StatusOK, call(r, http.MethodGet, "/posts", "", uid(owner)).Code)

	// update
	assert.Equal(t, http.StatusBadRequest, call(r, http.MethodPut, "/posts/"+id, `bukan json`, uid(owner)).Code)
	w = call(r, http.MethodPut, "/posts/"+id, `{"title":"Halo Dunia Baru","content":"isi baru"}`, uid(owner))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Halo Dunia Baru", decode(t, w)["data"].(map[string]interface{})["title"])
	assert.Equal(t, http.StatusForbidden, call(r, http.MethodPut, "/posts/"+id, `{"title":"x","content":"y"}`, uid(other)).Code)

	// toggle -> terbit
	w = call(r, http.MethodPatch, "/posts/"+id+"/toggle-publish", "", uid(owner))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, true, decode(t, w)["data"].(map[string]interface{})["is_published"])
	assert.Equal(t, http.StatusForbidden, call(r, http.MethodPatch, "/posts/"+id+"/toggle-publish", "", uid(other)).Code)

	// statistik
	w = call(r, http.MethodGet, "/posts/stats", "", uid(owner))
	assert.Equal(t, http.StatusOK, w.Code)
	stats := decode(t, w)["data"].(map[string]interface{})
	assert.Equal(t, float64(1), stats["total"])
	assert.Equal(t, float64(1), stats["published"])
	assert.Equal(t, float64(0), stats["draft"])
	assert.Equal(t, float64(1), stats["total_articles"])
	assert.Equal(t, float64(1), stats["total_selesai"])
	assert.Equal(t, float64(0), stats["belum_selesai"])

	// publik setelah terbit
	w = call(r, http.MethodGet, "/public/posts?q=halo&category=Umum", "", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Len(t, decode(t, w)["data"], 1)
	w = call(r, http.MethodGet, "/public/posts/halo-dunia", "", "")
	assert.Equal(t, http.StatusOK, w.Code)
	pub := decode(t, w)["data"].(map[string]interface{})
	assert.Equal(t, "andi", pub["author"].(map[string]interface{})["username"])

	// admin
	w = call(r, http.MethodGet, "/admin/users", "", uid(owner))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Len(t, decode(t, w)["data"], 2)
	w = call(r, http.MethodGet, "/admin/posts", "", uid(owner))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Len(t, decode(t, w)["data"], 1)
	w = call(r, http.MethodGet, "/admin/users/"+uid(owner)+"/posts", "", uid(owner))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Len(t, decode(t, w)["data"], 1)
	w = call(r, http.MethodGet, "/admin/users/9999/posts", "", uid(owner))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Len(t, decode(t, w)["data"], 0)

	// hapus
	assert.Equal(t, http.StatusForbidden, call(r, http.MethodDelete, "/posts/"+id, "", uid(other)).Code)
	assert.Equal(t, http.StatusOK, call(r, http.MethodDelete, "/posts/"+id, "", uid(owner)).Code)
	assert.Equal(t, http.StatusNotFound, call(r, http.MethodGet, "/posts/"+id, "", uid(owner)).Code)
}

func TestHandlerUnauthorized(t *testing.T) {
	r, _ := handlerSetup(t)

	cases := []struct{ method, path string }{
		{http.MethodGet, "/posts"},
		{http.MethodPost, "/posts"},
		{http.MethodGet, "/posts/stats"},
		{http.MethodGet, "/posts/1"},
		{http.MethodPut, "/posts/1"},
		{http.MethodPatch, "/posts/1/toggle-publish"},
		{http.MethodDelete, "/posts/1"},
	}
	for _, tc := range cases {
		w := call(r, tc.method, tc.path, `{"title":"a","content":"b"}`, "")
		assert.Equal(t, http.StatusUnauthorized, w.Code, tc.method+" "+tc.path)
	}
}

func TestHandlerInvalidID(t *testing.T) {
	r, _ := handlerSetup(t)

	cases := []struct{ method, path string }{
		{http.MethodGet, "/posts/abc"},
		{http.MethodPut, "/posts/abc"},
		{http.MethodPatch, "/posts/abc/toggle-publish"},
		{http.MethodDelete, "/posts/abc"},
		{http.MethodGet, "/posts/0"},
		{http.MethodGet, "/admin/users/abc/posts"},
	}
	for _, tc := range cases {
		w := call(r, tc.method, tc.path, `{"title":"a","content":"b"}`, "1")
		assert.Equal(t, http.StatusBadRequest, w.Code, tc.method+" "+tc.path)
	}
}

func TestHandlerServerErrors(t *testing.T) {
	r, db := handlerSetup(t)
	u := mkUser(t, db, "andi")
	assert.NoError(t, db.Migrator().DropTable(&database.Post{}))

	for _, path := range []string{"/public/posts", "/posts", "/posts/stats", "/admin/users", "/admin/posts", "/admin/users/1/posts"} {
		w := call(r, http.MethodGet, path, "", uid(u))
		assert.Equal(t, http.StatusInternalServerError, w.Code, path)
		assert.Equal(t, "error", decode(t, w)["status"])
	}
}

func TestRespondErrorDefault(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	respondError(c, errBoom)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "error", decode(t, w)["status"])
}

func TestHandlerUpdateChecksOwnershipBeforeBody(t *testing.T) {
	r, db := handlerSetup(t)
	owner := mkUser(t, db, "andi")
	other := mkUser(t, db, "budi")
	p := mkPost(t, db, owner.ID, "Judul", "judul", "Umum", false)

	// body kosong tetap menghasilkan 404 / 403, bukan 400
	assert.Equal(t, http.StatusNotFound, call(r, http.MethodPut, "/posts/9999", `{}`, uid(owner)).Code)
	assert.Equal(t, http.StatusForbidden, call(r, http.MethodPut, "/posts/"+strconv.Itoa(int(p.ID)), `{}`, uid(other)).Code)
}

func TestHandlerCreateAndUpdateWithPublishFlag(t *testing.T) {
	r, db := handlerSetup(t)
	owner := mkUser(t, db, "andi")

	// dibuat langsung terbit
	w := call(r, http.MethodPost, "/posts", `{"title":"Langsung Terbit","content":"isi","is_published":true}`, uid(owner))
	assert.Equal(t, http.StatusCreated, w.Code)
	data := decode(t, w)["data"].(map[string]interface{})
	id := strconv.Itoa(int(data["id"].(float64)))
	assert.Equal(t, true, data["is_published"])
	assert.NotNil(t, data["published_at"])

	// update is_published true pada artikel yang sudah terbit: tetap terbit
	w = call(r, http.MethodPut, "/posts/"+id, `{"title":"Langsung Terbit","content":"isi","is_published":true}`, uid(owner))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, true, decode(t, w)["data"].(map[string]interface{})["is_published"])

	// update is_published false: kembali menjadi draf
	w = call(r, http.MethodPut, "/posts/"+id, `{"title":"Langsung Terbit","content":"isi","is_published":false}`, uid(owner))
	assert.Equal(t, http.StatusOK, w.Code)
	data = decode(t, w)["data"].(map[string]interface{})
	assert.Equal(t, false, data["is_published"])
	assert.Nil(t, data["published_at"])

	// update is_published true pada draf: terbit dengan published_at
	w = call(r, http.MethodPut, "/posts/"+id, `{"title":"Langsung Terbit","content":"isi","is_published":true}`, uid(owner))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotNil(t, decode(t, w)["data"].(map[string]interface{})["published_at"])

	// kategori tidak valid pada PUT -> 400
	w = call(r, http.MethodPut, "/posts/"+id, `{"title":"x","content":"y","category":"Salah"}`, uid(owner))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPublicResponsesHideUserID(t *testing.T) {
	r, db := handlerSetup(t)
	owner := mkUser(t, db, "andi")
	mkPost(t, db, owner.ID, "Judul Publik", "judul-publik", "Umum", true)

	w := call(r, http.MethodGet, "/public/posts", "", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "user_id")

	w = call(r, http.MethodGet, "/public/posts/judul-publik", "", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "user_id")

	// endpoint penulis tetap menyertakan user_id
	w = call(r, http.MethodGet, "/posts", "", uid(owner))
	assert.Contains(t, w.Body.String(), "user_id")
}