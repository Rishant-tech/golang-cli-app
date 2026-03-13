package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rishant-tech/golang-cli-app/internal/config"
	"github.com/rishant-tech/golang-cli-app/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// Sentinel errors returned by the auth service.
var (
	ErrUserExists         = errors.New("username already taken")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrAccountLocked      = errors.New("account locked due to too many failed attempts")
	ErrInvalidTOTP        = errors.New("invalid 2FA code")
	ErrTOTPNotEnabled     = errors.New("2FA is not enabled for this account")
)

// Service handles registration, login, and account management.
type Service struct {
	db  *pgxpool.Pool
	cfg *config.Config
}

// NewService creates a new auth service.
func NewService(db *pgxpool.Pool, cfg *config.Config) *Service {
	return &Service{db: db, cfg: cfg}
}

// Register creates a new user with a bcrypt-hashed password.
func (s *Service) Register(ctx context.Context, username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	_, err = s.db.Exec(ctx,
		`INSERT INTO users (username, password_hash) VALUES ($1, $2)`,
		username, string(hash),
	)
	if err != nil {
		// Postgres unique_violation error code
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrUserExists
		}
		return err
	}

	return nil
}

// Login authenticates a user and returns a new session on success.
// If the user has 2FA enabled, totpCode must be provided.
func (s *Service) Login(ctx context.Context, username, password, totpCode string) (*models.Session, *models.User, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	if user.IsLocked() {
		return nil, nil, fmt.Errorf("%w, try again after %s",
			ErrAccountLocked, user.LockedUntil.Format("15:04:05"))
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		s.incrementFailedAttempts(ctx, user)
		return nil, nil, ErrInvalidCredentials
	}

	if user.Is2FAEnabled() {
		if totpCode == "" || !ValidateTOTP(totpCode, *user.TOTPSecret) {
			return nil, nil, ErrInvalidTOTP
		}
	}

	s.resetFailedAttempts(ctx, user.ID)
	s.updateLastLogin(ctx, user.ID)

	session, err := s.createSession(ctx, user.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("create session: %w", err)
	}

	return session, user, nil
}

// NeedsTOTP reports whether the given username has 2FA enabled.
// Used by the CLI to decide whether to prompt for a TOTP code before calling Login.
func (s *Service) NeedsTOTP(ctx context.Context, username string) bool {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return false
	}
	return user.Is2FAEnabled()
}

// getUserByUsername fetches a full user record by username.
func (s *Service) getUserByUsername(ctx context.Context, username string) (*models.User, error) {
	u := &models.User{}
	err := s.db.QueryRow(ctx, `
		SELECT id, username, password_hash, totp_secret, totp_enabled,
		       failed_attempts, locked_until, last_login_at, created_at
		FROM users
		WHERE username = $1
	`, username).Scan(
		&u.ID, &u.Username, &u.PasswordHash, &u.TOTPSecret,
		&u.TOTPEnabled, &u.FailedAttempts, &u.LockedUntil,
		&u.LastLoginAt, &u.CreatedAt,
	)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return u, nil
}

// incrementFailedAttempts increments the counter and locks the account if the threshold is reached.
func (s *Service) incrementFailedAttempts(ctx context.Context, user *models.User) {
	next := user.FailedAttempts + 1
	if next >= s.cfg.MaxFailedAttempts {
		lockedUntil := time.Now().Add(s.cfg.LockoutDuration)
		s.db.Exec(ctx,
			`UPDATE users SET failed_attempts = $1, locked_until = $2 WHERE id = $3`,
			next, lockedUntil, user.ID,
		)
		return
	}
	s.db.Exec(ctx,
		`UPDATE users SET failed_attempts = $1 WHERE id = $2`,
		next, user.ID,
	)
}

// resetFailedAttempts clears the lockout state after a successful login.
func (s *Service) resetFailedAttempts(ctx context.Context, userID string) {
	s.db.Exec(ctx,
		`UPDATE users SET failed_attempts = 0, locked_until = NULL WHERE id = $1`, userID,
	)
}

// updateLastLogin sets last_login_at to the current time.
func (s *Service) updateLastLogin(ctx context.Context, userID string) {
	s.db.Exec(ctx,
		`UPDATE users SET last_login_at = NOW() WHERE id = $1`, userID,
	)
}

// createSession inserts a new session row and returns it.
func (s *Service) createSession(ctx context.Context, userID string) (*models.Session, error) {
	sess := &models.Session{}
	expiresAt := time.Now().Add(s.cfg.SessionTimeout)
	err := s.db.QueryRow(ctx, `
		INSERT INTO sessions (user_id, expires_at)
		VALUES ($1, $2)
		RETURNING id, user_id, expires_at, created_at
	`, userID, expiresAt).Scan(
		&sess.ID, &sess.UserID, &sess.ExpiresAt, &sess.CreatedAt,
	)
	return sess, err
}
