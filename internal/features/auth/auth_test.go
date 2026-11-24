package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"app/internal/database"
	"app/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const testSecret = "test-secret"

var errBoom = errors.New("boom")

func init() {
	gin.SetMode(gin.TestMode)
}

// ---------- helper ----------

func setup(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db := database.NewTestDB()
	return NewService(NewRepository(db), testSecret), db
}

func mustRegister(t *testing.T, svc *Service, username, email, password string) *database.User {
	t.Helper()
	user, err := svc.Register(RegisterRequest{Username: username, Email: email, Password: password})
	assert.NoError(t, err)
	return user
}

// stubRepo membungkus repository asli dan dapat dipaksa gagal per method.
type stubRepo struct {
	Repository
	errFindByID, errFindIdentity, errExistsUsername, errExistsEmail, errCreate, errUpdate error
}

func (s *stubRepo) FindByID(id uint) (*database.User, error) {
	if s.errFindByID != nil {
		return nil, s.errFindByID
	}
	return s.Repository.FindByID(id)
}

func (s *stubRepo) FindByIdentity(identity string) (*database.User, error) {
	if s.errFindIdentity != nil {
		return nil, s.errFindIdentity
	}
	return s.Repository.FindByIdentity(identity)
}

func (s *stubRepo) ExistsUsername(username string, excludeID uint) (bool, error) {
	if s.errExistsUsername != nil {
		return false, s.errExistsUsername
	}
	return s.Repository.ExistsUsername(username, excludeID)
}

func (s *stubRepo) ExistsEmail(email string, excludeID uint) (bool, error) {
	if s.errExistsEmail != nil {
		return false, s.errExistsEmail
	}
	return s.Repository.ExistsEmail(email, excludeID)
}

func (s *stubRepo) Create(user *database.User) error {
	if s.errCreate != nil {
		return s.errCreate
	}
	return s.Repository.Create(user)
}

func (s *stubRepo) Update(user *database.User) error {
	if s.errUpdate != nil {
		return s.errUpdate
	}
	return s.Repository.Update(user)
}

func setupStub(t *testing.T) (*Service, *stubRepo) {
	t.Helper()
	stub := &stubRepo{Repository: NewRepository(database.NewTestDB())}
	return NewService(stub, testSecret), stub
}

// ---------- repository ----------

func TestRepository(t *testing.T) {
	repo := NewRepository(database.NewTestDB())

	user := &database.User{Username: "budi", Email: "budi@x.com", Password: "hash", Role: "member"}
	assert.NoError(t, repo.Create(user))

	byID, err := repo.FindByID(user.ID)
	assert.NoError(t, err)
	assert.Equal(t, "budi", byID.Username)

	_, err = repo.FindByID(9999)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	byName, err := repo.FindByIdentity("budi")
	assert.NoError(t, err)
	assert.Equal(t, user.ID, byName.ID)

	byEmail, err := repo.FindByIdentity("BUDI@X.COM")
	assert.NoError(t, err)
	assert.Equal(t, user.ID, byEmail.ID)

	_, err = repo.FindByIdentity("tidak-ada")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	user.Username = "budi2"
	assert.NoError(t, repo.Update(user))

	exists, err := repo.ExistsUsername("budi2", 0)
	assert.NoError(t, err)
	assert.True(t, exists)

	exists, err = repo.ExistsUsername("budi2", user.ID)
	assert.NoError(t, err)
	assert.False(t, exists)

	exists, err = repo.ExistsEmail("budi@x.com", 0)
	assert.NoError(t, err)
	assert.True(t, exists)

	exists, err = repo.ExistsEmail("budi@x.com", user.ID)
	assert.NoError(t, err)
	assert.False(t, exists)
}

// ---------- service ----------

func TestLookupError(t *testing.T) {
	assert.Equal(t, ErrUserNotFound, lookupError(gorm.ErrRecordNotFound, ErrUserNotFound))
	assert.Equal(t, errBoom, lookupError(errBoom, ErrUserNotFound))
}

func TestRegisterSuccess(t *testing.T) {
	svc, _ := setup(t)

	user, err := svc.Register(RegisterRequest{Username: "  budi ", Email: " Budi@Mail.com ", Password: "rahasia123"})

	assert.NoError(t, err)
	assert.Equal(t, "budi", user.Username)
	assert.Equal(t, "budi@mail.com", user.Email)
	assert.Equal(t, "member", user.Role)
	assert.NotEqual(t, "rahasia123", user.Password)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("rahasia123")))
}

