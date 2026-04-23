package cli

import (
	"testing"

	"github.com/srcodee/comot/internal/model"
	"github.com/srcodee/comot/internal/target"
)

func TestResolveSaveDirDefaultsToScope(t *testing.T) {
	cfg := model.Config{SaveDir: "hasil"}
	if err := resolveSaveDir(&cfg); err != nil {
		t.Fatalf("resolveSaveDir returned error: %v", err)
	}
	if cfg.SaveMode != "scope" {
		t.Fatalf("unexpected save mode: %q", cfg.SaveMode)
	}
	if cfg.SaveDir != "hasil" {
		t.Fatalf("unexpected save dir: %q", cfg.SaveDir)
	}
}

func TestResolveSaveDirSupportsPrefixedModes(t *testing.T) {
	cfg := model.Config{SaveDir: "full:dumped"}
	if err := resolveSaveDir(&cfg); err != nil {
		t.Fatalf("resolveSaveDir returned error: %v", err)
	}
	if cfg.SaveMode != "full" || cfg.SaveDir != "dumped" {
		t.Fatalf("unexpected resolved save config: mode=%q dir=%q", cfg.SaveMode, cfg.SaveDir)
	}
}

func TestShouldSaveResourceHonorsScopeMode(t *testing.T) {
	scope, err := target.Parse("*.example.com/*")
	if err != nil {
		t.Fatalf("parse target: %v", err)
	}

	if !shouldSaveResource("scope", scope, model.Resource{FinalURL: "https://app.example.com/a"}) {
		t.Fatal("expected in-scope resource to be saved")
	}
	if shouldSaveResource("scope", scope, model.Resource{FinalURL: "https://example.com/"}) {
		t.Fatal("did not expect out-of-scope resource to be saved in scope mode")
	}
	if !shouldSaveResource("full", scope, model.Resource{FinalURL: "https://example.com/"}) {
		t.Fatal("expected full mode to save out-of-scope resource")
	}
}
