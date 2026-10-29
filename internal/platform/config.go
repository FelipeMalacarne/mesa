// Package platform holds what every binary needs before the application
// exists: configuration, paths, logging and the version.
package platform

import (
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Version is set at build time with -ldflags "-X …/platform.Version=1.2.3".
var Version = "dev"

// Config is read from the environment (server) or defaults (desktop, CLI).
type Config struct {
	// Store selects where Mesa keeps its own data: "sqlite" or "postgres".
	StoreDriver string
	// StoreDSN is a SQLite file path or a PostgreSQL URL.
	StoreDSN string
	// AppKey encrypts secrets at rest. Server mode requires it; desktop and
	// CLI generate one under the config directory when absent.
	AppKey string
	// Listen is the server address, e.g. ":8080".
	Listen string
	// LogLevel is debug, info, warn or error.
	LogLevel string
	// LogFormat is text or json.
	LogFormat string
	// SessionIdle evicts unused database sessions after this long.
	SessionIdle time.Duration
	// CORSOrigins allows browsers on other origins (development).
	CORSOrigins []string
	// MCPWrites exposes apply_sql on the server's /mcp endpoint.
	MCPWrites bool
	// Project is where to look for a .mesa/ folder ("off" disables it). The
	// CLI and MCP binaries default it to the working directory.
	Project string
	// Connections is a file of connections the deployment declares (empty:
	// none). They are listed with the stored ones and cannot be edited.
	Connections string
	// OIDC signs people in to the server through an identity provider; with
	// no issuer the server has no sign-in.
	OIDC OIDC
	// PublicURL is where people reach the server (https://mesa.example.com),
	// for the single sign-on callback; empty takes it from each request.
	PublicURL string
}

// OIDC is the identity provider of a server.
type OIDC struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	// Label names the provider on the sign-in button ("Zitadel").
	Label string
	// Scopes are asked for beyond openid, email and profile.
	Scopes []string
	// RolesClaim is the claim listing Mesa roles; empty reads Zitadel's
	// project roles, then roles, then groups.
	RolesClaim string
	// DefaultRole is for people the provider names no Mesa role for; empty
	// keeps them out.
	DefaultRole string
}

// FromEnv reads MESA_* variables over defaults for the given config directory.
func FromEnv(configDir string) Config {
	c := Config{
		StoreDriver: env("MESA_STORE", "sqlite"),
		StoreDSN:    env("MESA_STORE_DSN", filepath.Join(configDir, "mesa.db")),
		AppKey:      env("MESA_APP_KEY", env("APP_KEY", "")),
		Listen:      env("MESA_LISTEN", ":"+env("PORT", "8080")),
		LogLevel:    env("MESA_LOG_LEVEL", "info"),
		LogFormat:   env("MESA_LOG_FORMAT", "text"),
		SessionIdle: 10 * time.Minute,
		MCPWrites:   env("MESA_MCP_WRITES", "false") == "true",
		Project:     env("MESA_PROJECT", ""),
		Connections: env("MESA_CONNECTIONS", ""),
		PublicURL:   strings.TrimRight(env("MESA_PUBLIC_URL", ""), "/"),
		OIDC: OIDC{
			Issuer:       strings.TrimRight(env("MESA_OIDC_ISSUER", ""), "/"),
			ClientID:     env("MESA_OIDC_CLIENT_ID", ""),
			ClientSecret: env("MESA_OIDC_CLIENT_SECRET", ""),
			Label:        env("MESA_OIDC_LABEL", ""),
			Scopes:       strings.Fields(env("MESA_OIDC_SCOPES", "")),
			RolesClaim:   env("MESA_OIDC_ROLES_CLAIM", ""),
			DefaultRole:  env("MESA_OIDC_DEFAULT_ROLE", ""),
		},
	}
	if v := env("MESA_SESSION_IDLE", ""); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.SessionIdle = d
		}
	}
	if v := env("MESA_CORS_ORIGINS", ""); v != "" {
		for _, o := range strings.Split(v, ",") {
			if o = strings.TrimSpace(o); o != "" {
				c.CORSOrigins = append(c.CORSOrigins, o)
			}
		}
	}
	// Backwards compatibility with the first releases.
	if c.StoreDriver == "sqlite" {
		if v := env("DATABASE_URL", ""); v != "" && !strings.HasPrefix(v, "postgres") {
			c.StoreDSN = v
		}
	}
	if env("DB_DRIVER", "") == "postgres" {
		c.StoreDriver = "postgres"
		c.StoreDSN = env("DATABASE_URL", c.StoreDSN)
	}
	return c
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

// ConfigDir is where desktop and CLI keep their data: $MESA_HOME, else the
// OS config directory + /mesa.
func ConfigDir() (string, error) {
	if v := os.Getenv("MESA_HOME"); v != "" {
		return v, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "mesa"), nil
}

// Logger builds a slog logger from the config.
func Logger(c Config) *slog.Logger {
	var level slog.Level
	switch strings.ToLower(c.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}
	if strings.ToLower(c.LogFormat) == "json" {
		return slog.New(slog.NewJSONHandler(os.Stderr, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stderr, opts))
}

// Int parses an integer environment variable with a default.
func Int(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
