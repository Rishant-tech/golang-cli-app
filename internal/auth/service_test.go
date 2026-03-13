package auth

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	pgxmock "github.com/pashagolub/pgxmock/v3"
	"github.com/pquerna/otp/totp"
	"github.com/rishant-tech/golang-cli-app/internal/config"
	"golang.org/x/crypto/bcrypt"
)

// testConfig returns a config suitable for unit tests (fast bcrypt, short timeouts).
func testConfig() *config.Config {
	return &config.Config{
		SessionTimeout:    30 * time.Minute,
		MaxFailedAttempts: 5,
		LockoutDuration:   15 * time.Minute,
	}
}

// hashedPassword generates a bcrypt hash using MinCost to keep tests fast.
func hashedPassword(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	return string(hash)
}

// userColumns lists the columns returned by getUserByUsername queries.
var userColumns = []string{
	"id", "username", "password_hash", "totp_secret", "totp_enabled",
	"failed_attempts", "locked_until", "last_login_at", "created_at",
}

// ============================================================================
// Register
// ============================================================================

func TestRegister_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("INSERT INTO users").
		WithArgs("alice", pgxmock.AnyArg()). // AnyArg matches the bcrypt hash
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	svc := NewService(mock, testConfig())
	if err := svc.Register(context.Background(), "alice", "password123"); err != nil {
		t.Errorf("Register() unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled mock expectations: %v", err)
	}
}

func TestRegister_DuplicateUsername(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()

	// Simulate Postgres unique_violation (23505)
	mock.ExpectExec("INSERT INTO users").
		WithArgs("alice", pgxmock.AnyArg()).
		WillReturnError(&pgconn.PgError{Code: "23505"})

	svc := NewService(mock, testConfig())
	err := svc.Register(context.Background(), "alice", "password123")

	if err != ErrUserExists {
		t.Errorf("expected ErrUserExists, got: %v", err)
	}
}

// ============================================================================
// Login
// ============================================================================