func TestRegisterDuplicates(t *testing.T) {
	svc, _ := setup(t)
	mustRegister(t, svc, "budi", "budi@mail.com", "rahasia123")

	_, err := svc.Register(RegisterRequest{Username: "budi", Email: "lain@mail.com", Password: "rahasia123"})
	assert.ErrorIs(t, err, ErrUsernameTaken)

	_, err = svc.Register(RegisterRequest{Username: "andi", Email: "budi@mail.com", Password: "rahasia123"})
	assert.ErrorIs(t, err, ErrEmailTaken)
}

func TestRegisterErrors(t *testing.T) {
	req := RegisterRequest{Username: "budi", Email: "budi@mail.com", Password: "rahasia123"}

	svc, stub := setupStub(t)
	stub.errExistsUsername = errBoom
	_, err := svc.Register(req)
	assert.ErrorIs(t, err, errBoom)

	svc, stub = setupStub(t)
	stub.errExistsEmail = errBoom
	_, err = svc.Register(req)
	assert.ErrorIs(t, err, errBoom)

	svc, stub = setupStub(t)
	stub.errCreate = errBoom
	_, err = svc.Register(req)
	assert.ErrorIs(t, err, errBoom)

	// bcrypt menolak password lebih dari 72 byte
	svc, _ = setupStub(t)
	_, err = svc.Register(RegisterRequest{Username: "budi", Email: "budi@mail.com", Password: strings.Repeat("a", 73)})
	assert.Error(t, err)
}

func TestLogin(t *testing.T) {
	svc, _ := setup(t)
	user := mustRegister(t, svc, "budi", "budi@mail.com", "rahasia123")

	result, err := svc.Login(LoginRequest{Username: "budi", Password: "rahasia123"})
	assert.NoError(t, err)
	assert.Equal(t, "budi", result.User.Username)
	assert.NotEmpty(t, result.Token)

	parsed, err := jwt.Parse(result.Token, func(*jwt.Token) (interface{}, error) { return []byte(testSecret), nil })
	assert.NoError(t, err)
	claims := parsed.Claims.(jwt.MapClaims)
	assert.Equal(t, float64(user.ID), claims["user_id"])
	assert.Equal(t, "budi", claims["username"])
	assert.Equal(t, "member", claims["role"])

	// login memakai email (huruf besar diabaikan)
	_, err = svc.Login(LoginRequest{Username: "BUDI@MAIL.COM", Password: "rahasia123"})
	assert.NoError(t, err)
}

func TestLoginFailures(t *testing.T) {
	svc, _ := setup(t)
	mustRegister(t, svc, "budi", "budi@mail.com", "rahasia123")

	_, err := svc.Login(LoginRequest{Username: "tidak-ada", Password: "rahasia123"})
	assert.ErrorIs(t, err, ErrInvalidCredentials)

	_, err = svc.Login(LoginRequest{Username: "budi", Password: "salah-total"})
	assert.ErrorIs(t, err, ErrInvalidCredentials)

	svcStub, stub := setupStub(t)
	stub.errFindIdentity = errBoom
	_, err = svcStub.Login(LoginRequest{Username: "budi", Password: "rahasia123"})
	assert.ErrorIs(t, err, errBoom)
}

func TestGetProfile(t *testing.T) {
	svc, _ := setup(t)
	user := mustRegister(t, svc, "budi", "budi@mail.com", "rahasia123")

	found, err := svc.GetProfile(user.ID)
	assert.NoError(t, err)
	assert.Equal(t, "budi", found.Username)

	_, err = svc.GetProfile(9999)
	assert.ErrorIs(t, err, ErrUserNotFound)

	svcStub, stub := setupStub(t)
	stub.errFindByID = errBoom
	_, err = svcStub.GetProfile(1)
	assert.ErrorIs(t, err, errBoom)
}

