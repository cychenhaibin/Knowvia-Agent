package config

import (
	"encoding/base64"
	"testing"
)

func TestDecodeFluxACredentialsKeyRejectsMissingOrWrongLength(t *testing.T) {
	for _, raw := range []string{"", base64.StdEncoding.EncodeToString(make([]byte, 31))} {
		_, err := DecodeFluxACredentialsKey(raw)
		if err == nil {
			t.Fatalf("key %q unexpectedly accepted", raw)
		}
	}
}

func TestLoadFluxAOriginsUsesDefaults(t *testing.T) {
	t.Setenv("QQA_FLUXA_PAID_ORIGIN", "")
	t.Setenv("QQA_FLUXA_FREE_ORIGIN", "")

	cfg := Load()
	if cfg.FluxAPaidOrigin != "https://fluxa.camila.qzz.io" {
		t.Fatalf("paid origin = %q", cfg.FluxAPaidOrigin)
	}
	if cfg.FluxAFreeOrigin != "https://free.camila.qzz.io" {
		t.Fatalf("free origin = %q", cfg.FluxAFreeOrigin)
	}
}

func TestLoadFluxAOriginsUsesEnvironment(t *testing.T) {
	t.Setenv("QQA_FLUXA_PAID_ORIGIN", "  HTTPS://paid.example:443/ ")
	t.Setenv("QQA_FLUXA_FREE_ORIGIN", "https://free.example/")

	cfg := Load()
	if cfg.FluxAPaidOrigin != "https://paid.example:443" {
		t.Fatalf("paid origin = %q", cfg.FluxAPaidOrigin)
	}
	if cfg.FluxAFreeOrigin != "https://free.example" {
		t.Fatalf("free origin = %q", cfg.FluxAFreeOrigin)
	}
}

func TestLoadFluxAOriginsRejectsUnsafeConfiguration(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "http scheme", value: "http://paid.example"},
		{name: "path", value: "https://paid.example/api"},
		{name: "userinfo", value: "https://user:pass@paid.example"},
		{name: "query", value: "https://paid.example?token=secret"},
		{name: "fragment", value: "https://paid.example#section"},
		{name: "empty fragment", value: "https://paid.example#"},
		{name: "missing host", value: "https:///"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("QQA_FLUXA_PAID_ORIGIN", tt.value)
			t.Setenv("QQA_FLUXA_FREE_ORIGIN", tt.value)

			cfg := Load()
			if cfg.FluxAPaidOrigin != "" {
				t.Fatalf("paid origin = %q, want fail-closed empty value", cfg.FluxAPaidOrigin)
			}
			if cfg.FluxAFreeOrigin != "" {
				t.Fatalf("free origin = %q, want fail-closed empty value", cfg.FluxAFreeOrigin)
			}
		})
	}
}
