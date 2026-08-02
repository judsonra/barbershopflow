package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/example/barberflow/backend/internal/auth"
	"github.com/example/barberflow/backend/internal/database"
	apihttp "github.com/example/barberflow/backend/internal/http"
	"github.com/example/barberflow/backend/internal/notify"
	"github.com/example/barberflow/backend/internal/repository"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		response, err := http.Get("http://localhost:" + env("API_PORT", "8080") + "/health")
		if err != nil || response.StatusCode != http.StatusOK {
			fmt.Fprintln(os.Stderr, "unhealthy")
			os.Exit(1)
		}
		_ = response.Body.Close()
		return
	}
	databaseURL := env("DATABASE_URL", "postgres://barbershop:change-me@localhost:5432/barbershop?sslmode=disable")
	port := env("API_PORT", "8080")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	if err := database.Migrate(ctx, pool); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	tokenizer := auth.NewTokenizer(
		env("JWT_SECRET", "dev-secret-change-me"),
		envDuration("JWT_ACCESS_TTL", 15*time.Minute),
		envDuration("JWT_REFRESH_TTL", 7*24*time.Hour),
	)
	oauthRedirectBase := env("OAUTH_REDIRECT_BASE_URL", "http://localhost:8080")
	handler := apihttp.New(repository.New(pool), apihttp.Config{
		Origins:     os.Getenv("CORS_ORIGINS"),
		Tokenizer:   tokenizer,
		Google:      auth.NewGoogleProvider(os.Getenv("GOOGLE_CLIENT_ID"), os.Getenv("GOOGLE_CLIENT_SECRET"), oauthRedirectBase+"/api/v1/auth/google/callback"),
		Facebook:    auth.NewFacebookProvider(os.Getenv("FACEBOOK_CLIENT_ID"), os.Getenv("FACEBOOK_CLIENT_SECRET"), oauthRedirectBase+"/api/v1/auth/facebook/callback"),
		Notifier:    notify.New(os.Getenv("ZENVIA_API_TOKEN"), env("ZENVIA_FROM_SMS", "BarberFlow"), env("ZENVIA_FROM_WHATSAPP", "BarberFlow")),
		FrontendURL: env("FRONTEND_URL", "http://localhost:5173"),
	})
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("API listening on :%s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return fallback
}
