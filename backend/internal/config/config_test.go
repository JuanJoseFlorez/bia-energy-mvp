package config

import (
	"net/url"
	"slices"
	"strings"
	"testing"
)

var allVars = []string{
	"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME",
	"HTTP_PORT", "CORS_ALLOWED_ORIGINS", "LOG_LEVEL",
}

// setEnv sets every known variable, using "" for the ones not in vals.
func setEnv(t *testing.T, vals map[string]string) {
	t.Helper()
	for _, k := range allVars {
		t.Setenv(k, vals[k])
	}
}

func validEnv() map[string]string {
	return map[string]string{
		"DB_HOST":     "db",
		"DB_USER":     "bia",
		"DB_PASSWORD": "secret",
		"DB_NAME":     "bia_energy",
	}
}

func TestLoadDefaults(t *testing.T) {
	setEnv(t, validEnv())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DBPort != 5432 {
		t.Errorf("DBPort = %d, want 5432", cfg.DBPort)
	}
	if cfg.HTTPPort != 8080 {
		t.Errorf("HTTPPort = %d, want 8080", cfg.HTTPPort)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", cfg.LogLevel)
	}
	if want := []string{"http://localhost:3000"}; !slices.Equal(cfg.CORSAllowedOrigins, want) {
		t.Errorf("CORSAllowedOrigins = %v, want %v", cfg.CORSAllowedOrigins, want)
	}
}

func TestLoadMissingRequiredNamesAll(t *testing.T) {
	setEnv(t, map[string]string{})

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	for _, name := range []string{"DB_HOST", "DB_USER", "DB_PASSWORD", "DB_NAME"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not mention %s", err, name)
		}
	}
}

func TestLoadWhitespaceOnlyRequiredIsReported(t *testing.T) {
	env := validEnv()
	env["DB_HOST"] = "  "
	setEnv(t, env)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "DB_HOST") {
		t.Errorf("error %q does not mention DB_HOST", err)
	}
}

func TestLoadInvalidValues(t *testing.T) {
	tests := []struct {
		name, key, value string
	}{
		{"non-numeric DB port", "DB_PORT", "abc"},
		{"DB port out of range", "DB_PORT", "70000"},
		{"HTTP port zero", "HTTP_PORT", "0"},
		{"unknown log level", "LOG_LEVEL", "verbose"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := validEnv()
			env[tt.key] = tt.value
			setEnv(t, env)

			_, err := Load()
			if err == nil {
				t.Fatal("Load() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.key) {
				t.Errorf("error %q does not mention %s", err, tt.key)
			}
		})
	}
}

func TestLoadCORSOriginsTrimmed(t *testing.T) {
	env := validEnv()
	env["CORS_ALLOWED_ORIGINS"] = " http://a.test , ,http://b.test "
	setEnv(t, env)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if want := []string{"http://a.test", "http://b.test"}; !slices.Equal(cfg.CORSAllowedOrigins, want) {
		t.Errorf("CORSAllowedOrigins = %v, want %v", cfg.CORSAllowedOrigins, want)
	}
}

func TestDatabaseURLEscapesCredentials(t *testing.T) {
	cfg := Config{DBHost: "db", DBPort: 5432, DBUser: "bia", DBPassword: "p@ss:w/rd?#", DBName: "bia_energy"}

	u, err := url.Parse(cfg.DatabaseURL())
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	if pw, _ := u.User.Password(); pw != cfg.DBPassword {
		t.Errorf("password = %q, want %q", pw, cfg.DBPassword)
	}
	if u.Host != "db:5432" {
		t.Errorf("host = %q, want db:5432", u.Host)
	}
	if u.Path != "/bia_energy" {
		t.Errorf("path = %q, want /bia_energy", u.Path)
	}
	if got := u.Query().Get("sslmode"); got != "disable" {
		t.Errorf("sslmode = %q, want disable", got)
	}
}
