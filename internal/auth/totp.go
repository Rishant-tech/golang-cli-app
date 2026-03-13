package auth

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
	gototp "github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// TOTPService handles generation, verification, and toggling of TOTP-based 2FA.
type TOTPService struct {
	db *pgxpool.Pool
}

// NewTOTPService creates a new TOTP service.
func NewTOTPService(db *pgxpool.Pool) *TOTPService {
	return &TOTPService{db: db}
}

// ValidateTOTP is a package-level helper used by Service.Login to verify a code against a secret.
func ValidateTOTP(code, secret string) bool {
	return totp.Validate(code, secret)
}

// Generate creates a new TOTP key for the given username.
// Returns the key (contains URL for QR code and the raw secret).
func (t *TOTPService) Generate(username string) (*gototp.Key, error) {
	return totp.Generate(totp.GenerateOpts{
		Issuer:      "GoLoginApp",
		AccountName: username,
	})
}

// Enable verifies the provided code against the secret and, if valid, saves 2FA for the user.
func (t *TOTPService) Enable(ctx context.Context, userID, secret, code string) error {
	if !totp.Validate(code, secret) {
		return ErrInvalidTOTP
	}

	_, err := t.db.Exec(ctx,
		`UPDATE users SET totp_secret = $1, totp_enabled = TRUE WHERE id = $2`,
		secret, userID,
	)
	return err
}

// Disable verifies the user's current TOTP code and, if valid, removes 2FA from the account.
func (t *TOTPService) Disable(ctx context.Context, userID, code string) error {
	var secret string
	err := t.db.QueryRow(ctx,
		`SELECT totp_secret FROM users WHERE id = $1 AND totp_enabled = TRUE`, userID,
	).Scan(&secret)
	if err != nil {
		return errors.New("2FA is not enabled on this account")
	}

	if !totp.Validate(code, secret) {
		return ErrInvalidTOTP
	}

	_, err = t.db.Exec(ctx,
		`UPDATE users SET totp_secret = NULL, totp_enabled = FALSE WHERE id = $1`, userID,
	)
	return err
}
