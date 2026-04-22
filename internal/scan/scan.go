package scan

import (
	"bytes"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/srcodee/comot/internal/model"
)

type compiledPattern struct {
	def model.PatternDefinition
	re  *regexp.Regexp
}

func Run(resource model.Resource, patterns []model.PatternDefinition, dedup bool) ([]model.ScanResult, error) {
	compiled := make([]compiledPattern, 0, len(patterns))
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern.Regex)
		if err != nil {
			return nil, fmt.Errorf("invalid regex %q: %w", pattern.Regex, err)
		}
		compiled = append(compiled, compiledPattern{def: pattern, re: re})
	}

	lines := bytes.Split(resource.Body, []byte("\n"))
	results := make([]model.ScanResult, 0)
	seen := map[string]struct{}{}

	for idx, line := range lines {
		text := string(line)
		for _, pattern := range compiled {
			matches := pattern.re.FindAllString(text, -1)
			for _, match := range matches {
				result := model.ScanResult{
					Pattern:        pattern.def.Regex,
					PatternName:    pattern.def.Name,
					PatternSource:  pattern.def.Source,
					MatchedValue:   match,
					Context:        snippetAroundMatch(text, match, 80),
					TargetURL:      resource.TargetURL,
					ResourceURL:    resource.FinalURL,
					ResourceKind:   inferResourceKind(resource),
					DiscoveredFrom: resource.ParentURL,
					Line:           idx + 1,
					Status:         resource.StatusCode,
					ContentType:    resource.ContentType,
				}
				if dedup {
					key := strings.Join([]string{
						result.Pattern,
						result.MatchedValue,
						result.ResourceURL,
						fmt.Sprintf("%d", result.Line),
					}, "|")
					if _, ok := seen[key]; ok {
						continue
					}
					seen[key] = struct{}{}
				}
				results = append(results, result)
			}
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].ResourceURL == results[j].ResourceURL {
			if results[i].Line == results[j].Line {
				return results[i].MatchedValue < results[j].MatchedValue
			}
			return results[i].Line < results[j].Line
		}
		return results[i].ResourceURL < results[j].ResourceURL
	})

	return results, nil
}

func inferResourceKind(resource model.Resource) string {
	ct := strings.ToLower(resource.ContentType)
	switch {
	case strings.Contains(ct, "html"):
		return "html"
	case strings.Contains(ct, "javascript"), strings.Contains(ct, "ecmascript"):
		return "js"
	case strings.Contains(ct, "json"):
		return "json"
	case strings.Contains(ct, "xml"):
		return "xml"
	case strings.Contains(ct, "css"):
		return "css"
	case strings.Contains(ct, "text"):
		return "text"
	}

	switch strings.ToLower(path.Ext(resource.FinalURL)) {
	case ".html", ".htm":
		return "html"
	case ".js", ".mjs":
		return "js"
	case ".json":
		return "json"
	case ".xml":
		return "xml"
	case ".css":
		return "css"
	case ".map":
		return "map"
	case ".txt":
		return "text"
	default:
		return "unknown"
	}
}

func snippetAroundMatch(text, match string, radius int) string {
	if radius <= 0 || match == "" {
		return strings.TrimSpace(text)
	}
	index := strings.Index(text, match)
	if index < 0 {
		return strings.TrimSpace(text)
	}
	start := index - radius
	if start < 0 {
		start = 0
	}
	end := index + len(match) + radius
	if end > len(text) {
		end = len(text)
	}
	snippet := text[start:end]
	snippet = strings.TrimSpace(snippet)
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(text) {
		snippet = snippet + "..."
	}
	return snippet
}
