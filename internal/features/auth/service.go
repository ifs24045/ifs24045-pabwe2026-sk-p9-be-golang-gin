package auth

import (
	"errors"
	"strings"
	"time"

	"app/internal/database"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrUsernameTaken      = errors.New("Username sudah digunakan")
	ErrEmailTaken         = errors.New("Email sudah digunakan")
	ErrInvalidCredentials = errors.New("Username atau password salah")
	ErrUserNotFound       = errors.New("Pengguna tidak ditemukan")
)

const tokenTTL = 24 * time.Hour

// Service berisi logika bisnis autentikasi.
type Service struct {
	repo   Repository
	secret string
}

// NewService membuat Service autentikasi dengan kunci penandatangan JWT.
func NewService(repo Repository, secret string) *Service {
	return &Service{repo: repo, secret: secret}
}

// lookupError mengubah "record tidak ditemukan" menjadi error domain.
func lookupError(err error, notFound error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return notFound
	}
	return err
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

// ensureUnique memastikan username dan email belum dipakai pengguna lain.
func (s *Service) ensureUnique(username, email string, excludeID uint) error {
	taken, err := s.repo.ExistsUsername(username, excludeID)
	if err != nil {
		return err
	}
	if taken {
		return ErrUsernameTaken
	}

	taken, err = s.repo.ExistsEmail(email, excludeID)
	if err != nil {
		return err
	}
	if taken {
		return ErrEmailTaken
	}
	return nil
}

// Register membuat akun penulis baru dengan role member.
func (s *Service) Register(req RegisterRequest) (*database.User, error) {
	username := strings.TrimSpace(req.Username)
	email := strings.ToLower(strings.TrimSpace(req.Email))

	if err := s.ensureUnique(username, email, 0); err != nil {
		return nil, err
	}

	hash, err := hashPassword(req.Password)
	if err != nil {
		return nil, err
	}

		role := "member"
	if strings.EqualFold(strings.TrimSpace(req.Role), "admin") {
		role = "admin"
	}

	user := &database.User{
		Username: username,
		Email:    email,
		Password: hash,
		Role:     role,
	}
	if err := s.repo.Create(user); err != nil {
		return nil, err
	}
	return user, nil
}

// GenerateToken menerbitkan token JWT HS256 berlaku 24 jam.
func (s *Service) GenerateToken(user *database.User) string {
	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"role":     user.Role,
		"exp":      time.Now().Add(tokenTTL).Unix(),
	}
	// Penandatanganan HS256 dengan kunci []byte tidak pernah menghasilkan error.
	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.secret))
	return token
}

// Login memverifikasi kredensial lalu menerbitkan token.
func (s *Service) Login(req LoginRequest) (*AuthResponse, error) {
	user, err := s.repo.FindByIdentity(strings.TrimSpace(req.Username))
	if err != nil {
		return nil, lookupError(err, ErrInvalidCredentials)
	}

	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) != nil {
		return nil, ErrInvalidCredentials
	}

	return &AuthResponse{Token: s.GenerateToken(user), User: ToUserResponse(user)}, nil
}

// GetProfile mengambil data pengguna berdasarkan ID.
func (s *Service) GetProfile(id uint) (*database.User, error) {
	user, err := s.repo.FindByID(id)
	if err != nil {
		return nil, lookupError(err, ErrUserNotFound)
	}
	return user, nil
}

// UpdateProfile memperbarui username, email, dan (opsional) password.
func (s *Service) UpdateProfile(id uint, req UpdateProfileRequest) (*database.User, error) {
	user, err := s.repo.FindByID(id)
	if err != nil {
		return nil, lookupError(err, ErrUserNotFound)
	}

	username := strings.TrimSpace(req.Username)
	if username == "" {
		username = user.Username
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		email = user.Email
	}
	if err := s.ensureUnique(username, email, id); err != nil {
		return nil, err
	}

	user.Username = username
	user.Email = email
	if req.Password != "" {
		hash, err := hashPassword(req.Password)
		if err != nil {
			return nil, err
		}
		user.Password = hash
	}

	if err := s.repo.Update(user); err != nil {
		return nil, err
	}
	return user, nil
}