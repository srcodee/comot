package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	"github.com/srcodee/comot/internal/scan"
)

type queueItem struct {
	URL            string
	TargetURL      string
	DiscoveredFrom string
}

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
  local overrides also checked at ./.comot.data/patterns.txt`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, cfg)
		},
		SilenceUsage: true,
	}

	cmd.Flags().StringVarP(&cfg.URL, "url", "u", "", "single target URL")
	cmd.Flags().StringVarP(&cfg.ListPath, "list", "l", "", "file containing URLs")
	cmd.Flags().BoolVarP(&cfg.UseStdin, "stdin", "I", false, "read URLs from stdin")
	cmd.Flags().StringSliceVarP(&cfg.Patterns, "pattern", "p", nil, "regex pattern, repeatable")
	cmd.Flags().StringSliceP("builtin", "b", nil, "built-in pattern name, repeatable")
	cmd.Flags().StringP("format", "f", "pattern,pattern_name,resource_url,matched_value", "output fields in order")
	cmd.Flags().StringVarP(&cfg.OutputType, "output", "o", model.OutputPlain, "export target: plain|json|csv for auto file, or filename/path like result.csv; terminal stays plain")
	cmd.Flags().DurationVarP(&cfg.Timeout, "timeout", "t", 15*time.Second, "HTTP timeout, e.g. 10s")
	cmd.Flags().BoolVarP(&cfg.Discover, "discover", "d", false, "recursively discover and scan related resources until exhausted")
	cmd.Flags().IntVarP(&cfg.MaxCrawl, "max-crawl", "m", 10000, "maximum number of resources to crawl when discovery is enabled")
	cmd.Flags().BoolVarP(&cfg.DedupResults, "dedup", "D", true, "deduplicate identical results")
	cmd.Flags().BoolVarP(&cfg.AllowOffDomain, "allow-off-domain", "a", false, "allow discovery outside the original host")

	return cmd
}

func run(cmd *cobra.Command, cfg model.Config) error {
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}
	cfg.Format, err = parseFormat(format)
	if err != nil {
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

	urls, err := collectURLs(cfg)
	if err != nil {
		return err
	}
	if len(urls) == 0 {
		return errors.New("no target URL provided")
	}
	if len(cfg.PatternDefs) == 0 {
		return errors.New("no pattern provided")
	}
	if cfg.MaxCrawl <= 0 {
		return errors.New("max-crawl must be greater than 0")
	}

	client := fetch.New(cfg.Timeout, cfg.MaxRedirects)
	tracker := progress.New(os.Stderr, true)
	defer tracker.Finish()

	terminalWriter, err := output.NewWriter(os.Stdout, cfg.Format, model.OutputPlain, true, true)
	if err != nil {
		return err
	}
	defer terminalWriter.Close()

	var exportWriter *output.Writer
	var file *os.File
	if cfg.OutputFile != "" {
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

	seenResources := make(map[string]struct{})
	queue := make([]queueItem, 0, len(urls))
	for _, url := range urls {
		queue = append(queue, queueItem{
			URL:       url,
			TargetURL: url,
		})
	}
	tracker.AddTotal(len(queue) - 1)

	for idx := 0; idx < len(queue); idx++ {
		item := queue[idx]
		if _, ok := seenResources[item.URL]; ok {
			continue
		}
		seenResources[item.URL] = struct{}{}

		tracker.Start("fetch " + item.URL)
		resource, err := client.FetchWithContext(item.URL, item.TargetURL, item.DiscoveredFrom)
		if err != nil {
			if item.DiscoveredFrom != "" || cfg.Discover {
				tracker.Advance()
				continue
			}
			return err
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

		if cfg.Discover {
			tracker.Start("discover " + resource.FinalURL)
			related, err := discover.Related(resource, cfg.AllowOffDomain)
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
					URL:            child.URL,
					TargetURL:      resource.TargetURL,
					DiscoveredFrom: child.DiscoveredFrom,
				})
				tracker.AddTotal(1)
			}
		}

		tracker.Advance()
	}
	return nil
}

func collectURLs(cfg model.Config) ([]string, error) {
	seen := map[string]struct{}{}
	var urls []string
	add := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		if _, ok := seen[raw]; ok {
			return
		}
		seen[raw] = struct{}{}
		urls = append(urls, raw)
	}

	if cfg.URL != "" {
		add(cfg.URL)
	}

	if cfg.ListPath != "" {
		file, err := os.Open(cfg.ListPath)
		if err != nil {
			return nil, fmt.Errorf("open list file: %w", err)
		}
		defer file.Close()
		if err := scanLines(file, add); err != nil {
			return nil, err
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
		if err := scanLines(os.Stdin, add); err != nil {
			return nil, err
		}
	}

	return urls, nil
}

func scanLines(r io.Reader, add func(string)) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		add(scanner.Text())
	}
	return scanner.Err()
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

	hasInputSource := cfg.URL != "" || cfg.ListPath != "" || cfg.UseStdin
	if !hasInputSource {
		return false
	}

	if len(cfg.PatternDefs) > 0 || len(cfg.Patterns) > 0 {
		return false
	}

	return true
}

func resolveOutput(cfg *model.Config, changed bool) error {
	if !changed {
		cfg.OutputType = model.OutputPlain
		cfg.OutputFile = ""
		return nil
	}

	raw := strings.TrimSpace(cfg.OutputType)
	if raw == "" {
		cfg.OutputType = model.OutputPlain
		cfg.OutputFile = defaultOutputName(model.OutputPlain)
		return nil
	}

	switch raw {
	case model.OutputPlain, model.OutputJSON, model.OutputCSV:
		cfg.OutputType = raw
		cfg.OutputFile = defaultOutputName(raw)
		return nil
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
