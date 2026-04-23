package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/srcodee/comot/internal/model"
)

func TestResolveOutputWithOutputDirUsesAutoFile(t *testing.T) {
	cfg := model.Config{
		OutputType: model.OutputJSON,
		OutputDir:  "results",
	}

	if err := resolveOutput(&cfg, true); err != nil {
		t.Fatalf("resolveOutput returned error: %v", err)
	}
	if cfg.OutputType != model.OutputJSON {
		t.Fatalf("unexpected output type: %q", cfg.OutputType)
	}
	if !strings.HasPrefix(cfg.OutputFile, "results"+string(filepath.Separator)+"comot-") {
		t.Fatalf("unexpected output file path: %q", cfg.OutputFile)
	}
	if !strings.HasSuffix(cfg.OutputFile, ".json") {
		t.Fatalf("expected json file, got %q", cfg.OutputFile)
	}
}

func TestResolveOutputWithOutputDirAndDefaultOutputType(t *testing.T) {
	cfg := model.Config{
		OutputType: model.OutputPlain,
		OutputDir:  "loot",
	}

	if err := resolveOutput(&cfg, false); err != nil {
		t.Fatalf("resolveOutput returned error: %v", err)
	}
	if !strings.HasPrefix(cfg.OutputFile, "loot"+string(filepath.Separator)+"comot-") {
		t.Fatalf("unexpected output file path: %q", cfg.OutputFile)
	}
	if !strings.HasSuffix(cfg.OutputFile, ".txt") {
		t.Fatalf("expected text file, got %q", cfg.OutputFile)
	}
}

func TestResolveOutputRejectsOutputDirWithCustomFilename(t *testing.T) {
	cfg := model.Config{
		OutputType: "result.csv",
		OutputDir:  "loot",
	}

	if err := resolveOutput(&cfg, true); err == nil {
		t.Fatal("expected error when output-dir is combined with custom filename")
	}
}