func TestLogin_Success(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()

	hash := hashedPassword(t, "password123")
	now := time.Now()

	// 1. getUserByUsername
	mock.ExpectQuery("SELECT id, username").
		WithArgs("alice").
		WillReturnRows(pgxmock.NewRows(userColumns).AddRow(
			"user-id-1", "alice", hash,
			nil, false, // totp_secret, totp_enabled
			0, nil, nil, now, // failed_attempts, locked_until, last_login_at, created_at
		))

	// 2. resetFailedAttempts
	mock.ExpectExec("UPDATE users SET failed_attempts = 0").
		WithArgs("user-id-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	// 3. updateLastLogin
	mock.ExpectExec("UPDATE users SET last_login_at").
		WithArgs("user-id-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	// 4. createSession
	sessionExpiry := now.Add(30 * time.Minute)
	mock.ExpectQuery("INSERT INTO sessions").
		WithArgs("user-id-1", pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "expires_at", "created_at"}).
			AddRow("session-id-1", "user-id-1", sessionExpiry, now))

	svc := NewService(mock, testConfig())
	session, user, err := svc.Login(context.Background(), "alice", "password123", "")

	if err != nil {
		t.Fatalf("Login() unexpected error: %v", err)
	}
	if session == nil || session.ID != "session-id-1" {
		t.Errorf("expected session ID 'session-id-1', got: %v", session)
	}
	if user == nil || user.Username != "alice" {
		t.Errorf("expected user 'alice', got: %v", user)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled mock expectations: %v", err)
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()

	mock.ExpectQuery("SELECT id, username").
		WithArgs("unknown").
		WillReturnRows(pgxmock.NewRows(userColumns)) // empty result set

	svc := NewService(mock, testConfig())
	_, _, err := svc.Login(context.Background(), "unknown", "password123", "")

	if err != ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got: %v", err)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()

	hash := hashedPassword(t, "correct-password")
	now := time.Now()

	mock.ExpectQuery("SELECT id, username").
		WithArgs("alice").
		WillReturnRows(pgxmock.NewRows(userColumns).AddRow(
			"user-id-1", "alice", hash,
			nil, false, 0, nil, nil, now,
		))

	// After wrong password: incrementFailedAttempts is called (attempts 0→1, below threshold)
	mock.ExpectExec("UPDATE users SET failed_attempts").
		WithArgs(1, "user-id-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	svc := NewService(mock, testConfig())
	_, _, err := svc.Login(context.Background(), "alice", "wrong-password", "")

	if err != ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got: %v", err)
	}
}

func TestLogin_AccountLocked(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()

	lockedUntil := time.Now().Add(10 * time.Minute)
	now := time.Now()
	hash := hashedPassword(t, "password123")

	mock.ExpectQuery("SELECT id, username").
		WithArgs("alice").
		WillReturnRows(pgxmock.NewRows(userColumns).AddRow(
			"user-id-1", "alice", hash,
			nil, false, 5, &lockedUntil, nil, now,
		))

	svc := NewService(mock, testConfig())
	_, _, err := svc.Login(context.Background(), "alice", "password123", "")

	if err == nil {
		t.Fatal("expected an error for locked account, got nil")
	}
	// The error wraps ErrAccountLocked — use errors.Is behaviour via string check
	// since fmt.Errorf wraps with %w
	if !isWrappedError(err, ErrAccountLocked) {
		t.Errorf("expected error to wrap ErrAccountLocked, got: %v", err)
	}
}

func TestLogin_InvalidTOTPCode(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()

	hash := hashedPassword(t, "password123")
	secret := "JBSWY3DPEHPK3PXP"
	now := time.Now()

	mock.ExpectQuery("SELECT id, username").
		WithArgs("alice").
		WillReturnRows(pgxmock.NewRows(userColumns).AddRow(
			"user-id-1", "alice", hash,
			&secret, true, // 2FA enabled
			0, nil, nil, now,
		))

	svc := NewService(mock, testConfig())
	_, _, err := svc.Login(context.Background(), "alice", "password123", "000000")

	if err != ErrInvalidTOTP {
		t.Errorf("expected ErrInvalidTOTP, got: %v", err)
	}
}

func TestLogin_SuccessWithTOTP(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()

	hash := hashedPassword(t, "password123")
	now := time.Now()

	// Generate a real TOTP secret and a valid code for right now
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "test", AccountName: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	secret := key.Secret()
	validCode, err := totp.GenerateCode(secret, now)
	if err != nil {
		t.Fatal(err)
	}

	// 1. getUserByUsername — returns user with 2FA enabled
	mock.ExpectQuery("SELECT id, username").
		WithArgs("alice").
		WillReturnRows(pgxmock.NewRows(userColumns).AddRow(
			"user-id-1", "alice", hash,
			&secret, true,
			0, nil, nil, now,
		))

	// 2. resetFailedAttempts
	mock.ExpectExec("UPDATE users SET failed_attempts = 0").
		WithArgs("user-id-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	// 3. updateLastLogin
	mock.ExpectExec("UPDATE users SET last_login_at").
		WithArgs("user-id-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	// 4. createSession
	sessionExpiry := now.Add(30 * time.Minute)
	mock.ExpectQuery("INSERT INTO sessions").
		WithArgs("user-id-1", pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "expires_at", "created_at"}).
			AddRow("session-id-1", "user-id-1", sessionExpiry, now))

	svc := NewService(mock, testConfig())
	session, user, err := svc.Login(context.Background(), "alice", "password123", validCode)

	if err != nil {
		t.Fatalf("Login() with TOTP unexpected error: %v", err)
	}
	if session == nil {
		t.Error("expected session, got nil")
	}
	if user == nil || !user.Is2FAEnabled() {
		t.Error("expected user with 2FA enabled")
	}
}

func TestLogin_LockoutAfterMaxFailedAttempts(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()

	hash := hashedPassword(t, "correct-password")
	now := time.Now()
	cfg := testConfig()
	cfg.MaxFailedAttempts = 5

	mock.ExpectQuery("SELECT id, username").
		WithArgs("alice").
		WillReturnRows(pgxmock.NewRows(userColumns).AddRow(
			"user-id-1", "alice", hash,
			nil, false,
			4, nil, nil, now, // 4 previous failures — next attempt triggers lockout
		))

	// incrementFailedAttempts: 4+1 = 5 >= MaxFailedAttempts → sets locked_until
	mock.ExpectExec("UPDATE users SET failed_attempts").
		WithArgs(5, pgxmock.AnyArg(), "user-id-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	svc := NewService(mock, cfg)
	_, _, err := svc.Login(context.Background(), "alice", "wrong-password", "")

	if err != ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials on wrong password, got: %v", err)
	}
}

// ============================================================================
// NeedsTOTP
// ============================================================================

func TestNeedsTOTP_ReturnsTrueWhen2FAEnabled(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()

	secret := "JBSWY3DPEHPK3PXP"
	now := time.Now()
	hash := hashedPassword(t, "password123")

	mock.ExpectQuery("SELECT id, username").
		WithArgs("alice").
		WillReturnRows(pgxmock.NewRows(userColumns).AddRow(
			"user-id-1", "alice", hash,
			&secret, true, 0, nil, nil, now,
		))

	svc := NewService(mock, testConfig())
	if !svc.NeedsTOTP(context.Background(), "alice") {
		t.Error("expected NeedsTOTP() = true for user with 2FA enabled")
	}
}

func TestNeedsTOTP_ReturnsFalseWhen2FADisabled(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()

	now := time.Now()
	hash := hashedPassword(t, "password123")

	mock.ExpectQuery("SELECT id, username").
		WithArgs("alice").
		WillReturnRows(pgxmock.NewRows(userColumns).AddRow(
			"user-id-1", "alice", hash,
			nil, false, 0, nil, nil, now,
		))

	svc := NewService(mock, testConfig())
	if svc.NeedsTOTP(context.Background(), "alice") {
		t.Error("expected NeedsTOTP() = false for user without 2FA")
	}
}

func TestNeedsTOTP_ReturnsFalseForUnknownUser(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()

	mock.ExpectQuery("SELECT id, username").
		WithArgs("nobody").
		WillReturnRows(pgxmock.NewRows(userColumns)) // empty

	svc := NewService(mock, testConfig())
	if svc.NeedsTOTP(context.Background(), "nobody") {
		t.Error("expected NeedsTOTP() = false for unknown user")
	}
}

// ============================================================================
// SessionService
// ============================================================================

func TestSessionService_GetWithUser_Success(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()

	now := time.Now()
	expiry := now.Add(30 * time.Minute)

	mock.ExpectQuery("SELECT s.id, s.user_id").
		WithArgs("session-id-1").
		WillReturnRows(pgxmock.NewRows([]string{
			"s.id", "s.user_id", "s.expires_at", "s.created_at",
			"u.id", "u.username", "u.totp_enabled", "u.totp_secret", "u.last_login_at", "u.created_at",
		}).AddRow(
			"session-id-1", "user-id-1", expiry, now,
			"user-id-1", "alice", false, nil, nil, now,
		))

	svc := NewSessionService(mock)
	sess, user, err := svc.GetWithUser(context.Background(), "session-id-1")

	if err != nil {
		t.Fatalf("GetWithUser() unexpected error: %v", err)
	}
	if sess.ID != "session-id-1" {
		t.Errorf("unexpected session ID: %s", sess.ID)
	}
	if user.Username != "alice" {
		t.Errorf("unexpected username: %s", user.Username)
	}
}

func TestSessionService_GetWithUser_NotFound(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()

	mock.ExpectQuery("SELECT s.id, s.user_id").
		WithArgs("expired-session").
		WillReturnRows(pgxmock.NewRows([]string{})) // empty = expired or missing

	svc := NewSessionService(mock)
	_, _, err := svc.GetWithUser(context.Background(), "expired-session")

	if err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got: %v", err)
	}
}

func TestSessionService_Delete_Success(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()

	mock.ExpectExec("DELETE FROM sessions").
		WithArgs("session-id-1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	svc := NewSessionService(mock)
	if err := svc.Delete(context.Background(), "session-id-1"); err != nil {
		t.Errorf("Delete() unexpected error: %v", err)
	}
}

// ============================================================================
// helpers
// ============================================================================

// isWrappedError checks whether target is in the error chain of err.
// Equivalent to errors.Is but works for sentinel errors wrapped with %w.
func isWrappedError(err, target error) bool {
	for err != nil {
		if err == target {
			return true
		}
		// unwrap one level
		type unwrapper interface{ Unwrap() error }
		if u, ok := err.(unwrapper); ok {
			err = u.Unwrap()
		} else {
			return false
		}
	}
	return false
}
