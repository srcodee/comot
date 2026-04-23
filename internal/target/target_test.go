package target

import "testing"

func TestParseLiteralURL(t *testing.T) {
	spec, err := Parse("https://example.com/api")
	if err != nil {
		t.Fatalf("parse target: %v", err)
	}

	if got, want := len(spec.BootstrapSeeds), 1; got != want {
		t.Fatalf("bootstrap seed count = %d, want %d", got, want)
	}
	if got, want := spec.BootstrapSeeds[0], "https://example.com/api"; got != want {
		t.Fatalf("bootstrap seed = %q, want %q", got, want)
	}
	if !spec.Matches("https://example.com/api") {
		t.Fatalf("expected exact URL to match")
	}
	if spec.Matches("http://example.com/api") {
		t.Fatalf("did not expect alternate scheme to match")
	}
}

func TestParseLiteralURLWithPort(t *testing.T) {
	spec, err := Parse("http://127.0.0.1:8080/api")
	if err != nil {
		t.Fatalf("parse target: %v", err)
	}

	if !spec.Matches("http://127.0.0.1:8080/api") {
		t.Fatalf("expected exact host and port to match")
	}
	if spec.Matches("http://127.0.0.1/api") {
		t.Fatalf("did not expect missing port to match")
	}
}

func TestParseWildcardHostDualScheme(t *testing.T) {
	spec, err := Parse("*.google.com")
	if err != nil {
		t.Fatalf("parse target: %v", err)
	}

	if got, want := spec.BootstrapSeeds[0], "https://google.com/"; got != want {
		t.Fatalf("first seed = %q, want %q", got, want)
	}
	if got, want := spec.BootstrapSeeds[1], "http://google.com/"; got != want {
		t.Fatalf("second seed = %q, want %q", got, want)
	}

	cases := map[string]bool{
		"https://www.google.com/":   true,
		"http://mail.google.com/x":  true,
		"https://google.com/":       false,
		"https://a.b.google.com/":   false,
		"https://www.google.co.id/": false,
		"mailto:test@google.com":    false,
	}
	for raw, want := range cases {
		if got := spec.Matches(raw); got != want {
			t.Fatalf("Matches(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestParseWildcardPath(t *testing.T) {
	spec, err := Parse("https://*.google.com/meong/*")
	if err != nil {
		t.Fatalf("parse target: %v", err)
	}

	if !spec.Matches("https://www.google.com/meong/a/b") {
		t.Fatalf("expected nested path to match")
	}
	if spec.Matches("https://www.google.com/kucing/a") {
		t.Fatalf("did not expect other path to match")
	}
	if got, want := spec.BootstrapSeeds[0], "https://google.com/meong/"; got != want {
		t.Fatalf("bootstrap seed = %q, want %q", got, want)
	}
}

func TestParseMixedHostLabels(t *testing.T) {
	spec, err := Parse("id.*.google.com")
	if err != nil {
		t.Fatalf("parse target: %v", err)
	}

	if !spec.Matches("https://id.mail.google.com/") {
		t.Fatalf("expected host pattern to match")
	}
	if spec.Matches("https://mail.google.com/") {
		t.Fatalf("did not expect shorter host to match")
	}
	if spec.Matches("https://id.a.b.google.com/") {
		t.Fatalf("did not expect extra labels to match")
	}
}
