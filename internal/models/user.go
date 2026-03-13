package models

import "time"

// User represents a registered user stored in the database.
type User struct {
	ID             string
	Username       string
	PasswordHash   string
	TOTPSecret     *string    // nil when 2FA is disabled
	TOTPEnabled    bool
	FailedAttempts int
	LockedUntil    *time.Time // nil when not locked
	LastLoginAt    *time.Time // nil on first login
	CreatedAt      time.Time
}

// Session represents an active authenticated session.
type Session struct {
	ID        string
	UserID    string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// IsLocked reports whether the account is currently locked out.
func (u *User) IsLocked() bool {
	return u.LockedUntil != nil && u.LockedUntil.After(time.Now())
}

// Is2FAEnabled reports whether TOTP-based 2FA is active for this user.
func (u *User) Is2FAEnabled() bool {
	return u.TOTPEnabled && u.TOTPSecret != nil
}
