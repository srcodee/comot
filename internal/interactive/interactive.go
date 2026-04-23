package interactive

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"

	"github.com/srcodee/comot/internal/model"
	"github.com/srcodee/comot/internal/save"
)

const scriptedEnv = "COMOT_SCRIPTED_PROMPTS"

type Prompter interface {
	Input(message, defaultValue string, required bool) (string, error)
	MultiSelect(message string, options []string) ([]string, error)
	Select(message string, options []string, defaultValue string) (string, error)
}

type ScriptedAnswers struct {
	TargetURL    string   `json:"target_url"`
	HistoryDirs  []string `json:"history_dirs"`
	BuiltinNames []string `json:"builtin_names"`
	CustomRegex  string   `json:"custom_regex"`
	Format       string   `json:"format"`
	OutputType   string   `json:"output_type"`
}

type surveyPrompter struct{}

type scriptedPrompter struct {
	answers ScriptedAnswers
}

func Complete(cfg model.Config, builtins []model.PatternDefinition) (model.Config, error) {
	prompter, err := newPrompterFromEnv()
	if err != nil {
		return cfg, err
	}
	return CompleteWithPrompter(cfg, builtins, prompter)
}

func CompleteWithPrompter(cfg model.Config, builtins []model.PatternDefinition, prompter Prompter) (model.Config, error) {
	if cfg.HistoryDir != "" {
		dirs, err := save.DiscoverHistoryDirs(cfg.HistoryDir)
		if err != nil {
			return cfg, err
		}
		if len(dirs) == 1 {
			cfg.HistoryDirs = dirs
		} else {
			options := make([]string, 0, len(dirs)+1)
			options = append(options, "[all]")
			for _, dir := range dirs {
				options = append(options, filepath.Base(dir)+" :: "+dir)
			}
			selected, err := prompter.MultiSelect("Select history folders:", options)
			if err != nil {
				return cfg, err
			}
			if len(selected) == 0 {
				return cfg, fmt.Errorf("at least one history folder is required")
			}
			for _, item := range selected {
				if item == "[all]" {
					cfg.HistoryDirs = dirs
					break
				}
				cfg.HistoryDirs = append(cfg.HistoryDirs, strings.SplitN(item, " :: ", 2)[1])
			}
		}
	}

	if cfg.URL == "" && cfg.ListPath == "" && !cfg.UseStdin && cfg.HistoryDir == "" {
		value, err := prompter.Input("Target URL:", "", true)
		if err != nil {
			return cfg, err
		}
		cfg.URL = value
	}

	options := make([]string, 0, len(builtins))
	byOption := make(map[string]model.PatternDefinition, len(builtins))
	for _, def := range builtins {
		label := fmt.Sprintf("%s :: %s", def.Name, previewRegex(def.Regex, 58))
		options = append(options, label)
		byOption[label] = def
	}

	if len(cfg.Patterns) == 0 {
		selected, err := prompter.MultiSelect("Select built-in patterns:", options)
		if err != nil {
			return cfg, err
		}

		for _, item := range selected {
			if def, ok := byOption[item]; ok {
				cfg.Patterns = append(cfg.Patterns, def.Regex)
				cfg.PatternDefs = append(cfg.PatternDefs, def)
				continue
			}

			name := strings.SplitN(item, " :: ", 2)[0]
			for _, def := range builtins {
				if def.Name == name {
					cfg.Patterns = append(cfg.Patterns, def.Regex)
					cfg.PatternDefs = append(cfg.PatternDefs, def)
				}
			}
		}
	}

	custom, err := prompter.Input("Add custom regex (optional, comma separated for multiple):", "", false)
	if err != nil {
		return cfg, err
	}
	if custom != "" {
		for _, item := range strings.Split(custom, ",") {
			item = strings.TrimSpace(item)
			if item != "" {
				cfg.Patterns = append(cfg.Patterns, item)
				cfg.PatternDefs = append(cfg.PatternDefs, model.PatternDefinition{
					Name:   "custom",
					Regex:  item,
					Source: "custom:interactive",
				})
			}
		}
	}

	if len(cfg.Patterns) == 0 {
		return cfg, fmt.Errorf("at least one pattern is required")
	}

	if len(cfg.Format) == 0 {
		format, err := prompter.Input("Output fields:", "pattern,pattern_name,resource_url,matched_value", true)
		if err != nil {
			return cfg, err
		}
		cfg.Format = splitCSV(format)
	}

	if cfg.OutputType == "" {
		value, err := prompter.Select(
			"Output type:",
			[]string{model.OutputCSV, model.OutputJSON, model.OutputPlain},
			model.OutputPlain,
		)
		if err != nil {
			return cfg, err
		}
		cfg.OutputType = value
	}

	return cfg, nil
}

