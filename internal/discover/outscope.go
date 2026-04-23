package discover

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
)

type OutScopeMatcher struct {
	categories map[string]struct{}
	patterns   []*regexp.Regexp
}

func CompileOutScope(values []string) (OutScopeMatcher, error) {
	matcher := OutScopeMatcher{
		categories: make(map[string]struct{}),
		patterns:   make([]*regexp.Regexp, 0),
	}

	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		lower := strings.ToLower(value)
		switch lower {
		case "images", "image", "css", "video":
			matcher.categories[lower] = struct{}{}
			continue
		}
		re, err := compileWildcardPattern(lower)
		if err != nil {
			return OutScopeMatcher{}, err
		}
		matcher.patterns = append(matcher.patterns, re)
	}

	return matcher, nil
}

func (m OutScopeMatcher) Matches(candidate *url.URL) bool {
	if candidate == nil {
		return false
	}
	if m.matchesCategory(candidate) {
		return true
	}

	raw := strings.ToLower(candidate.String())
	for _, pattern := range m.patterns {
		if pattern.MatchString(raw) {
			return true
		}
	}
	return false
}

func (m OutScopeMatcher) matchesCategory(candidate *url.URL) bool {
	ext := strings.ToLower(path.Ext(candidate.Path))
	if _, ok := m.categories["images"]; ok {
		switch ext {
		case ".jpg", ".jpeg", ".png", ".gif", ".svg", ".webp", ".bmp", ".ico", ".avif":
			return true
		}
	}
	if _, ok := m.categories["css"]; ok && ext == ".css" {
		return true
	}
	if _, ok := m.categories["video"]; ok {
		switch ext {
		case ".mp4", ".mov", ".avi", ".webm", ".mkv", ".m4v":
			return true
		}
	}
	return false
}

func compileWildcardPattern(raw string) (*regexp.Regexp, error) {
	var builder strings.Builder
	builder.WriteString("^")
	for _, r := range raw {
		if r == '*' {
			builder.WriteString(".*")
			continue
		}
		builder.WriteString(regexp.QuoteMeta(string(r)))
	}
	builder.WriteString("$")
	re, err := regexp.Compile(builder.String())
	if err != nil {
		return nil, fmt.Errorf("compile out-scope pattern %q: %w", raw, err)
	}
	return re, nil
}
