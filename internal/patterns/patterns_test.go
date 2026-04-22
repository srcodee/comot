package patterns

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadBuiltinsFallsBackToEmbedded(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("mkdir home dir: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir tempdir: %v", err)
	}
	previousHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", homeDir); err != nil {
		t.Fatalf("set HOME: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
		_ = os.Setenv("HOME", previousHome)
	})

	defs, err := LoadBuiltins()
	if err != nil {
		t.Fatalf("LoadBuiltins returned error: %v", err)
	}
	if len(defs) == 0 {
		t.Fatalf("LoadBuiltins returned no definitions")
	}
	generatedPath := filepath.Join(homeDir, ".comot.data", dataFile)
	if _, err := os.Stat(generatedPath); err != nil {
		t.Fatalf("expected generated patterns file at %s: %v", generatedPath, err)
	}
	if !strings.Contains(defs[0].Source, generatedPath) {
		t.Fatalf("expected generated file source, got %q", defs[0].Source)
	}
}

func TestLoadBuiltinsPrefersExternalFile(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, ".comot.data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, dataFile), []byte("custom || custom-regex\n"), 0o644); err != nil {
		t.Fatalf("write patterns file: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir tempdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	defs, err := LoadBuiltins()
	if err != nil {
		t.Fatalf("LoadBuiltins returned error: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(defs))
	}
	if defs[0].Name != "custom" {
		t.Fatalf("expected custom definition, got %q", defs[0].Name)
	}
	if !strings.Contains(defs[0].Source, filepath.Join(".comot.data", dataFile)) {
		t.Fatalf("expected external file source, got %q", defs[0].Source)
	}
}
