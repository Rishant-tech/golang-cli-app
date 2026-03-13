package models

import (
	"testing"
	"time"
)

// -- IsLocked -----------------------------------------------------------------

func TestUser_IsLocked_WhenLockedUntilIsNil(t *testing.T) {
	u := User{LockedUntil: nil}
	if u.IsLocked() {
		t.Error("expected IsLocked() = false when LockedUntil is nil")
	}
}

func TestUser_IsLocked_WhenLockedUntilIsInFuture(t *testing.T) {
	future := time.Now().Add(15 * time.Minute)
	u := User{LockedUntil: &future}
	if !u.IsLocked() {
		t.Error("expected IsLocked() = true when LockedUntil is in the future")
	}
}

func TestUser_IsLocked_WhenLockedUntilIsInPast(t *testing.T) {
	past := time.Now().Add(-1 * time.Minute)
	u := User{LockedUntil: &past}
	if u.IsLocked() {
		t.Error("expected IsLocked() = false when LockedUntil has already passed")
	}
}

// -- Is2FAEnabled -------------------------------------------------------------

func TestUser_Is2FAEnabled_WhenEnabledWithSecret(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"
	u := User{TOTPEnabled: true, TOTPSecret: &secret}
	if !u.Is2FAEnabled() {
		t.Error("expected Is2FAEnabled() = true when TOTPEnabled=true and secret is set")
	}
}

func TestUser_Is2FAEnabled_WhenEnabledButSecretIsNil(t *testing.T) {
	// Inconsistent DB state — treat as disabled
	u := User{TOTPEnabled: true, TOTPSecret: nil}
	if u.Is2FAEnabled() {
		t.Error("expected Is2FAEnabled() = false when TOTPSecret is nil even if TOTPEnabled=true")
	}
}

func TestUser_Is2FAEnabled_WhenDisabled(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"
	u := User{TOTPEnabled: false, TOTPSecret: &secret}
	if u.Is2FAEnabled() {
		t.Error("expected Is2FAEnabled() = false when TOTPEnabled=false")
	}
}

func TestUser_Is2FAEnabled_WhenBothFalseAndNil(t *testing.T) {
	u := User{TOTPEnabled: false, TOTPSecret: nil}
	if u.Is2FAEnabled() {
		t.Error("expected Is2FAEnabled() = false when both fields are zero values")
	}
}
