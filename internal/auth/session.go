package auth

import (
	"context"
	"errors"

	"github.com/rishant-tech/golang-cli-app/internal/models"
)

var ErrSessionNotFound = errors.New("session not found or expired")

// SessionService manages session lookup and invalidation.
type SessionService struct {
	db dbIface // defined in service.go — *pgxpool.Pool satisfies this
}

// NewSessionService creates a new session service.
func NewSessionService(db dbIface) *SessionService {
	return &SessionService{db: db}
}

// GetWithUser returns an active (non-expired) session and the associated user details.
func (s *SessionService) GetWithUser(ctx context.Context, sessionID string) (*models.Session, *models.User, error) {
	sess := &models.Session{}
	user := &models.User{}

	err := s.db.QueryRow(ctx, `
		SELECT s.id, s.user_id, s.expires_at, s.created_at,
		       u.id, u.username, u.totp_enabled, u.totp_secret, u.last_login_at, u.created_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.id = $1 AND s.expires_at > NOW()
	`, sessionID).Scan(
		&sess.ID, &sess.UserID, &sess.ExpiresAt, &sess.CreatedAt,
		&user.ID, &user.Username, &user.TOTPEnabled, &user.TOTPSecret,
		&user.LastLoginAt, &user.CreatedAt,
	)
	if err != nil {
		return nil, nil, ErrSessionNotFound
	}

	return sess, user, nil
}

// Delete removes a session row, effectively logging the user out.
func (s *SessionService) Delete(ctx context.Context, sessionID string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, sessionID)
	return err
}
