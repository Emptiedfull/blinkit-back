package config

import (
	"fmt"
	"os"
)

type config struct {
	DBUrl     string
	JWTSecret string
	Port      string
}

func GetConfig() (*config, error) {
	cfg := &config{
		DBUrl:     os.Getenv("DATABASE_URL"),
		JWTSecret: envDefault("JWT_SECRET", "water is wet"),
		Port:      envDefault("PORT", "8070"),
	}

	if cfg.DBUrl == "" {
		return nil, fmt.Errorf("NO DB URL")
	}

	return cfg, nil

}

func envDefault(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val

}
