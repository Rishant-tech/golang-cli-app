package cli

import (
	"context"
	"fmt"
	"os"

	qrterminal "github.com/mdp/qrterminal/v3"
	"github.com/pterm/pterm"
)

// cmdWhoami prints a formatted table of the current user's details.
// It is also displayed automatically right after a successful login.
func (r *Router) cmdWhoami() {
	mfaStatus := "Disabled"
	if r.user.Is2FAEnabled() {
		mfaStatus = "Enabled"
	}

	lastLogin := "Never (this is your first login)"
	if r.user.LastLoginAt != nil {
		lastLogin = r.user.LastLoginAt.Format("2006-01-02 15:04:05 UTC")
	}

	pterm.DefaultTable.WithHasHeader(false).WithData(pterm.TableData{
		{"Username", r.user.Username},
		{"Registered", r.user.CreatedAt.Format("2006-01-02 15:04:05 UTC")},
		{"MFA Status", mfaStatus},
		{"Session Expires", r.session.ExpiresAt.Format("2006-01-02 15:04:05 UTC")},
		{"Last Login", lastLogin},
	}).Render()
	fmt.Println()
}

// cmdEnable2FA walks the user through enabling TOTP-based 2FA.
// Displays a QR code, then verifies the user can generate valid codes before saving.
func (r *Router) cmdEnable2FA() {
	if r.user.Is2FAEnabled() {
		pterm.Warning.Println("2FA is already enabled on your account. Disable it first if you want to reset it.")
		return
	}

	key, err := r.totpSvc.Generate(r.user.Username)
	if err != nil {
		pterm.Error.Printf("Failed to generate 2FA key: %v\n", err)
		return
	}

	fmt.Println("\nScan the QR code below with Google Authenticator or a compatible app:")
	fmt.Println()
	qrterminal.Generate(key.URL(), qrterminal.L, os.Stdout)
	fmt.Printf("\nCan't scan? Enter this key manually: %s\n\n", key.Secret())

	code, err := r.readInput("Enter the 6-digit code from your app to verify: ")
	if err != nil {
		return
	}

	ctx := context.Background()
	if err := r.totpSvc.Enable(ctx, r.user.ID, key.Secret(), code); err != nil {
		pterm.Error.Printf("Failed to enable 2FA: %v\n", err)
		return
	}

	// Update in-memory user state so Is2FAEnabled() reflects the change immediately
	secret := key.Secret()
	r.user.TOTPEnabled = true
	r.user.TOTPSecret = &secret

	pterm.Success.Println("2FA has been enabled on your account.")
}

// cmdDisable2FA disables TOTP 2FA after verifying the user's current code.
func (r *Router) cmdDisable2FA() {
	if !r.user.Is2FAEnabled() {
		pterm.Warning.Println("2FA is not enabled on your account.")
		return
	}

	code, err := r.readInput("Enter your current 2FA code to confirm: ")
	if err != nil {
		return
	}

	ctx := context.Background()
	if err := r.totpSvc.Disable(ctx, r.user.ID, code); err != nil {
		pterm.Error.Printf("Failed to disable 2FA: %v\n", err)
		return
	}

	// Update in-memory user state
	r.user.TOTPEnabled = false
	r.user.TOTPSecret = nil

	pterm.Success.Println("2FA has been disabled on your account.")
}

// cmdLogout invalidates the current session and returns to the pre-login state.
func (r *Router) cmdLogout() {
	ctx := context.Background()

	if err := r.sessionSvc.Delete(ctx, r.session.ID); err != nil {
		pterm.Error.Printf("Failed to invalidate session: %v\n", err)
		return
	}

	username := r.user.Username

	// Clear session state
	r.session = nil
	r.user = nil

	// Restore pre-login prompt and tab-completion
	r.setPrompt("> ")
	r.rl.Config.AutoComplete = preLoginCompleter()

	pterm.Success.Printf("Logged out successfully. Goodbye, %s!\n", username)
}

// cmdHelpPostLogin prints available commands after login.
func (r *Router) cmdHelpPostLogin() {
	fmt.Println()
	pterm.DefaultSection.Println("Available Commands")
	pterm.DefaultTable.WithHasHeader(true).WithData(pterm.TableData{
		{"Command", "Description"},
		{"whoami", "Show your account details and session info"},
		{"enable-2fa", "Enable TOTP-based two-factor authentication"},
		{"disable-2fa", "Disable two-factor authentication"},
		{"logout", "End your current session"},
		{"help", "Show this help message"},
		{"exit", "Quit the application"},
	}).Render()
	fmt.Println()
}
