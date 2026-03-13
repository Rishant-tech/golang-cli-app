package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
)

// cmdRegister handles the 'register' command.
// Prompts for username, password, and password confirmation.
func (r *Router) cmdRegister() {
	username, err := r.readInput("Username: ")
	if err != nil {
		return
	}
	if username == "" {
		pterm.Error.Println("Username cannot be empty.")
		return
	}

	password, err := r.rl.ReadPassword("Password: ")
	if err != nil {
		return
	}
	if len(password) < 6 {
		pterm.Error.Println("Password must be at least 6 characters.")
		return
	}

	confirm, err := r.rl.ReadPassword("Confirm password: ")
	if err != nil {
		return
	}
	if string(password) != string(confirm) {
		pterm.Error.Println("Passwords do not match.")
		return
	}

	ctx := context.Background()
	if err := r.authSvc.Register(ctx, username, string(password)); err != nil {
		pterm.Error.Printf("Registration failed: %v\n", err)
		return
	}

	pterm.Success.Printf("User '%s' registered successfully. You can now login.\n", username)
}

// cmdLogin handles the 'login' command.
// Prompts for credentials and a TOTP code if 2FA is enabled on the account.
func (r *Router) cmdLogin() {
	username, err := r.readInput("Username: ")
	if err != nil {
		return
	}
	if username == "" {
		pterm.Error.Println("Username cannot be empty.")
		return
	}

	password, err := r.rl.ReadPassword("Password: ")
	if err != nil {
		return
	}

	ctx := context.Background()

	// Only prompt for TOTP code if the account has 2FA enabled
	totpCode := ""
	if r.authSvc.NeedsTOTP(ctx, username) {
		totpCode, err = r.readInput("2FA Code: ")
		if err != nil {
			return
		}
	}

	session, user, err := r.authSvc.Login(ctx, username, string(password), totpCode)
	if err != nil {
		pterm.Error.Printf("Login failed: %v\n", err)
		return
	}

	// Store session state in the router
	r.session = session
	r.user = user

	// Update prompt and tab-completion for the logged-in state
	r.setPrompt(fmt.Sprintf("[%s] > ", user.Username))
	r.rl.Config.AutoComplete = postLoginCompleter()

	pterm.Success.Printf("Welcome back, %s!\n\n", user.Username)
	r.cmdWhoami()
}

// cmdHelpPreLogin prints available commands before login.
func (r *Router) cmdHelpPreLogin() {
	fmt.Println()
	pterm.DefaultSection.Println("Available Commands")
	pterm.DefaultTable.WithHasHeader(true).WithData(pterm.TableData{
		{"Command", "Description"},
		{"register", "Create a new user account"},
		{"login", "Login with username and password"},
		{"help", "Show this help message"},
		{"exit", "Quit the application"},
	}).Render()
	fmt.Println()
}

// cmdExit prints a farewell message and exits the process.
func (r *Router) cmdExit() {
	pterm.Info.Println("Goodbye!")
	os.Exit(0)
}
