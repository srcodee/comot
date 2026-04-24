package fetch

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/srcodee/comot/internal/model"
)

type Client struct {
	http *http.Client
}

func New(timeout time.Duration, maxRedirects int) *Client {
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			return nil
		},
	}
	return &Client{http: client}
}

func (c *Client) Fetch(rawURL string) (model.Resource, error) {
	return c.FetchWithSpec(model.RequestSpec{Method: http.MethodGet, URL: rawURL}, rawURL, "")
}

func (c *Client) FetchWithContext(rawURL, targetURL, parentURL string) (model.Resource, error) {
	return c.FetchWithSpec(model.RequestSpec{Method: http.MethodGet, URL: rawURL}, targetURL, parentURL)
}

func (c *Client) FetchWithSpec(spec model.RequestSpec, targetURL, parentURL string) (model.Resource, error) {
	method := strings.TrimSpace(spec.Method)
	if method == "" {
		method = http.MethodGet
	}

	var bodyReader io.Reader
	if len(spec.Body) > 0 {
		bodyReader = bytes.NewReader(spec.Body)
	}

	req, err := http.NewRequest(method, spec.URL, bodyReader)
	if err != nil {
		return model.Resource{}, fmt.Errorf("build request for %s: %w", spec.URL, err)
	}
	if spec.Headers != nil {
		req.Header = spec.Headers.Clone()
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "comot/0.1")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return model.Resource{}, fmt.Errorf("fetch %s: %w", spec.URL, err)
	}
	defer resp.Body.Close()

	body, err := readDecodedBody(resp)
	if err != nil {
		return model.Resource{}, fmt.Errorf("read body %s: %w", spec.URL, err)
	}

	contentType := resp.Header.Get("Content-Type")
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = contentType[:idx]
	}

	return model.Resource{
		Method:      method,
		URL:         spec.URL,
		FinalURL:    resp.Request.URL.String(),
		TargetURL:   targetURL,
		ParentURL:   parentURL,
		StatusCode:  resp.StatusCode,
		ContentType: strings.TrimSpace(contentType),
		Body:        body,
		Header:      resp.Header.Clone(),
	}, nil
}

func readDecodedBody(resp *http.Response) ([]byte, error) {
	reader := io.Reader(resp.Body)
	encoding := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))

	switch encoding {
	case "", "identity":
	case "gzip", "x-gzip":
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("decode gzip response: %w", err)
		}
		defer gz.Close()
		reader = gz
	case "deflate":
		zr, err := zlib.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("decode deflate response: %w", err)
		}
		defer zr.Close()
		reader = zr
	}

	return io.ReadAll(io.LimitReader(reader, 8<<20))
}
