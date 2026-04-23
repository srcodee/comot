package discover

import (
	"net/url"
	"testing"
)

func TestOutScopeMatcherCategories(t *testing.T) {
	matcher, err := CompileOutScope([]string{"images", "css", "video"})
	if err != nil {
		t.Fatalf("CompileOutScope returned error: %v", err)
	}

	cases := map[string]bool{
		"https://example.com/a.png":  true,
		"https://example.com/a.css":  true,
		"https://example.com/a.mp4":  true,
		"https://example.com/a.html": false,
	}
	for raw, want := range cases {
		u, _ := url.Parse(raw)
		if got := matcher.Matches(u); got != want {
			t.Fatalf("Matches(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestOutScopeMatcherWildcardPattern(t *testing.T) {
	matcher, err := CompileOutScope([]string{"*.svg.*.img"})
	if err != nil {
		t.Fatalf("CompileOutScope returned error: %v", err)
	}

	u, _ := url.Parse("https://cdn.example.com/icon.svg.large.img")
	if !matcher.Matches(u) {
		t.Fatal("expected wildcard pattern to match")
	}
}
