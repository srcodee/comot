package discover

import (
	"bytes"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/srcodee/comot/internal/model"
)

type ScopeMatcher interface {
	MatchURL(candidate *url.URL) bool
}

var skipExtensions = map[string]struct{}{
	".jpg": {}, ".jpeg": {}, ".png": {}, ".gif": {}, ".svg": {}, ".webp": {},
	".mp4": {}, ".mov": {}, ".avi": {}, ".mp3": {}, ".wav": {}, ".woff": {},
	".woff2": {}, ".ttf": {}, ".eot": {}, ".ico": {}, ".pdf": {},
}

var textDiscoveryPatterns = []*regexp.Regexp{
	regexp.MustCompile(`https?://[^\s"'<>\\]+`),
	regexp.MustCompile(`(?:src|href|url)\s*[:=]\s*["']([^"']+)["']`),
	regexp.MustCompile(`["']([^"']+\.(?:js|json|xml|map)(?:\?[^"']*)?)["']`),
	regexp.MustCompile(`["']((?:/|\./|\.\./)[^"']+)["']`),
	regexp.MustCompile(`sourceMappingURL=([^\s"'<>]+)`),
}

func Related(base model.Resource, scope ScopeMatcher, outScope OutScopeMatcher, allowOffDomain bool, aggressive bool) ([]model.DiscoveredResource, error) {
	baseURL, err := url.Parse(base.FinalURL)
	if err != nil {
		return nil, err
	}

	seen := map[string]struct{}{}
	related := make([]model.DiscoveredResource, 0)
	add := func(raw string) {
		addCandidate(baseURL, raw, scope, outScope, allowOffDomain, aggressive, seen, &related, base.FinalURL)
	}

	if strings.Contains(strings.ToLower(base.ContentType), "html") {
		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(base.Body))
		if err != nil {
			return nil, err
		}

		doc.Find("script[src]").Each(func(_ int, sel *goquery.Selection) {
			if src, ok := sel.Attr("src"); ok {
				add(src)
			}
		})

		doc.Find("link[href], a[href]").Each(func(_ int, sel *goquery.Selection) {
			if href, ok := sel.Attr("href"); ok {
				add(href)
			}
		})
	}

	if isTextDiscoverable(base) {
		text := string(base.Body)
		for _, re := range textDiscoveryPatterns {
			matches := re.FindAllStringSubmatch(text, -1)
			for _, match := range matches {
				candidate := match[0]
				if len(match) > 1 && strings.TrimSpace(match[1]) != "" {
					candidate = match[1]
				}
				add(candidate)
			}
		}
	}

	return related, nil
}

func isTextDiscoverable(resource model.Resource) bool {
	ct := strings.ToLower(resource.ContentType)
	if strings.Contains(ct, "html") ||
		strings.Contains(ct, "javascript") ||
		strings.Contains(ct, "json") ||
		strings.Contains(ct, "xml") ||
		strings.Contains(ct, "text") {
		return true
	}

	ext := strings.ToLower(path.Ext(resource.FinalURL))
	switch ext {
	case ".js", ".json", ".xml", ".map", ".txt":
		return true
	default:
		return false
	}
}

func addCandidate(baseURL *url.URL, raw string, scope ScopeMatcher, outScope OutScopeMatcher, allowOffDomain bool, aggressive bool, seen map[string]struct{}, related *[]model.DiscoveredResource, discoveredFrom string) {
	raw = normalizeCandidate(raw)
	if raw == "" {
		return
	}

	abs, err := baseURL.Parse(raw)
	if err != nil || abs.Scheme == "" || abs.Host == "" {
		return
	}
	if !allowOffDomain && scope != nil && !scope.MatchURL(abs) {
		return
	}
	if outScope.Matches(abs) {
		return
	}
	if shouldSkip(abs.Path) {
		return
	}
	if !aggressive && !isRelevant(abs.Path) && !looksStructured(abs.RawQuery) {
		return
	}

	key := abs.String()
	if _, ok := seen[key]; ok {
		return
	}
	seen[key] = struct{}{}
	*related = append(*related, model.DiscoveredResource{
		URL:            key,
		DiscoveredFrom: discoveredFrom,
	})
}

func normalizeCandidate(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, `"'`)
	raw = strings.TrimSuffix(raw, `\`)
	raw = strings.TrimSuffix(raw, `;`)
	raw = strings.TrimSuffix(raw, `,`)
	return raw
}

func shouldSkip(p string) bool {
	ext := strings.ToLower(path.Ext(p))
	_, ok := skipExtensions[ext]
	return ok
}

func isRelevant(p string) bool {
	ext := strings.ToLower(path.Ext(p))
	switch ext {
	case ".js", ".json", ".xml", ".map", ".txt", ".conf", ".config":
		return true
	default:
		lower := strings.ToLower(p)
		return strings.Contains(lower, "api") ||
			strings.Contains(lower, "graphql") ||
			strings.Contains(lower, "swagger") ||
			strings.Contains(lower, "openapi")
	}
}

func looksStructured(q string) bool {
	q = strings.ToLower(q)
	return strings.Contains(q, "json") ||
		strings.Contains(q, "graphql") ||
		strings.Contains(q, "swagger") ||
		strings.Contains(q, "openapi")
}
