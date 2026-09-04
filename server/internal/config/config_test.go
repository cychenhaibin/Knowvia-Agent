package config

import (
	"strings"
	"testing"
)

func clearSecurityEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"QQA_ENV", "QQA_SERVER_ADDR", "QQA_JWT_SECRET", "QQA_PYTHON_PROXY_TOKEN", "QQA_PYTHON_PROXY_BASE_URL", "QQA_DEV_USERS"} {
		t.Setenv(key, "")
	}
}

func TestLoadProductionDefaultsFailClosed(t *testing.T) {
	clearSecurityEnv(t)
	cfg := Load()
	if cfg.ServerAddr != "127.0.0.1:8088" {
		t.Fatalf("default server address = %q", cfg.ServerAddr)
	}
	if cfg.JWTSecret != "" || cfg.PythonProxyToken != "" || len(cfg.DevUsers) != 0 {
		t.Fatalf("production defaults include credentials: %#v", cfg)
	}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "QQA_JWT_SECRET") {
		t.Fatalf("expected missing JWT secret error, got %v", err)
	}
}

func TestValidateRejectsDevelopmentCredentialsInProduction(t *testing.T) {
	clearSecurityEnv(t)
	t.Setenv("QQA_JWT_SECRET", "quickque-agent-dev-secret")
	t.Setenv("QQA_DEV_USERS", "admin:admin123")
	cfg := Load()
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected production validation to reject development credentials")
	}
}

func TestDevelopmentModeAllowsExplicitDevelopmentDefaults(t *testing.T) {
	clearSecurityEnv(t)
	t.Setenv("QQA_ENV", "development")
	cfg := Load()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("development defaults should validate: %v", err)
	}
	if cfg.JWTSecret == "" || cfg.PythonProxyToken == "" || len(cfg.DevUsers) != 1 {
		t.Fatalf("development defaults missing: %#v", cfg)
	}
}

func TestValidateRequiresProxyTokenWhenProxyEnabled(t *testing.T) {
	clearSecurityEnv(t)
	t.Setenv("QQA_JWT_SECRET", "a-long-production-secret-that-is-not-a-default")
	t.Setenv("QQA_PYTHON_PROXY_BASE_URL", "http://127.0.0.1:8000")
	cfg := Load()
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "QQA_PYTHON_PROXY_TOKEN") {
		t.Fatalf("expected missing proxy token error, got %v", err)
	}
}
