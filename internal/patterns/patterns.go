package patterns

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"comot/internal/model"
)

const dataFile = "patterns.txt"

func LoadBuiltins() ([]model.PatternDefinition, error) {
	paths := []string{
		filepath.Join(".comot.data", dataFile),
		filepath.Join("..", ".comot.data", dataFile),
	}

	if exe, err := os.Executable(); err == nil {
		base := filepath.Dir(exe)
		paths = append(paths,
			filepath.Join(base, ".comot.data", dataFile),
			filepath.Join(filepath.Dir(base), ".comot.data", dataFile),
		)
	}

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
	return nil, fmt.Errorf("load built-in patterns: %w", lastErr)
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

	var defs []model.PatternDefinition
	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "||", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid pattern line %d in %s", lineNo, path)
		}
		name := strings.TrimSpace(parts[0])
		regex := strings.TrimSpace(parts[1])
		if name == "" || regex == "" {
			return nil, fmt.Errorf("invalid pattern line %d in %s", lineNo, path)
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
