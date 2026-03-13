package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/pterm/pterm"
	"github.com/rishant-tech/golang-cli-app/internal/auth"
	"github.com/rishant-tech/golang-cli-app/internal/models"
)

// Router is the main CLI loop. It holds the current session state and
// dispatches commands to the appropriate handler based on login state.
type Router struct {
	authSvc    *auth.Service
	sessionSvc *auth.SessionService
	totpSvc    *auth.TOTPService

	session *models.Session // nil when not logged in
	user    *models.User    // nil when not logged in

	rl            *readline.Instance
	currentPrompt string
}

// NewRouter wires up all services and creates the readline instance.
func NewRouter(authSvc *auth.Service, sessionSvc *auth.SessionService, totpSvc *auth.TOTPService) *Router {
	r := &Router{
		authSvc:    authSvc,
		sessionSvc: sessionSvc,
		totpSvc:    totpSvc,
	}

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "> ",
		HistoryFile:     "/tmp/golang-cli-app-history",
		AutoComplete:    preLoginCompleter(),
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		panic(fmt.Sprintf("failed to init readline: %v", err))
	}

	r.rl = rl
	r.currentPrompt = "> "
	return r
}

// Run starts the interactive CLI loop.
func (r *Router) Run() {
	defer r.rl.Close()

	pterm.DefaultHeader.Println("GoLoginApp — Secure CLI Login System")
	fmt.Println("Type 'help' to see available commands.")
	fmt.Println()

	for {
		line, err := r.rl.Readline()
		if err == readline.ErrInterrupt {
			if strings.TrimSpace(line) == "" {
				r.cmdExit()
				return
			}
			continue
		}
		if err == io.EOF {
			r.cmdExit()
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check if an active session has expired between commands
		if r.session != nil && r.session.ExpiresAt.Before(time.Now()) {
			pterm.Warning.Println("Your session has expired. Please login again.")
			r.session = nil
			r.user = nil
			r.setPrompt("> ")
			r.rl.Config.AutoComplete = preLoginCompleter()
		}

		if r.session == nil {
			r.handlePreLogin(line)
		} else {
			r.handlePostLogin(line)
		}
	}
}

// handlePreLogin dispatches commands available before authentication.
func (r *Router) handlePreLogin(cmd string) {
	switch cmd {
	case "register":
		r.cmdRegister()
	case "login":
		r.cmdLogin()
	case "help":
		r.cmdHelpPreLogin()
	case "exit", "quit":
		r.cmdExit()
	default:
		pterm.Error.Printf("Unknown command: '%s'. Type 'help' for available commands.\n", cmd)
	}
}

// handlePostLogin dispatches commands available after authentication.
func (r *Router) handlePostLogin(cmd string) {
	switch cmd {
	case "whoami":
		r.cmdWhoami()
	case "enable-2fa":
		r.cmdEnable2FA()
	case "disable-2fa":
		r.cmdDisable2FA()
	case "logout":
		r.cmdLogout()
	case "help":
		r.cmdHelpPostLogin()
	case "exit", "quit":
		r.cmdExit()
	default:
		pterm.Error.Printf("Unknown command: '%s'. Type 'help' for available commands.\n", cmd)
	}
}

// readInput temporarily changes the prompt, reads one line, then restores the prompt.
// Use this for sub-prompts within commands (e.g. "Username: ").
func (r *Router) readInput(prompt string) (string, error) {
	r.rl.SetPrompt(prompt)
	line, err := r.rl.Readline()
	r.rl.SetPrompt(r.currentPrompt)
	return strings.TrimSpace(line), err
}

// setPrompt updates the displayed prompt and remembers it for restoration after sub-prompts.
func (r *Router) setPrompt(p string) {
	r.currentPrompt = p
	r.rl.SetPrompt(p)
}

// preLoginCompleter returns tab-completion options for the logged-out state.
func preLoginCompleter() readline.AutoCompleter {
	return readline.NewPrefixCompleter(
		readline.PcItem("register"),
		readline.PcItem("login"),
		readline.PcItem("help"),
		readline.PcItem("exit"),
	)
}

// postLoginCompleter returns tab-completion options for the logged-in state.
func postLoginCompleter() readline.AutoCompleter {
	return readline.NewPrefixCompleter(
		readline.PcItem("whoami"),
		readline.PcItem("enable-2fa"),
		readline.PcItem("disable-2fa"),
		readline.PcItem("logout"),
		readline.PcItem("help"),
		readline.PcItem("exit"),
	)
}
