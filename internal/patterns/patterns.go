package patterns

import (
	"bufio"
	"embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/srcodee/comot/internal/model"
)

const dataFile = "patterns.txt"

//go:embed patterns_embedded.txt
var embeddedFS embed.FS

func LoadBuiltins() ([]model.PatternDefinition, error) {
	paths := candidatePaths()

	var lastErr error
	for _, path := range paths {
		defs, err := loadFile(path)
		if err == nil {
			for i := range defs {
				defs[i].Source = fmt.Sprintf("builtin:%s:%s", path, defs[i].Name)
			}
			return defs, nil
		}
		lastErr = err
	}

	if lastErr == nil {
		lastErr = errors.New("pattern data file not found")
	}

	defs, err := loadEmbedded()
	if err == nil {
		if generatedPath, genErr := ensureGeneratedFile(); genErr == nil {
			for i := range defs {
				defs[i].Source = fmt.Sprintf("builtin:%s:%s", generatedPath, defs[i].Name)
			}
			return defs, nil
		}
		for i := range defs {
			defs[i].Source = fmt.Sprintf("builtin:embedded:%s", defs[i].Name)
		}
		return defs, nil
	}

	return nil, fmt.Errorf("load built-in patterns: %w", errors.Join(lastErr, err))
}

func FindByNames(defs []model.PatternDefinition, names []string) []model.PatternDefinition {
	index := make(map[string]model.PatternDefinition, len(defs))
	for _, def := range defs {
		index[def.Name] = def
	}

	results := make([]model.PatternDefinition, 0, len(names))
	for _, name := range names {
		if def, ok := index[name]; ok {
			results = append(results, def)
		}
	}
	return results
}

func loadFile(path string) ([]model.PatternDefinition, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return loadReader(file, path)
}

func loadEmbedded() ([]model.PatternDefinition, error) {
	file, err := embeddedFS.Open("patterns_embedded.txt")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return loadReader(file, "embedded:patterns_embedded.txt")
}

func candidatePaths() []string {
	paths := []string{
		filepath.Join(".comot.data", dataFile),
		filepath.Join("..", ".comot.data", dataFile),
	}

	if home, err := os.UserHomeDir(); err == nil && home != "" {
		paths = append(paths, filepath.Join(home, ".comot.data", dataFile))
	}

	if exe, err := os.Executable(); err == nil {
		base := filepath.Dir(exe)
		paths = append(paths,
			filepath.Join(base, ".comot.data", dataFile),
			filepath.Join(filepath.Dir(base), ".comot.data", dataFile),
		)
	}

	return uniquePaths(paths)
}

func ensureGeneratedFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		if err == nil {
			err = errors.New("home directory not available")
		}
		return "", err
	}

	path := filepath.Join(home, ".comot.data", dataFile)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	data, err := embeddedFS.ReadFile("patterns_embedded.txt")
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func uniquePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		result = append(result, path)
	}
	return result
}

func loadReader(reader io.Reader, source string) ([]model.PatternDefinition, error) {
	var defs []model.PatternDefinition
	scanner := bufio.NewScanner(reader)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "||", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid pattern line %d in %s", lineNo, source)
		}
		name := strings.TrimSpace(parts[0])
		regex := strings.TrimSpace(parts[1])
		if name == "" || regex == "" {
			return nil, fmt.Errorf("invalid pattern line %d in %s", lineNo, source)
		}
		defs = append(defs, model.PatternDefinition{
			Name:  name,
			Regex: regex,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return defs, nil
}
