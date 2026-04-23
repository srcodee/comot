package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/srcodee/comot/internal/model"
	"github.com/srcodee/comot/internal/save"
)

func TestWildcardTargetAutoDiscoversWithoutFlag(t *testing.T) {
	var mu sync.Mutex
	hits := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits[r.URL.Path]++
		mu.Unlock()

		switch r.URL.Path {
		case "/foo/":
			w.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(w, `<a href="/foo/api-child">child</a>`)
		case "/foo/api-child":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = io.WriteString(w, `needle`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	target := strings.TrimPrefix(server.URL, "http://") + "/foo/*"
	if err := executeCommand(t, "-u", target, "-p", "needle"); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if hits["/foo/"] == 0 {
		t.Fatalf("expected seed path to be fetched")
	}
	if hits["/foo/api-child"] == 0 {
		t.Fatalf("expected wildcard target to auto-discover child path")
	}
}

func TestWildcardTargetFollowsPHPLikePaths(t *testing.T) {
	var mu sync.Mutex
	hits := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits[r.URL.Path]++
		mu.Unlock()

		switch r.URL.Path {
		case "/foo/":
			w.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(w, `<a href="/foo/assets/harga.php">harga</a>`)
		case "/foo/assets/harga.php":
			w.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(w, `needle`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	target := strings.TrimPrefix(server.URL, "http://") + "/foo/*"
	if err := executeCommand(t, "-u", target, "-p", "needle"); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if hits["/foo/assets/harga.php"] == 0 {
		t.Fatalf("expected wildcard target to follow php-like path")
	}
}

func TestLiteralTargetNeedsDiscoverFlag(t *testing.T) {
	t.Run("without-discover", func(t *testing.T) {
		hits := runLiteralTargetScenario(t, false)
		if hits["/"] == 0 {
			t.Fatalf("expected seed path to be fetched")
		}
		if hits["/api-child"] != 0 {
			t.Fatalf("did not expect child path without -d, got %d", hits["/api-child"])
		}
	})

	t.Run("with-discover", func(t *testing.T) {
		hits := runLiteralTargetScenario(t, true)
		if hits["/"] == 0 {
			t.Fatalf("expected seed path to be fetched")
		}
		if hits["/api-child"] == 0 {
			t.Fatalf("expected child path with -d")
		}
	})
}

func TestWildcardTargetFallsBackToHTTPWhenHTTPSFails(t *testing.T) {
	var mu sync.Mutex
	hits := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits[r.URL.Path]++
		mu.Unlock()

		switch r.URL.Path {
		case "/foo/":
			w.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(w, `<a href="/foo/api-child">child</a>`)
		case "/foo/api-child":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = io.WriteString(w, `needle`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	target := strings.TrimPrefix(server.URL, "http://") + "/foo/*"
	if err := executeCommand(t, "-u", target, "-p", "needle"); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if hits["/foo/"] == 0 {
		t.Fatalf("expected HTTP seed to be fetched after HTTPS fallback")
	}
	if hits["/foo/api-child"] == 0 {
		t.Fatalf("expected discovered child to be fetched after HTTP fallback")
	}
}

func TestMaxCrawlLimitsWildcardDiscovery(t *testing.T) {
	var mu sync.Mutex
	hits := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits[r.URL.Path]++
		mu.Unlock()

		switch r.URL.Path {
		case "/foo/":
			w.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(w, `<a href="/foo/api-one">one</a><a href="/foo/api-two">two</a>`)
		case "/foo/api-one", "/foo/api-two":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = io.WriteString(w, `needle`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	target := strings.TrimPrefix(server.URL, "http://") + "/foo/*"
	if err := executeCommand(t, "-u", target, "-m", "3", "-p", "needle"); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if hits["/foo/"] == 0 {
		t.Fatalf("expected seed path to be fetched")
	}
	childFetches := hits["/foo/api-one"] + hits["/foo/api-two"]
	if childFetches != 1 {
		t.Fatalf("expected only one child fetch with --max-crawl 3, got %d", childFetches)
	}
}

func TestHistoryDirScansSavedResources(t *testing.T) {
	dir := t.TempDir()
	saver, err := save.New(dir)
	if err != nil {
		t.Fatalf("save.New returned error: %v", err)
	}
	if err := saver.Save(model.Resource{
		FinalURL:    "https://example.com/index.html",
		ContentType: "text/html",
		StatusCode:  200,
		Body:        []byte("needle"),
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if err := saver.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	stdout, err := executeCommandOutput(t, "--history-dir", dir, "-p", "needle")
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	if !strings.Contains(stdout, "https://example.com/index.html") {
		t.Fatalf("expected history scan output to contain saved URL, got %q", stdout)
	}
}

func TestWildcardTargetHonorsOutScopeCategories(t *testing.T) {
	var mu sync.Mutex
	hits := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits[r.URL.Path]++
		mu.Unlock()

		switch r.URL.Path {
		case "/foo/":
			w.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(w, `<a href="/foo/pic.png">img</a><a href="/foo/app.js">js</a>`)
		case "/foo/pic.png", "/foo/app.js":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = io.WriteString(w, `needle`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	target := strings.TrimPrefix(server.URL, "http://") + "/foo/*"
	if err := executeCommand(t, "-u", target, "--out-scope", "images", "-p", "needle"); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if hits["/foo/pic.png"] != 0 {
		t.Fatalf("did not expect image resource to be fetched")
	}
	if hits["/foo/app.js"] == 0 {
		t.Fatalf("expected non-image resource to be fetched")
	}
}

func runLiteralTargetScenario(t *testing.T, discover bool) map[string]int {
	t.Helper()

	var mu sync.Mutex
	hits := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits[r.URL.Path]++
		mu.Unlock()

		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(w, `<a href="/api-child">child</a>`)
		case "/api-child":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = io.WriteString(w, `needle`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	args := []string{"-u", server.URL, "-p", "needle"}
	if discover {
		args = append(args, "-d")
	}
	if err := executeCommand(t, args...); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	result := make(map[string]int, len(hits))
	for path, count := range hits {
		result[path] = count
	}
	return result
}

func executeCommand(t *testing.T, args ...string) error {
	t.Helper()
	_, err := executeCommandOutput(t, args...)
	return err
}

func executeCommandOutput(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := NewRootCommand()
	cmd.SetArgs(args)

	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	stderrRead, stderrWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}

	originalStdout := os.Stdout
	originalStderr := os.Stderr
	os.Stdout = stdoutWrite
	os.Stderr = stderrWrite
	defer func() {
		os.Stdout = originalStdout
		os.Stderr = originalStderr
	}()

	runErr := cmd.Execute()

	_ = stdoutWrite.Close()
	_ = stderrWrite.Close()
	stdoutData, _ := io.ReadAll(stdoutRead)
	_, _ = io.ReadAll(stderrRead)
	_ = stdoutRead.Close()
	_ = stderrRead.Close()

	return string(stdoutData), runErr
}
