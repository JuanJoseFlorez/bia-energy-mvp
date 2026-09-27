// Package config loads and validates service configuration from environment variables.
package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Config holds all settings the API needs at startup.
type Config struct {
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string

	HTTPPort           int
	CORSAllowedOrigins []string
	LogLevel           string
}

var validLogLevels = map[string]bool{"debug": true, "info": true, "warn": true, "error": true}

// Load reads the environment, applies defaults and validates every value.
// It reports all problems at once so misconfiguration is fixed in one pass.
func Load() (Config, error) {
	var problems []string

	cfg := Config{
		DBHost: strings.TrimSpace(os.Getenv("DB_HOST")),
		DBUser: strings.TrimSpace(os.Getenv("DB_USER")),
		// DB_PASSWORD is left untrimmed: a password may legitimately contain spaces.
		DBPassword: os.Getenv("DB_PASSWORD"),
		DBName:     strings.TrimSpace(os.Getenv("DB_NAME")),
		LogLevel:   strings.ToLower(getOr("LOG_LEVEL", "info")),
	}

	for _, req := range []struct{ name, value string }{
		{"DB_HOST", cfg.DBHost},
		{"DB_USER", cfg.DBUser},
		{"DB_PASSWORD", cfg.DBPassword},
		{"DB_NAME", cfg.DBName},
	} {
		if req.value == "" {
			problems = append(problems, req.name+" is required")
		}
	}

	var problem string
	if cfg.DBPort, problem = parsePort("DB_PORT", 5432); problem != "" {
		problems = append(problems, problem)
	}
	if cfg.HTTPPort, problem = parsePort("HTTP_PORT", 8080); problem != "" {
		problems = append(problems, problem)
	}
	if !validLogLevels[cfg.LogLevel] {
		problems = append(problems, fmt.Sprintf("LOG_LEVEL %q must be one of debug, info, warn, error", cfg.LogLevel))
	}

	cfg.CORSAllowedOrigins = parseList(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if len(cfg.CORSAllowedOrigins) == 0 {
		cfg.CORSAllowedOrigins = []string{"http://localhost:3000"}
	}

	if len(problems) > 0 {
		return Config{}, fmt.Errorf("invalid configuration: %s", strings.Join(problems, "; "))
	}
	return cfg, nil
}

// DatabaseURL builds a pgx connection URL with credentials safely escaped.
func (c Config) DatabaseURL() string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.DBUser, c.DBPassword),
		Host:     net.JoinHostPort(c.DBHost, strconv.Itoa(c.DBPort)),
		Path:     "/" + c.DBName,
		RawQuery: url.Values{"sslmode": {"disable"}}.Encode(),
	}
	return u.String()
}

func getOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// parsePort returns the port and, if invalid, a problem description.
func parsePort(key string, fallback int) (int, string) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, ""
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Sprintf("%s %q must be an integer between 1 and 65535", key, raw)
	}
	return port, ""
}

// parseList splits a comma-separated value, trimming spaces and dropping empty items.
func parseList(raw string) []string {
	var out []string
	for _, item := range strings.Split(raw, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}
