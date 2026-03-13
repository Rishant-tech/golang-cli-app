package main

import (
	"context"
	"fmt"
	"os"

	"github.com/rishant-tech/golang-cli-app/internal/auth"
	"github.com/rishant-tech/golang-cli-app/internal/cli"
	"github.com/rishant-tech/golang-cli-app/internal/config"
	"github.com/rishant-tech/golang-cli-app/internal/db"
)

func main() {
	cfg := config.Load()

	ctx := context.Background()

	pool, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.RunMigrations(cfg.DatabaseURL); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to run migrations: %v\n", err)
		os.Exit(1)
	}

	authSvc := auth.NewService(pool, cfg)
	sessionSvc := auth.NewSessionService(pool)
	totpSvc := auth.NewTOTPService(pool)

	router := cli.NewRouter(authSvc, sessionSvc, totpSvc)
	router.Run()
}
