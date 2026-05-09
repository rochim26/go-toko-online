package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Env                    string
	Port                   string
	BaseURL                string
	Secret                 string
	DatabaseURL            string
	UploadDir              string
	AdminBootstrapEmail    string
	AdminBootstrapPassword string
}

func Load() (*Config, error) {
	_ = godotenv.Load()
	c := &Config{
		Env:                    getenv("APP_ENV", "development"),
		Port:                   getenv("APP_PORT", "8100"),
		BaseURL:                getenv("APP_BASE_URL", "http://localhost:8100"),
		Secret:                 getenv("APP_SECRET", ""),
		DatabaseURL:            getenv("DATABASE_URL", ""),
		UploadDir:              getenv("UPLOAD_DIR", "static/uploads"),
		AdminBootstrapEmail:    getenv("ADMIN_BOOTSTRAP_EMAIL", "admin@local"),
		AdminBootstrapPassword: getenv("ADMIN_BOOTSTRAP_PASSWORD", ""),
	}
	if c.Secret == "" || len(c.Secret) < 16 {
		return nil, fmt.Errorf("APP_SECRET must be at least 16 characters")
	}
	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	c.BaseURL = strings.TrimRight(c.BaseURL, "/")
	return c, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
