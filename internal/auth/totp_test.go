package auth

import (
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

// generateTestTOTPCode creates a fresh key and returns the secret + a valid code
// generated at the current time. Used across multiple TOTP tests.
func generateTestTOTPCode(t *testing.T) (secret, code string) {
	t.Helper()
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "TestApp",
		AccountName: "testuser",
	})
	if err != nil {
		t.Fatalf("failed to generate TOTP key: %v", err)
	}
	code, err = totp.GenerateCode(key.Secret(), time.Now())
	if err != nil {
		t.Fatalf("failed to generate TOTP code: %v", err)
	}
	return key.Secret(), code
}

func TestValidateTOTP_ValidCode(t *testing.T) {
	secret, code := generateTestTOTPCode(t)
	if !ValidateTOTP(code, secret) {
		t.Error("expected ValidateTOTP() = true for a freshly generated code")
	}
}

func TestValidateTOTP_InvalidCode(t *testing.T) {
	secret, _ := generateTestTOTPCode(t)
	if ValidateTOTP("000000", secret) {
		t.Error("expected ValidateTOTP() = false for code '000000' against a real secret")
	}
}

func TestValidateTOTP_EmptyCode(t *testing.T) {
	secret, _ := generateTestTOTPCode(t)
	if ValidateTOTP("", secret) {
		t.Error("expected ValidateTOTP() = false for an empty code")
	}
}

func TestValidateTOTP_EmptySecret(t *testing.T) {
	if ValidateTOTP("123456", "") {
		t.Error("expected ValidateTOTP() = false for an empty secret")
	}
}

func TestTOTPService_Generate_ReturnsKeyWithURL(t *testing.T) {
	svc := &TOTPService{} // db not needed for Generate

	key, err := svc.Generate("alice")
	if err != nil {
		t.Fatalf("Generate() returned error: %v", err)
	}
	if key == nil {
		t.Fatal("Generate() returned nil key")
	}
	if key.Secret() == "" {
		t.Error("expected non-empty secret from generated key")
	}
	if key.URL() == "" {
		t.Error("expected non-empty URL from generated key")
	}
}
