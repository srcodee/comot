package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
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

func TestParseRequestSpecsKeepsPlainURLListsCompatible(t *testing.T) {
	raw := "https://example.com/one\nhttps://example.com/two\n"

	specs, err := parseRequestSpecs(raw)
	if err != nil {
		t.Fatalf("parseRequestSpecs returned error: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("expected 2 request specs, got %d", len(specs))
	}
	if specs[0].URL != "https://example.com/one" || specs[1].URL != "https://example.com/two" {
		t.Fatalf("unexpected URLs: %#v", specs)
	}
	if specs[0].Method != http.MethodGet || specs[1].Method != http.MethodGet {
		t.Fatalf("expected GET for plain URL specs, got %#v", specs)
	}
}

func TestParseRequestSpecsParsesRawHTTPBlock(t *testing.T) {
	raw := "POST /home HTTP/1.1\r\nHost: example.com\r\nUser-Agent: test-agent\r\nCookie: a=b\r\n\r\nhello=world"

	specs, err := parseRequestSpecs(raw)
	if err != nil {
		t.Fatalf("parseRequestSpecs returned error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 request spec, got %d", len(specs))
	}
	spec := specs[0]
	if spec.Method != http.MethodPost {
		t.Fatalf("expected POST method, got %q", spec.Method)
	}
	if spec.URL != "https://example.com/home" {
		t.Fatalf("expected https://example.com/home, got %q", spec.URL)
	}
	if spec.Headers.Get("User-Agent") != "test-agent" {
		t.Fatalf("expected User-Agent header to be preserved")
	}
	if spec.Headers.Get("Cookie") != "a=b" {
		t.Fatalf("expected Cookie header to be preserved")
	}
	if string(spec.Body) != "hello=world" {
		t.Fatalf("expected body to be preserved, got %q", string(spec.Body))
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

func TestListFileSupportsRawHTTPRequests(t *testing.T) {
	var (
		mu           sync.Mutex
		gotMethod    string
		gotUserAgent string
		gotCookie    string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/home" {
			http.NotFound(w, r)
			return
		}
		mu.Lock()
		gotMethod = r.Method
		gotUserAgent = r.Header.Get("User-Agent")
		gotCookie = r.Header.Get("Cookie")
		mu.Unlock()
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "needle")
	}))
	defer server.Close()

	parsedURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse returned error: %v", err)
	}

	listPath := filepath.Join(t.TempDir(), "requests.txt")
	rawRequest := strings.Join([]string{
		"GET /home HTTP/1.1",
		"Host: " + parsedURL.Host,
		"User-Agent: raw-client/1.0",
		"Cookie: session=abc123",
		"",
	}, "\n")
	if err := os.WriteFile(listPath, []byte(rawRequest), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	if err := executeCommand(t, "-l", listPath, "-p", "needle"); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if gotMethod != http.MethodGet {
		t.Fatalf("expected GET method, got %q", gotMethod)
	}
	if gotUserAgent != "raw-client/1.0" {
		t.Fatalf("expected User-Agent header to be forwarded, got %q", gotUserAgent)
	}
	if gotCookie != "session=abc123" {
		t.Fatalf("expected Cookie header to be forwarded, got %q", gotCookie)
	}
}

func TestWildcardScopeCanDriveDiscoveryForRawRequestSeed(t *testing.T) {
	var (
		mu             sync.Mutex
		hits           = map[string]int{}
		childCookie    string
		childUserAgent string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits[r.URL.Path]++
		mu.Unlock()

		switch r.URL.Path {
		case "/home":
			w.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(w, `<a href="/child">child</a>`)
		case "/child":
			mu.Lock()
			childCookie = r.Header.Get("Cookie")
			childUserAgent = r.Header.Get("User-Agent")
			mu.Unlock()
			w.Header().Set("Content-Type", "text/plain")
			_, _ = io.WriteString(w, `needle`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	parsedURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse returned error: %v", err)
	}

	listPath := filepath.Join(t.TempDir(), "raw.txt")
	rawRequest := strings.Join([]string{
		"GET /home HTTP/1.1",
		"Host: " + parsedURL.Host,
		"User-Agent: raw-client/1.0",
		"Cookie: session=abc123",
		"",
	}, "\n")
	if err := os.WriteFile(listPath, []byte(rawRequest), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	scope := strings.TrimPrefix(server.URL, "http://") + "/*"
	if err := executeCommand(t, "-u", scope, "-l", listPath, "-d", "-p", "needle"); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if hits["/home"] == 0 {
		t.Fatalf("expected raw request seed path to be fetched")
	}
	if hits["/child"] == 0 {
		t.Fatalf("expected child path to be discovered under wildcard scope")
	}
	if hits["/"] != 0 {
		t.Fatalf("did not expect wildcard scope bootstrap root to be fetched when raw request seed is provided")
	}
	if childCookie != "session=abc123" {
		t.Fatalf("expected cookie to be inherited for discovered child, got %q", childCookie)
	}
	if childUserAgent != "raw-client/1.0" {
		t.Fatalf("expected user-agent to be inherited for discovered child, got %q", childUserAgent)
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
