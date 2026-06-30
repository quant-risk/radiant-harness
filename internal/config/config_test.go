package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/quant-risk/radiant-harness/v3/internal/config"
)

func writeConfig(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ".radiant.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoad_NoFile_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil Config")
	}
	if cfg.Model != "" || cfg.MaxIter != 0 {
		t.Errorf("expected empty config, got %+v", cfg)
	}
}

func TestLoad_ModelField(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "model: claude-opus-4-8\n")
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Model != "claude-opus-4-8" {
		t.Errorf("expected model 'claude-opus-4-8', got %q", cfg.Model)
	}
}

func TestLoad_MaxIterField(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "max_iter: 10\n")
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.MaxIter != 10 {
		t.Errorf("expected max_iter 10, got %d", cfg.MaxIter)
	}
}

func TestLoad_AllFields(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `model: claude-sonnet-4-6
max_iter: 5
profile: lean
webhook_url: https://hooks.example.com/radiant
fleet_concurrency: 3
fleet_max_retries: 2
auto_route: true
`)
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Model != "claude-sonnet-4-6" {
		t.Errorf("model: %q", cfg.Model)
	}
	if cfg.MaxIter != 5 {
		t.Errorf("max_iter: %d", cfg.MaxIter)
	}
	if cfg.Profile != "lean" {
		t.Errorf("profile: %q", cfg.Profile)
	}
	if cfg.WebhookURL != "https://hooks.example.com/radiant" {
		t.Errorf("webhook_url: %q", cfg.WebhookURL)
	}
	if cfg.FleetConcurrency != 3 {
		t.Errorf("fleet_concurrency: %d", cfg.FleetConcurrency)
	}
	if cfg.FleetMaxRetries != 2 {
		t.Errorf("fleet_max_retries: %d", cfg.FleetMaxRetries)
	}
	if !cfg.AutoRoute {
		t.Error("auto_route should be true")
	}
}

func TestLoad_YmlExtension(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".radiant.yml"), []byte("model: test-model\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Model != "test-model" {
		t.Errorf("expected model 'test-model', got %q", cfg.Model)
	}
}

func TestLoad_InvalidYAML_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "model: {not: valid: yaml:\n")
	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}
