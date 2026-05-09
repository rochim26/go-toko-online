package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tokoonline/app/internal/app"
	"github.com/tokoonline/app/internal/config"
	"github.com/tokoonline/app/internal/db"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	sqlDB, err := app.OpenSQL(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("sql: %v", err)
	}
	defer sqlDB.Close()

	a := app.New(cfg, pool, sqlDB)

	// Bootstrap admin if missing
	if cfg.AdminBootstrapEmail != "" && cfg.AdminBootstrapPassword != "" {
		if err := a.Auth.EnsureAdmin(ctx, cfg.AdminBootstrapEmail, cfg.AdminBootstrapPassword); err != nil {
			log.Printf("WARN: ensure admin: %v", err)
		}
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           a.Routes(),
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("listening on :%s (%s)", cfg.Port, cfg.BaseURL)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("shutting down...")
	shutdown, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
}