func TestUpdateProfileSuccess(t *testing.T) {
	svc, _ := setup(t)
	user := mustRegister(t, svc, "budi", "budi@mail.com", "rahasia123")

	// tanpa password, username & email sendiri tetap boleh dipakai
	updated, err := svc.UpdateProfile(user.ID, UpdateProfileRequest{Username: "budi", Email: "BUDI@mail.com"})
	assert.NoError(t, err)
	assert.Equal(t, "budi@mail.com", updated.Email)
	_, err = svc.Login(LoginRequest{Username: "budi", Password: "rahasia123"})
	assert.NoError(t, err)

	// dengan password baru
	updated, err = svc.UpdateProfile(user.ID, UpdateProfileRequest{Username: "budi baru", Email: "baru@mail.com", Password: "passwordbaru"})
	assert.NoError(t, err)
	assert.Equal(t, "budi baru", updated.Username)
	_, err = svc.Login(LoginRequest{Username: "budi baru", Password: "passwordbaru"})
	assert.NoError(t, err)
	_, err = svc.Login(LoginRequest{Username: "budi baru", Password: "rahasia123"})
	assert.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestUpdateProfileConflicts(t *testing.T) {
	svc, _ := setup(t)
	mustRegister(t, svc, "budi", "budi@mail.com", "rahasia123")
	andi := mustRegister(t, svc, "andi", "andi@mail.com", "rahasia123")

	_, err := svc.UpdateProfile(andi.ID, UpdateProfileRequest{Username: "budi", Email: "andi@mail.com"})
	assert.ErrorIs(t, err, ErrUsernameTaken)

	_, err = svc.UpdateProfile(andi.ID, UpdateProfileRequest{Username: "andi", Email: "budi@mail.com"})
	assert.ErrorIs(t, err, ErrEmailTaken)
}

func TestUpdateProfileErrors(t *testing.T) {
	req := UpdateProfileRequest{Username: "budi", Email: "budi@mail.com"}

	svc, _ := setup(t)
	_, err := svc.UpdateProfile(9999, req)
	assert.ErrorIs(t, err, ErrUserNotFound)

	svc, stub := setupStub(t)
	stub.errFindByID = errBoom
	_, err = svc.UpdateProfile(1, req)
	assert.ErrorIs(t, err, errBoom)

	svc, stub = setupStub(t)
	user := mustRegister(t, svc, "budi", "budi@mail.com", "rahasia123")
	stub.errExistsUsername = errBoom
	_, err = svc.UpdateProfile(user.ID, req)
	assert.ErrorIs(t, err, errBoom)

	svc, stub = setupStub(t)
	user = mustRegister(t, svc, "budi", "budi@mail.com", "rahasia123")
	stub.errExistsEmail = errBoom
	_, err = svc.UpdateProfile(user.ID, req)
	assert.ErrorIs(t, err, errBoom)

	svc, stub = setupStub(t)
	user = mustRegister(t, svc, "budi", "budi@mail.com", "rahasia123")
	stub.errUpdate = errBoom
	_, err = svc.UpdateProfile(user.ID, req)
	assert.ErrorIs(t, err, errBoom)

	svc, _ = setupStub(t)
	user = mustRegister(t, svc, "budi", "budi@mail.com", "rahasia123")
	_, err = svc.UpdateProfile(user.ID, UpdateProfileRequest{Username: "budi", Email: "budi@mail.com", Password: strings.Repeat("a", 73)})
	assert.Error(t, err)
}

// ---------- handler ----------

func newRouter(h *Handler) *gin.Engine {
	r := gin.New()
	r.POST("/register", h.Register)
	r.POST("/login", h.Login)

	protected := r.Group("", middleware.Auth(testSecret))
	protected.POST("/logout", h.Logout)
	protected.GET("/profile", h.Profile)
	protected.PUT("/profile", h.UpdateProfile)

	// rute tanpa middleware Auth: context tidak berisi user
	r.GET("/open-profile", h.Profile)
	r.PUT("/open-profile", h.UpdateProfile)
	return r
}

func call(r http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
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

func handlerSetup(t *testing.T) (*gin.Engine, *Service, *gorm.DB) {
	t.Helper()
	svc, db := setup(t)
	return newRouter(NewHandler(svc)), svc, db
}

func loginToken(t *testing.T, svc *Service, username, password string) string {
	t.Helper()
	result, err := svc.Login(LoginRequest{Username: username, Password: password})
	assert.NoError(t, err)
	return result.Token
}

func TestHandlerRegister(t *testing.T) {
	r, _, _ := handlerSetup(t)

	w := call(r, http.MethodPost, "/register", `bukan json`, "")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = call(r, http.MethodPost, "/register", `{"username":"budi","email":"budi@mail.com","password":"123"}`, "")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	body := `{"username":"budi","email":"budi@mail.com","password":"rahasia123"}`
	w = call(r, http.MethodPost, "/register", body, "")
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "success", decode(t, w)["status"])
	assert.NotEmpty(t, decode(t, w)["data"].(map[string]interface{})["token"])

	w = call(r, http.MethodPost, "/register", body, "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "fail", decode(t, w)["status"])
}

func TestUpdateProfileKeepsExistingWhenEmpty(t *testing.T) {
	svc, _ := setup(t)
	user := mustRegister(t, svc, "budi", "budi@mail.com", "rahasia123")

	updated, err := svc.UpdateProfile(user.ID, UpdateProfileRequest{Password: "passwordbaru"})

	assert.NoError(t, err)
	assert.Equal(t, "budi", updated.Username)
	assert.Equal(t, "budi@mail.com", updated.Email)
	_, err = svc.Login(LoginRequest{Username: "budi", Password: "passwordbaru"})
	assert.NoError(t, err)
}

func TestHandlerUpdateProfilePartial(t *testing.T) {
	r, svc, _ := handlerSetup(t)
	mustRegister(t, svc, "budi", "budi@mail.com", "rahasia123")
	token := loginToken(t, svc, "budi", "rahasia123")

	w := call(r, http.MethodPut, "/profile", `{"email":"baru@mail.com","password":"passwordbaru"}`, token)

	assert.Equal(t, http.StatusOK, w.Code)
	_, err := svc.Login(LoginRequest{Username: "budi", Password: "passwordbaru"})
	assert.NoError(t, err)
}

func TestHandlerLogin(t *testing.T) {
	r, svc, _ := handlerSetup(t)
	mustRegister(t, svc, "budi", "budi@mail.com", "rahasia123")

	w := call(r, http.MethodPost, "/login", `{"username":"budi"}`, "")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = call(r, http.MethodPost, "/login", `{"username":"budi","password":"salah"}`, "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	w = call(r, http.MethodPost, "/login", `{"username":"budi","password":"rahasia123"}`, "")
	assert.Equal(t, http.StatusOK, w.Code)
	data := decode(t, w)["data"].(map[string]interface{})
	assert.NotEmpty(t, data["token"])
	assert.Equal(t, "budi", data["user"].(map[string]interface{})["username"])
}

func TestHandlerLogout(t *testing.T) {
	r, svc, _ := handlerSetup(t)
	mustRegister(t, svc, "budi", "budi@mail.com", "rahasia123")
	token := loginToken(t, svc, "budi", "rahasia123")

	w := call(r, http.MethodPost, "/logout", "", token)
	assert.Equal(t, http.StatusOK, w.Code)

	w = call(r, http.MethodPost, "/logout", "", "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandlerProfile(t *testing.T) {
	r, svc, db := handlerSetup(t)
	user := mustRegister(t, svc, "budi", "budi@mail.com", "rahasia123")
	token := loginToken(t, svc, "budi", "rahasia123")

	w := call(r, http.MethodGet, "/profile", "", token)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "budi", decode(t, w)["data"].(map[string]interface{})["username"])

	w = call(r, http.MethodGet, "/open-profile", "", "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	db.Delete(&database.User{}, user.ID)
	w = call(r, http.MethodGet, "/profile", "", token)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlerUpdateProfile(t *testing.T) {
	r, svc, db := handlerSetup(t)
	mustRegister(t, svc, "andi", "andi@mail.com", "rahasia123")
	user := mustRegister(t, svc, "budi", "budi@mail.com", "rahasia123")
	token := loginToken(t, svc, "budi", "rahasia123")

	w := call(r, http.MethodPut, "/open-profile", `{"username":"budi","email":"budi@mail.com"}`, "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	w = call(r, http.MethodPut, "/profile", `bukan json`, token)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = call(r, http.MethodPut, "/profile", `{"username":"budi baru","email":"baru@mail.com"}`, token)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "budi baru", decode(t, w)["data"].(map[string]interface{})["username"])

	w = call(r, http.MethodPut, "/profile", `{"username":"andi","email":"baru@mail.com"}`, token)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	db.Delete(&database.User{}, user.ID)
	w = call(r, http.MethodPut, "/profile", `{"username":"budi baru","email":"baru@mail.com"}`, token)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRespondErrorDefault(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	respondError(c, errBoom)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "error", decode(t, w)["status"])
}