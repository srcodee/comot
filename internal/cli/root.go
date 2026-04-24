package cli

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/srcodee/comot/internal/discover"
	"github.com/srcodee/comot/internal/fetch"
	"github.com/srcodee/comot/internal/interactive"
	"github.com/srcodee/comot/internal/model"
	"github.com/srcodee/comot/internal/output"
	"github.com/srcodee/comot/internal/patterns"
	"github.com/srcodee/comot/internal/progress"
	"github.com/srcodee/comot/internal/save"
	"github.com/srcodee/comot/internal/scan"
	"github.com/srcodee/comot/internal/target"
)

type queueItem struct {
	Request        model.RequestSpec
	URL            string
	TargetURL      string
	DiscoveredFrom string
	Scope          target.Spec
}

const version = "v1.0.3"

func NewRootCommand() *cobra.Command {
	cfg := model.Config{
		Format:       []string{"pattern", "pattern_name", "resource_url", "matched_value"},
		OutputType:   model.OutputPlain,
		Timeout:      15 * time.Second,
		MaxCrawl:     10000,
		MaxRedirects: 5,
		DedupResults: true,
	}

	cmd := &cobra.Command{
		Use:   "comot",
		Short: "Fetch URLs and scan responses with regex patterns",
		Long: `Fetch URLs and scan responses with regex patterns.

Available format fields:
  target_url
  resource_url
  discovered_from
  matched_value
  context
  pattern
  pattern_name
  pattern_source
  resource_kind
  line
  status
  content_type

Built-in pattern file:
  generated on first run at ~/.comot.data/patterns.txt
  local overrides also checked at ./.comot.data/patterns.txt

Wildcard scope notes:
  if -u contains *, discovery is enabled automatically
  discovered URLs are followed only when they still match the target scope
  targets without a scheme try https first and then http

Example:
  comot -u '*.example.com/*' -p 'regex'
  comot -u 'example.com/*' -l requests.txt -d -p 'regex'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, cfg)
		},
		SilenceUsage: true,
	}

	cmd.Flags().StringVarP(&cfg.URL, "url", "u", "", "target URL or wildcard scope")
	cmd.Flags().BoolP("version", "v", false, "print version and exit")
	cmd.Flags().StringVarP(&cfg.ListPath, "list", "l", "", "file containing URLs or raw HTTP request blocks")
	cmd.Flags().BoolVarP(&cfg.UseStdin, "stdin", "I", false, "read URLs or raw HTTP request blocks from stdin")
	cmd.Flags().StringVar(&cfg.HistoryDir, "history-dir", "", "scan previously saved resources from a --save-dir folder")
	cmd.Flags().StringVar(&cfg.HistoryDir, "hd", "", "alias for --history-dir")
	cmd.Flags().StringSliceVarP(&cfg.Patterns, "pattern", "p", nil, "regex pattern, repeatable")
	cmd.Flags().StringSliceP("builtin", "b", nil, "built-in pattern name, repeatable")
	cmd.Flags().StringP("format", "f", "pattern,pattern_name,resource_url,matched_value", "output fields in order")
	cmd.Flags().StringVarP(&cfg.OutputType, "output", "o", model.OutputPlain, "export target: plain|json|csv for auto file, or filename/path like result.csv; terminal stays plain")
	cmd.Flags().StringVar(&cfg.OutputDir, "output-dir", "", "directory for auto-named export files")
	cmd.Flags().StringVar(&cfg.SaveDir, "save-dir", "", "directory for saving fetched resource bodies; accepts dir, scope:dir, or full:dir")
	cmd.Flags().StringVar(&cfg.SaveDir, "sd", "", "alias for --save-dir")
	cmd.Flags().StringSliceVar(&cfg.OutScope, "out-scope", nil, "exclude discovered URLs by category or wildcard pattern, e.g. images,css,video or *.svg.*.img")
	cmd.Flags().StringSliceVar(&cfg.OutScope, "os", nil, "alias for --out-scope")
	cmd.Flags().DurationVarP(&cfg.Timeout, "timeout", "t", 15*time.Second, "HTTP timeout, e.g. 10s")
	cmd.Flags().BoolVarP(&cfg.Discover, "discover", "d", false, "discover URLs from responses and recursively scan those still matching the target scope")
	cmd.Flags().IntVarP(&cfg.MaxCrawl, "max-crawl", "m", 10000, "maximum number of resources to crawl when discovery is enabled")
	cmd.Flags().BoolVarP(&cfg.DedupResults, "dedup", "D", true, "deduplicate identical results")
	cmd.Flags().BoolVarP(&cfg.AllowOffDomain, "allow-off-domain", "a", false, "allow discovery outside the target scope")

	return cmd
}

func run(cmd *cobra.Command, cfg model.Config) error {
	showVersion, err := cmd.Flags().GetBool("version")
	if err != nil {
		return err
	}
	if showVersion {
		_, err := fmt.Fprintln(os.Stdout, version)
		return err
	}

	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}
	cfg.Format, err = parseFormat(format)
	if err != nil {
		return err
	}
	if err := resolveSaveDir(&cfg); err != nil {
		return err
	}
	if err := resolveOutput(&cfg, cmd.Flags().Changed("output")); err != nil {
		return err
	}

	builtinNames, err := cmd.Flags().GetStringSlice("builtin")
	if err != nil {
		return err
	}

	builtins, err := patterns.LoadBuiltins()
	if err != nil {
		return err
	}

	for _, def := range patterns.FindByNames(builtins, builtinNames) {
		cfg.Patterns = append(cfg.Patterns, def.Regex)
		cfg.PatternDefs = appendPattern(cfg.PatternDefs, def)
	}
	for _, pattern := range cfg.Patterns {
		cfg.PatternDefs = appendPattern(cfg.PatternDefs, model.PatternDefinition{
			Name:   "custom",
			Regex:  pattern,
			Source: "custom:cli --pattern",
		})
	}

	if shouldEnterInteractive(cmd, cfg) {
		cfg, err = interactive.Complete(cfg, builtins)
		if err != nil {
			return err
		}
	}

	if len(cfg.PatternDefs) == 0 {
		return errors.New("no pattern provided")
	}

	terminalWriter, err := output.NewWriter(os.Stdout, cfg.Format, model.OutputPlain, true, true)
	if err != nil {
		return err
	}
	defer terminalWriter.Close()

	var exportWriter *output.Writer
	var file *os.File
	if cfg.OutputFile != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.OutputFile), 0o755); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
		file, err = os.Create(cfg.OutputFile)
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}
		defer file.Close()
		exportWriter, err = output.NewWriter(file, cfg.Format, cfg.OutputType, false, false)
		if err != nil {
			return err
		}
		defer exportWriter.Close()
	}

	resourceSaver, err := save.New(cfg.SaveDir)
	if err != nil {
		return err
	}
	if resourceSaver != nil {
		defer resourceSaver.Close()
	}

	outScope, err := discover.CompileOutScope(cfg.OutScope)
	if err != nil {
		return err
	}

	if strings.TrimSpace(cfg.HistoryDir) != "" {
		if len(cfg.HistoryDirs) == 0 {
			cfg.HistoryDirs = []string{cfg.HistoryDir}
		}
		return runHistoryScan(cfg, terminalWriter, exportWriter)
	}

	targets, err := collectRequests(cfg)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return errors.New("no target URL provided")
	}
	if cfg.MaxCrawl <= 0 {
		return errors.New("max-crawl must be greater than 0")
	}

	client := fetch.New(cfg.Timeout, cfg.MaxRedirects)
	tracker := progress.New(os.Stderr, true)
	defer tracker.Finish()

	seenResources := make(map[string]struct{})
	queue := make([]queueItem, 0, len(targets))
	autoDiscover := cfg.Discover
	scopeOverride := strings.TrimSpace(cfg.URL)
	if cfg.ListPath == "" && !cfg.UseStdin {
		scopeOverride = ""
	}
	for _, reqSpec := range targets {
		scopeTarget := reqSpec.URL
		if scopeOverride != "" {
			scopeTarget = scopeOverride
		}

		spec, err := target.Parse(scopeTarget)
		if err != nil {
			return err
		}
		if spec.Wildcard {
			autoDiscover = true
		}

		if scopeOverride != "" {
			current := reqSpec
			if current.Method == "" {
				current.Method = http.MethodGet
			}
			queue = append(queue, queueItem{
				Request:   current,
				URL:       current.URL,
				TargetURL: spec.Raw,
				Scope:     spec,
			})
			continue
		}

		for _, seed := range spec.BootstrapSeeds {
			current := reqSpec
			current.URL = seed
			if current.Method == "" {
				current.Method = http.MethodGet
			}
			queue = append(queue, queueItem{
				Request:   current,
				URL:       seed,
				TargetURL: spec.Raw,
				Scope:     spec,
			})
		}
	}
	tracker.AddTotal(len(queue) - 1)

	for idx := 0; idx < len(queue); idx++ {
		item := queue[idx]
		if _, ok := seenResources[item.URL]; ok {
			continue
		}
		seenResources[item.URL] = struct{}{}

		tracker.Start("fetch " + item.URL)
		resource, err := client.FetchWithSpec(item.Request, item.TargetURL, item.DiscoveredFrom)
		if err != nil {
			if item.DiscoveredFrom != "" || autoDiscover {
				tracker.Advance()
				continue
			}
			return err
		}

		if resourceSaver != nil && shouldSaveResource(cfg.SaveMode, item.Scope, resource) {
			if err := resourceSaver.Save(resource); err != nil {
				return err
			}
		}

		tracker.Start("scan " + resource.FinalURL)
		scanned, err := scan.Run(resource, cfg.PatternDefs, cfg.DedupResults)
		if err != nil {
			return err
		}
		tracker.BeforeOutput()
		if err := terminalWriter.WriteResults(scanned); err != nil {
			return err
		}
		if exportWriter != nil {
			if err := exportWriter.WriteResults(scanned); err != nil {
				return err
			}
		}

		if autoDiscover {
			tracker.Start("discover " + resource.FinalURL)
			related, err := discover.Related(resource, item.Scope, outScope, cfg.AllowOffDomain, item.Scope.Wildcard)
			if err != nil {
				return err
			}
			for _, child := range related {
				if _, ok := seenResources[child.URL]; ok || containsDiscovered(queue[idx+1:], child.URL) {
					continue
				}
				if len(queue) >= cfg.MaxCrawl {
					break
				}
				queue = append(queue, queueItem{
					Request:        inheritedRequest(item.Request, child.URL),
					URL:            child.URL,
					TargetURL:      resource.TargetURL,
					DiscoveredFrom: child.DiscoveredFrom,
					Scope:          item.Scope,
				})
				tracker.AddTotal(1)
			}
		}

		tracker.Advance()
	}
	return nil
}

func runHistoryScan(cfg model.Config, terminalWriter, exportWriter *output.Writer) error {
	for _, dir := range cfg.HistoryDirs {
		resources, err := save.LoadResources(dir)
		if err != nil {
			return err
		}
		for _, resource := range resources {
			scanned, err := scan.Run(resource, cfg.PatternDefs, cfg.DedupResults)
			if err != nil {
				return err
			}
			if err := terminalWriter.WriteResults(scanned); err != nil {
				return err
			}
			if exportWriter != nil {
				if err := exportWriter.WriteResults(scanned); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func collectRequests(cfg model.Config) ([]model.RequestSpec, error) {
	seen := map[string]struct{}{}
	var requests []model.RequestSpec
	add := func(spec model.RequestSpec) {
		if strings.TrimSpace(spec.URL) == "" {
			return
		}
		key := spec.Method + "\x00" + spec.URL + "\x00" + canonicalHeaderKey(spec.Headers)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		requests = append(requests, spec)
	}

	if cfg.URL != "" && cfg.ListPath == "" && !cfg.UseStdin {
		add(model.RequestSpec{Method: http.MethodGet, URL: strings.TrimSpace(cfg.URL)})
	}

	if cfg.ListPath != "" {
		file, err := os.ReadFile(cfg.ListPath)
		if err != nil {
			return nil, fmt.Errorf("open list file: %w", err)
		}
		specs, err := parseRequestSpecs(string(file))
		if err != nil {
			return nil, err
		}
		for _, spec := range specs {
			add(spec)
		}
	}

	if cfg.UseStdin {
		info, err := os.Stdin.Stat()
		if err != nil {
			return nil, err
		}
		if (info.Mode() & os.ModeCharDevice) != 0 {
			return nil, errors.New("--stdin provided but no piped input detected")
		}
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, err
		}
		specs, err := parseRequestSpecs(string(data))
		if err != nil {
			return nil, err
		}
		for _, spec := range specs {
			add(spec)
		}
	}

	return requests, nil
}

func parseRequestSpecs(raw string) ([]model.RequestSpec, error) {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	lines := strings.Split(raw, "\n")
	specs := make([]model.RequestSpec, 0, len(lines))
	for i := 0; i < len(lines); {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}

		if !isRequestLine(line) {
			specs = append(specs, model.RequestSpec{
				Method: http.MethodGet,
				URL:    line,
			})
			i++
			continue
		}

		blockLines := []string{lines[i]}
		i++

		for i < len(lines) {
			current := lines[i]
			trimmed := strings.TrimSpace(current)
			if trimmed == "" {
				blockLines = append(blockLines, current)
				i++
				continue
			}
			if isRequestLine(trimmed) {
				break
			}
			blockLines = append(blockLines, current)
			i++
		}

		spec, err := parseRequestBlock(strings.Join(blockLines, "\n"))
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

func parseRequestBlock(block string) (model.RequestSpec, error) {
	block = strings.TrimSpace(block)
	if block == "" {
		return model.RequestSpec{}, nil
	}
	if !isRequestLine(firstLine(block)) {
		return model.RequestSpec{Method: http.MethodGet, URL: block}, nil
	}

	lines := strings.Split(block, "\n")
	parts := strings.Fields(strings.TrimSpace(lines[0]))
	if len(parts) < 2 {
		return model.RequestSpec{}, fmt.Errorf("invalid request line %q", lines[0])
	}

	headers := make(http.Header)
	bodyStart := len(lines)
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			bodyStart = i + 1
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		headers.Add(strings.TrimSpace(name), strings.TrimSpace(value))
	}

	rawURL, err := buildRawRequestURL(parts[1], headers.Get("Host"))
	if err != nil {
		return model.RequestSpec{}, err
	}

	var body []byte
	if bodyStart < len(lines) {
		body = []byte(strings.Join(lines[bodyStart:], "\n"))
	}

	return model.RequestSpec{
		Method:  strings.ToUpper(parts[0]),
		URL:     rawURL,
		Headers: headers,
		Body:    body,
	}, nil
}

func firstLine(block string) string {
	scanner := bufio.NewScanner(strings.NewReader(block))
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
}

func isRequestLine(line string) bool {
	parts := strings.Fields(line)
	return len(parts) >= 3 && strings.HasPrefix(strings.ToUpper(parts[2]), "HTTP/")
}

func buildRawRequestURL(target, host string) (string, error) {
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		return target, nil
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return "", fmt.Errorf("raw request is missing Host header")
	}
	scheme := "https"
	if strings.HasPrefix(host, "localhost:") || strings.HasPrefix(host, "127.0.0.1:") || strings.HasSuffix(host, ":80") {
		scheme = "http"
	}
	u := &url.URL{Scheme: scheme, Host: host, Path: target}
	if strings.Contains(target, "?") {
		pathValue, queryValue, _ := strings.Cut(target, "?")
		u.Path = pathValue
		u.RawQuery = queryValue
	}
	return u.String(), nil
}

func canonicalHeaderKey(headers http.Header) string {
	if len(headers) == 0 {
		return ""
	}
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder bytes.Buffer
	for _, key := range keys {
		values := append([]string(nil), headers[key]...)
		sort.Strings(values)
		builder.WriteString(key)
		builder.WriteByte(':')
		builder.WriteString(strings.Join(values, ","))
		builder.WriteByte('\n')
	}
	return builder.String()
}

func inheritedRequest(parent model.RequestSpec, urlValue string) model.RequestSpec {
	headers := make(http.Header)
	for _, key := range []string{"User-Agent", "Accept", "Accept-Language", "Accept-Encoding", "Cookie"} {
		if value := parent.Headers.Get(key); value != "" {
			headers.Set(key, value)
		}
	}
	return model.RequestSpec{
		Method:  http.MethodGet,
		URL:     urlValue,
		Headers: headers,
	}
}

func parseFormat(raw string) ([]string, error) {
	parts := strings.Split(raw, ",")
	fields := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := model.AllowedFields[part]; !ok {
			return nil, fmt.Errorf("unsupported format field %q", part)
		}
		fields = append(fields, part)
	}
	if len(fields) == 0 {
		return nil, errors.New("format must contain at least one field")
	}
	return fields, nil
}

func containsDiscovered(items []queueItem, target string) bool {
	for _, item := range items {
		if item.URL == target {
			return true
		}
	}
	return false
}

func appendPattern(defs []model.PatternDefinition, candidate model.PatternDefinition) []model.PatternDefinition {
	for _, def := range defs {
		if def.Regex == candidate.Regex && def.Name == candidate.Name && def.Source == candidate.Source {
			return defs
		}
	}
	return append(defs, candidate)
}

func shouldEnterInteractive(cmd *cobra.Command, cfg model.Config) bool {
	if cmd.Flags().NFlag() == 0 {
		return true
	}

	hasInputSource := cfg.URL != "" || cfg.ListPath != "" || cfg.UseStdin || cfg.HistoryDir != ""
	if !hasInputSource {
		return false
	}

	if len(cfg.PatternDefs) > 0 || len(cfg.Patterns) > 0 {
		return false
	}

	return true
}

func resolveOutput(cfg *model.Config, changed bool) error {
	if !changed && strings.TrimSpace(cfg.OutputDir) == "" {
		cfg.OutputType = model.OutputPlain
		cfg.OutputFile = ""
		return nil
	}

	raw := strings.TrimSpace(cfg.OutputType)
	if raw == "" {
		cfg.OutputType = model.OutputPlain
		cfg.OutputFile = defaultOutputPath(model.OutputPlain, cfg.OutputDir)
		return nil
	}

	switch raw {
	case model.OutputPlain, model.OutputJSON, model.OutputCSV:
		cfg.OutputType = raw
		cfg.OutputFile = defaultOutputPath(raw, cfg.OutputDir)
		return nil
	}

	if strings.TrimSpace(cfg.OutputDir) != "" {
		return errors.New("output-dir cannot be combined with a custom output filename")
	}

	cfg.OutputFile = raw
	ext := strings.ToLower(filepath.Ext(raw))
	switch ext {
	case ".json":
		cfg.OutputType = model.OutputJSON
	case ".csv":
		cfg.OutputType = model.OutputCSV
	default:
		cfg.OutputType = model.OutputPlain
	}
	return nil
}

func resolveSaveDir(cfg *model.Config) error {
	raw := strings.TrimSpace(cfg.SaveDir)
	if raw == "" {
		cfg.SaveMode = ""
		return nil
	}

	mode := "scope"
	dir := raw
	if idx := strings.Index(raw, ":"); idx >= 0 {
		prefix := strings.ToLower(strings.TrimSpace(raw[:idx]))
		switch prefix {
		case "scope", "full":
			mode = prefix
			dir = strings.TrimSpace(raw[idx+1:])
		}
	}
	if dir == "" {
		return errors.New("save-dir must include a folder path")
	}

	cfg.SaveDir = dir
	cfg.SaveMode = mode
	return nil
}

func shouldSaveResource(mode string, scope target.Spec, resource model.Resource) bool {
	switch mode {
	case "", "scope":
		return scope.Matches(resource.FinalURL)
	case "full":
		return true
	default:
		return false
	}
}

func defaultOutputPath(outputType string, dir string) string {
	name := defaultOutputName(outputType)
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return name
	}
	return filepath.Join(dir, name)
}

func defaultOutputName(outputType string) string {
	stamp := time.Now().Format("20060102-150405")
	switch outputType {
	case model.OutputJSON:
		return "comot-" + stamp + ".json"
	case model.OutputCSV:
		return "comot-" + stamp + ".csv"
	default:
		return "comot-" + stamp + ".txt"
	}
}
