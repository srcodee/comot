package fetch

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"comot/internal/model"
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
	return c.FetchWithContext(rawURL, rawURL, "")
}

func (c *Client) FetchWithContext(rawURL, targetURL, parentURL string) (model.Resource, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return model.Resource{}, fmt.Errorf("build request for %s: %w", rawURL, err)
	}
	req.Header.Set("User-Agent", "comot/0.1")

	resp, err := c.http.Do(req)
	if err != nil {
		return model.Resource{}, fmt.Errorf("fetch %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return model.Resource{}, fmt.Errorf("read body %s: %w", rawURL, err)
	}

	contentType := resp.Header.Get("Content-Type")
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = contentType[:idx]
	}

	return model.Resource{
		URL:         rawURL,
		FinalURL:    resp.Request.URL.String(),
		TargetURL:   targetURL,
		ParentURL:   parentURL,
		StatusCode:  resp.StatusCode,
		ContentType: strings.TrimSpace(contentType),
		Body:        body,
		Header:      resp.Header.Clone(),
	}, nil
}
