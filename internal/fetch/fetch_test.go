package fetch

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/srcodee/comot/internal/model"
)

func TestFetchWithSpecDecodesGzipResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Content-Encoding", "gzip")

		gz := gzip.NewWriter(w)
		defer gz.Close()
		_, _ = io.WriteString(gz, "needle")
	}))
	defer server.Close()

	client := New(5*time.Second, 2)
	resource, err := client.FetchWithSpec(model.RequestSpec{
		Method: http.MethodGet,
		URL:    server.URL,
		Headers: http.Header{
			"Accept-Encoding": []string{"gzip"},
		},
	}, server.URL, "")
	if err != nil {
		t.Fatalf("FetchWithSpec returned error: %v", err)
	}
	if string(resource.Body) != "needle" {
		t.Fatalf("expected decoded body, got %q", string(resource.Body))
	}
}
