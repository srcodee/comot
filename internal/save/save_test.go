package save

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/srcodee/comot/internal/model"
)

func TestSaverWritesResourceAndIndex(t *testing.T) {
	dir := t.TempDir()
	saver, err := New(dir)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	err = saver.Save(model.Resource{
		FinalURL:    "https://example.com/assets/app.js?ver=1",
		ContentType: "application/javascript",
		StatusCode:  200,
		Body:        []byte("console.log('x')"),
	})
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(dir, "example.com", "assets", "app__*.js"))
	if err != nil {
		t.Fatalf("glob returned error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 saved file, got %d", len(matches))
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if string(data) != "console.log('x')" {
		t.Fatalf("unexpected saved body: %q", string(data))
	}

	indexData, err := os.ReadFile(filepath.Join(dir, "index.txt"))
	if err != nil {
		t.Fatalf("read index.txt: %v", err)
	}
	index := string(indexData)
	if !strings.Contains(index, "https://example.com/assets/app.js?ver=1") {
		t.Fatalf("index missing URL: %q", index)
	}
	if !strings.Contains(index, "example.com/assets/") {
		t.Fatalf("index missing relative path: %q", index)
	}
	if err := saver.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
}

func TestSaverShortensVeryLongPathSegments(t *testing.T) {
	dir := t.TempDir()
	saver, err := New(dir)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	longPart := strings.Repeat("a", 240)
	err = saver.Save(model.Resource{
		FinalURL:    "https://example.com/" + longPart + "/harga.php",
		ContentType: "text/html",
		StatusCode:  200,
		Body:        []byte("ok"),
	})
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(dir, "example.com", "*", "harga.php"))
	if err != nil {
		t.Fatalf("glob returned error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 saved file for shortened path, got %d", len(matches))
	}
	shortenedDir := filepath.Base(filepath.Dir(matches[0]))
	if len(shortenedDir) > maxPathPartLen {
		t.Fatalf("expected shortened directory name, got len %d", len(shortenedDir))
	}
	if !strings.Contains(shortenedDir, "__") {
		t.Fatalf("expected shortened directory to contain suffix hash, got %q", shortenedDir)
	}
	if err := saver.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
}

func TestLoadResourcesReadsSavedHistory(t *testing.T) {
	dir := t.TempDir()
	saver, err := New(dir)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	err = saver.Save(model.Resource{
		FinalURL:    "https://example.com/index.html",
		ContentType: "text/html",
		StatusCode:  200,
		Body:        []byte("needle"),
	})
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if err := saver.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	resources, err := LoadResources(dir)
	if err != nil {
		t.Fatalf("LoadResources returned error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	if string(resources[0].Body) != "needle" {
		t.Fatalf("unexpected resource body: %q", string(resources[0].Body))
	}
	if resources[0].FinalURL != "https://example.com/index.html" {
		t.Fatalf("unexpected FinalURL: %q", resources[0].FinalURL)
	}
}

func TestLoadResourcesFallsBackWhenIndexMissing(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "example.com"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "example.com", "index.html"), []byte("needle"), 0o644); err != nil {
		t.Fatalf("write html: %v", err)
	}

	resources, err := LoadResources(dir)
	if err != nil {
		t.Fatalf("LoadResources returned error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 fallback resource, got %d", len(resources))
	}
	if resources[0].ContentType != "text/html" {
		t.Fatalf("unexpected fallback content type: %q", resources[0].ContentType)
	}
	if !strings.Contains(resources[0].FinalURL, "example.com/index.html") {
		t.Fatalf("unexpected fallback FinalURL: %q", resources[0].FinalURL)
	}
}

func TestDiscoverHistoryDirsFallsBackToChildFoldersWithoutIndex(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "example.com")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatalf("mkdir child: %v", err)
	}
	if err := os.WriteFile(filepath.Join(child, "index.html"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write child file: %v", err)
	}

	dirs, err := DiscoverHistoryDirs(root)
	if err != nil {
		t.Fatalf("DiscoverHistoryDirs returned error: %v", err)
	}
	if len(dirs) == 0 {
		t.Fatal("expected at least one discovered history dir")
	}
	found := false
	for _, dir := range dirs {
		if dir == child {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected discovered dirs to include %q, got %#v", child, dirs)
	}
}
