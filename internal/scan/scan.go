package scan

import (
	"bytes"
	"fmt"
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
					TargetURL:      resource.TargetURL,
					ResourceURL:    resource.FinalURL,
					DiscoveredFrom: resource.ParentURL,
					URL:            resource.FinalURL,
					Line:           idx + 1,
					Status:         resource.StatusCode,
					ContentType:    resource.ContentType,
				}
				if dedup {
					key := strings.Join([]string{
						result.Pattern,
						result.MatchedValue,
						result.URL,
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
		if results[i].URL == results[j].URL {
			if results[i].Line == results[j].Line {
				return results[i].MatchedValue < results[j].MatchedValue
			}
			return results[i].Line < results[j].Line
		}
		return results[i].URL < results[j].URL
	})

	return results, nil
}
