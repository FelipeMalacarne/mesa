// Package config provides configuration loading functionality for the application.
package config

import "os"

type Config struct {
	AppKey      string
	DatabaseURL string
	Port        string
}

func Load() Config {
	return Config{
		AppKey:      getEnv("APP_KEY", "default_app_key_please_change_me"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://mesa:mesa@postgres:5432/mesa?sslmode=disable"),
		Port:        getEnv("PORT", "8080"),
	}
}

func getEnv(key, def string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return def
}
