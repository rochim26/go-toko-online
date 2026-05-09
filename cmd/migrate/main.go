package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}
	dir := "migrations"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		log.Fatalf("create migrations table: %v", err)
	}

	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		log.Fatalf("glob: %v", err)
	}
	sort.Strings(files)

	for _, f := range files {
		name := filepath.Base(f)
		version := strings.TrimSuffix(name, ".sql")
		var exists bool
		if err := conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)", version).Scan(&exists); err != nil {
			log.Fatalf("check %s: %v", version, err)
		}
		if exists {
			fmt.Printf("[skip] %s\n", version)
			continue
		}
		buf, err := os.ReadFile(f)
		if err != nil {
			log.Fatalf("read %s: %v", f, err)
		}
		fmt.Printf("[apply] %s\n", version)
		tx, err := conn.Begin(ctx)
		if err != nil {
			log.Fatalf("begin: %v", err)
		}
		if _, err := tx.Exec(ctx, string(buf)); err != nil {
			tx.Rollback(ctx)
			log.Fatalf("apply %s: %v", version, err)
		}
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES($1)", version); err != nil {
			tx.Rollback(ctx)
			log.Fatalf("track %s: %v", version, err)
		}
		if err := tx.Commit(ctx); err != nil {
			log.Fatalf("commit %s: %v", version, err)
		}
	}
	fmt.Println("[ok] migrations complete")
}
