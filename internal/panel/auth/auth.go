package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lucasile/deft/internal/panel/db"
	"golang.org/x/crypto/bcrypt"
)

const (
	AdminRole       = "admin"
	SessionCookie   = "deft_session"
	sessionLifetime = 7 * 24 * time.Hour
)

type contextKey string

const userContextKey contextKey = "user"

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type Session struct {
	Token     string
	CSRFToken string
	ExpiresAt time.Time
}

type Service struct {
	db *db.DB
}

func NewService(database *db.DB) *Service {
	return &Service{db: database}
}

func (s *Service) RegisterFirstUser(username, password string) (*User, error) {
	username = strings.TrimSpace(username)
	if err := validateCredentials(username, password); err != nil {
		return nil, err
	}

	count, err := s.userCount()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, fmt.Errorf("registration is closed")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	userID, err := randomHex(16)
	if err != nil {
		return nil, err
	}

	_, err = s.db.Exec(
		"INSERT INTO users (id, username, password_hash, role, created_at) VALUES (?, ?, ?, ?, ?)",
		userID,
		username,
		string(hash),
		AdminRole,
		time.Now().Unix(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &User{ID: userID, Username: username, Role: AdminRole}, nil
}

func (s *Service) Login(username, password string) (*Session, *User, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, nil, fmt.Errorf("username and password are required")
	}

	var user User
	var passwordHash string
	err := s.db.QueryRow(
		"SELECT id, username, password_hash, role FROM users WHERE username = ?",
		username,
	).Scan(&user.ID, &user.Username, &passwordHash, &user.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, fmt.Errorf("invalid username or password")
	}
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return nil, nil, fmt.Errorf("invalid username or password")
	}

	session, err := s.createSession(user.ID)
	if err != nil {
		return nil, nil, err
	}

	return session, &user, nil
}

func (s *Service) AuthenticateSession(token string) (*User, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("session token is required")
	}

	tokenHash := hashToken(token)
	now := time.Now().Unix()

	var user User
	var expiresAt int64
	err := s.db.QueryRow(
		`SELECT users.id, users.username, users.role, sessions.expires_at
		 FROM sessions
		 JOIN users ON users.id = sessions.user_id
		 WHERE sessions.token_hash = ?`,
		tokenHash,
	).Scan(&user.ID, &user.Username, &user.Role, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("session not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}
	if expiresAt <= now {
		_ = s.DeleteSession(token)
		return nil, fmt.Errorf("session expired")
	}

	_, err = s.db.Exec("UPDATE sessions SET last_seen_at = ? WHERE token_hash = ?", now, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("failed to update session: %w", err)
	}

	return &user, nil
}

func (s *Service) ValidateCSRFToken(sessionToken, csrfToken string) error {
	sessionToken = strings.TrimSpace(sessionToken)
	csrfToken = strings.TrimSpace(csrfToken)
	if sessionToken == "" || csrfToken == "" {
		return fmt.Errorf("csrf token is required")
	}

	var storedToken string
	err := s.db.QueryRow(
		"SELECT csrf_token FROM sessions WHERE token_hash = ? AND expires_at > ?",
		hashToken(sessionToken),
		time.Now().Unix(),
	).Scan(&storedToken)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("session not found")
	}
	if err != nil {
		return fmt.Errorf("failed to load csrf token: %w", err)
	}
	if storedToken != csrfToken {
		return fmt.Errorf("invalid csrf token")
	}

	return nil
}

func (s *Service) CSRFToken(sessionToken string) (string, error) {
	sessionToken = strings.TrimSpace(sessionToken)
	if sessionToken == "" {
		return "", fmt.Errorf("session token is required")
	}

	var csrfToken string
	err := s.db.QueryRow(
		"SELECT csrf_token FROM sessions WHERE token_hash = ? AND expires_at > ?",
		hashToken(sessionToken),
		time.Now().Unix(),
	).Scan(&csrfToken)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("session not found")
	}
	if err != nil {
		return "", fmt.Errorf("failed to load csrf token: %w", err)
	}

	return csrfToken, nil
}

func (s *Service) DeleteSession(token string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	if _, err := s.db.Exec("DELETE FROM sessions WHERE token_hash = ?", hashToken(token)); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

func WithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func UserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(userContextKey).(*User)
	return user, ok
}

func (s *Service) createSession(userID string) (*Session, error) {
	token, err := randomHex(32)
	if err != nil {
		return nil, err
	}
	csrfToken, err := randomHex(32)
	if err != nil {
		return nil, err
	}
	sessionID, err := randomHex(16)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	expiresAt := now.Add(sessionLifetime)
	_, err = s.db.Exec(
		"INSERT INTO sessions (id, user_id, token_hash, csrf_token, created_at, expires_at, last_seen_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		sessionID,
		userID,
		hashToken(token),
		csrfToken,
		now.Unix(),
		expiresAt.Unix(),
		now.Unix(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &Session{Token: token, CSRFToken: csrfToken, ExpiresAt: expiresAt}, nil
}

func (s *Service) userCount() (int, error) {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return count, nil
}

func validateCredentials(username, password string) error {
	if len(username) < 3 || len(username) > 64 {
		return fmt.Errorf("username must be 3-64 characters")
	}
	if len(password) < 12 {
		return fmt.Errorf("password must be at least 12 characters")
	}
	return nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomHex(size int) (string, error) {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to create random value: %w", err)
	}
	return hex.EncodeToString(b), nil
}