func newPrompterFromEnv() (Prompter, error) {
	raw := strings.TrimSpace(os.Getenv(scriptedEnv))
	if raw == "" {
		return surveyPrompter{}, nil
	}

	var answers ScriptedAnswers
	if err := json.Unmarshal([]byte(raw), &answers); err != nil {
		return nil, fmt.Errorf("parse %s: %w", scriptedEnv, err)
	}
	return scriptedPrompter{answers: answers}, nil
}

func (surveyPrompter) Input(message, defaultValue string, required bool) (string, error) {
	prompt := &survey.Input{
		Message: message,
		Default: defaultValue,
	}
	var out string
	opts := []survey.AskOpt{}
	if required {
		opts = append(opts, survey.WithValidator(survey.Required))
	}
	if err := survey.AskOne(prompt, &out, opts...); err != nil {
		return "", err
	}
	return out, nil
}

func (surveyPrompter) MultiSelect(message string, options []string) ([]string, error) {
	var out []string
	if err := survey.AskOne(&survey.MultiSelect{
		Message: message,
		Options: options,
	}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (surveyPrompter) Select(message string, options []string, defaultValue string) (string, error) {
	var out string
	if err := survey.AskOne(&survey.Select{
		Message: message,
		Options: options,
		Default: defaultValue,
	}, &out); err != nil {
		return "", err
	}
	return out, nil
}

func (p scriptedPrompter) Input(message, defaultValue string, required bool) (string, error) {
	value := defaultValue
	switch message {
	case "Target URL:":
		if p.answers.TargetURL != "" {
			value = p.answers.TargetURL
		}
	case "Add custom regex (optional, comma separated for multiple):":
		value = p.answers.CustomRegex
	case "Output fields:":
		if p.answers.Format != "" {
			value = p.answers.Format
		}
	}
	if required && strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s is required", message)
	}
	return value, nil
}

func (p scriptedPrompter) MultiSelect(message string, options []string) ([]string, error) {
	switch message {
	case "Select built-in patterns:":
		if len(p.answers.BuiltinNames) == 0 {
			return nil, nil
		}
		selected := make([]string, 0, len(p.answers.BuiltinNames))
		for _, name := range p.answers.BuiltinNames {
			matched := false
			for _, option := range options {
				if strings.HasPrefix(option, name+" ::") || option == name {
					selected = append(selected, option)
					matched = true
					break
				}
			}
			if !matched {
				return nil, fmt.Errorf("scripted built-in pattern %q not found", name)
			}
		}
		return selected, nil
	case "Select history folders:":
		if len(p.answers.HistoryDirs) == 0 {
			return []string{"[all]"}, nil
		}
		selected := make([]string, 0, len(p.answers.HistoryDirs))
		for _, dir := range p.answers.HistoryDirs {
			matched := false
			for _, option := range options {
				if option == "[all]" && dir == "[all]" {
					selected = append(selected, option)
					matched = true
					break
				}
				if strings.HasSuffix(option, " :: "+dir) {
					selected = append(selected, option)
					matched = true
					break
				}
			}
			if !matched {
				return nil, fmt.Errorf("scripted history folder %q not found", dir)
			}
		}
		return selected, nil
	default:
		return nil, fmt.Errorf("unsupported multi-select prompt %q", message)
	}
}

func (p scriptedPrompter) Select(message string, options []string, defaultValue string) (string, error) {
	if message != "Output type:" {
		return "", fmt.Errorf("unsupported select prompt %q", message)
	}
	value := defaultValue
	if p.answers.OutputType != "" {
		value = p.answers.OutputType
	}
	for _, option := range options {
		if option == value {
			return value, nil
		}
	}
	return "", fmt.Errorf("invalid scripted output type %q", value)
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func previewRegex(raw string, max int) string {
	raw = strings.TrimSpace(raw)
	if len(raw) <= max {
		return raw
	}
	if max <= 3 {
		return raw[:max]
	}
	return raw[:max-3] + "..."
}
