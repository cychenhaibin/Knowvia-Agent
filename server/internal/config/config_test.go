package config

import "testing"

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
	t.Setenv("QQA_FLUXA_PAID_ORIGIN", "https://paid.example")
	t.Setenv("QQA_FLUXA_FREE_ORIGIN", "https://free.example")

	cfg := Load()
	if cfg.FluxAPaidOrigin != "https://paid.example" {
		t.Fatalf("paid origin = %q", cfg.FluxAPaidOrigin)
	}
	if cfg.FluxAFreeOrigin != "https://free.example" {
		t.Fatalf("free origin = %q", cfg.FluxAFreeOrigin)
	}
}
