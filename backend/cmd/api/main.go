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

	"github.com/example/barberflow/backend/internal/database"
	apihttp "github.com/example/barberflow/backend/internal/http"
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

	handler := apihttp.New(repository.New(pool), os.Getenv("CORS_ORIGINS"))
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
