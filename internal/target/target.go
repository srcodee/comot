package target

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

type Spec struct {
	Raw            string
	Schemes        []string
	HostLabels     []string
	PathPattern    string
	Wildcard       bool
	Port           string
	hostExact      string
	pathRegex      *regexp.Regexp
	BootstrapSeeds []string
}

func Parse(raw string) (Spec, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Spec{}, fmt.Errorf("empty target")
	}

	scheme, rest := splitScheme(raw)
	host, pathPattern, hasExplicitPath, err := splitHostPath(rest)
	if err != nil {
		return Spec{}, err
	}

	host = strings.TrimSpace(host)
	if host == "" {
		return Spec{}, fmt.Errorf("target %q is missing host", raw)
	}

	hostName, port := splitHostPort(host)
	hostLabels := strings.Split(strings.ToLower(hostName), ".")
	for _, label := range hostLabels {
		if label == "" {
			return Spec{}, fmt.Errorf("target %q has invalid host", raw)
		}
	}

	schemes := []string{"https", "http"}
	if scheme != "" {
		schemes = []string{strings.ToLower(scheme)}
	}

	pathRegex, err := compilePath(pathPattern)
	if err != nil {
		return Spec{}, err
	}

	if !hasExplicitPath {
		pathPattern = "/*"
		pathRegex, err = compilePath(pathPattern)
		if err != nil {
			return Spec{}, err
		}
	}

	spec := Spec{
		Raw:         raw,
		Schemes:     schemes,
		HostLabels:  hostLabels,
		PathPattern: pathPattern,
		Wildcard:    strings.Contains(raw, "*"),
		Port:        port,
		hostExact:   strings.ToLower(hostName),
		pathRegex:   pathRegex,
	}
	spec.BootstrapSeeds = buildBootstrapSeeds(spec)
	return spec, nil
}

func (s Spec) Matches(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return s.MatchURL(parsed)
}

func (s Spec) MatchURL(candidate *url.URL) bool {
	if candidate == nil || candidate.Hostname() == "" {
		return false
	}

	if !containsFold(s.Schemes, candidate.Scheme) {
		return false
	}
	if !s.matchHost(candidate) {
		return false
	}

	pathValue := candidate.EscapedPath()
	if pathValue == "" {
		pathValue = "/"
	}
	return s.pathRegex.MatchString(pathValue)
}

func (s Spec) matchHost(candidate *url.URL) bool {
	host := strings.ToLower(candidate.Hostname())
	if s.Port != "" && candidate.Port() != s.Port {
		return false
	}
	if s.Port == "" && candidate.Port() != "" && !strings.Contains(s.Raw, "*") {
		return false
	}
	if !strings.Contains(s.hostExact, "*") {
		return host == s.hostExact
	}

	labels := strings.Split(host, ".")
	if len(labels) != len(s.HostLabels) {
		return false
	}
	for idx, want := range s.HostLabels {
		if want == "*" {
			if labels[idx] == "" {
				return false
			}
			continue
		}
		if labels[idx] != want {
			return false
		}
	}
	return true
}

func splitScheme(raw string) (string, string) {
	if idx := strings.Index(raw, "://"); idx >= 0 {
		return raw[:idx], raw[idx+3:]
	}
	return "", raw
}

func splitHostPort(host string) (string, string) {
	if strings.Count(host, ":") == 0 {
		return host, ""
	}
	parsedHost, parsedPort, err := net.SplitHostPort(host)
	if err == nil {
		return parsedHost, parsedPort
	}
	return host, ""
}

func splitHostPath(raw string) (string, string, bool, error) {
	if raw == "" {
		return "", "", false, fmt.Errorf("missing target body")
	}
	if strings.HasPrefix(raw, "/") {
		return "", "", false, fmt.Errorf("target %q must include host", raw)
	}
	if idx := strings.Index(raw, "/"); idx >= 0 {
		return raw[:idx], normalizePathPattern(raw[idx:]), true, nil
	}
	return raw, "/", false, nil
}

func normalizePathPattern(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "/"
	}
	if !strings.HasPrefix(raw, "/") {
		raw = "/" + raw
	}
	return raw
}

func compilePath(pattern string) (*regexp.Regexp, error) {
	if pattern == "" {
		pattern = "/"
	}

	var builder strings.Builder
	builder.WriteString("^")
	for _, r := range pattern {
		if r == '*' {
			builder.WriteString(".*")
			continue
		}
		builder.WriteString(regexp.QuoteMeta(string(r)))
	}
	builder.WriteString("$")
	return regexp.Compile(builder.String())
}

func buildBootstrapSeeds(spec Spec) []string {
	host := bootstrapHost(spec.HostLabels)
	if host == "" {
		host = spec.hostExact
	}
	if spec.Port != "" {
		host = net.JoinHostPort(host, spec.Port)
	}

	path := bootstrapPath(spec.PathPattern)
	seeds := make([]string, 0, len(spec.Schemes))
	for _, scheme := range spec.Schemes {
		seeds = append(seeds, scheme+"://"+host+path)
	}
	return seeds
}

func bootstrapHost(labels []string) string {
	start := len(labels)
	for i := len(labels) - 1; i >= 0; i-- {
		if labels[i] == "*" {
			break
		}
		start = i
	}
	if start >= len(labels) {
		return ""
	}
	return strings.Join(labels[start:], ".")
}

func bootstrapPath(pattern string) string {
	if pattern == "" || pattern == "/" {
		return "/"
	}
	if idx := strings.Index(pattern, "*"); idx >= 0 {
		pattern = pattern[:idx]
	}
	if pattern == "" {
		return "/"
	}
	if !strings.HasPrefix(pattern, "/") {
		pattern = "/" + pattern
	}
	return pattern
}

func containsFold(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(value, want) {
			return true
		}
	}
	return false
}
