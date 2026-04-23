package save

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/srcodee/comot/internal/model"
)

func LoadResources(baseDir string) ([]model.Resource, error) {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		return nil, fmt.Errorf("empty history directory")
	}

	indexPath := filepath.Join(baseDir, "index.txt")
	file, err := os.Open(indexPath)
	if err != nil {
		return loadResourcesFromFiles(baseDir)
	}
	defer file.Close()

	var resources []model.Resource
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 5)
		if len(parts) != 5 {
			return loadResourcesFromFiles(baseDir)
		}

		statusCode, err := strconv.Atoi(parts[1])
		if err != nil {
			return loadResourcesFromFiles(baseDir)
		}
		body, err := os.ReadFile(filepath.Join(baseDir, filepath.FromSlash(parts[4])))
		if err != nil {
			return loadResourcesFromFiles(baseDir)
		}

		resources = append(resources, model.Resource{
			URL:         parts[3],
			FinalURL:    parts[3],
			TargetURL:   parts[3],
			StatusCode:  statusCode,
			ContentType: parts[2],
			Body:        body,
		})
	}
	if err := scanner.Err(); err != nil {
		return loadResourcesFromFiles(baseDir)
	}
	return resources, nil
}

func DiscoverHistoryDirs(baseDir string) ([]string, error) {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		return nil, fmt.Errorf("empty history directory")
	}

	indexPath := filepath.Join(baseDir, "index.txt")
	if _, err := os.Stat(indexPath); err == nil {
		return []string{baseDir}, nil
	}

	found := make([]string, 0)
	seen := make(map[string]struct{})
	if hasFiles, err := dirHasImmediateHistoryFiles(baseDir); err == nil && hasFiles {
		seen[baseDir] = struct{}{}
		found = append(found, baseDir)
	}

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, fmt.Errorf("discover history directories: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(baseDir, entry.Name())
		if _, ok := seen[dir]; ok {
			continue
		}
		hasFiles, err := dirHasHistoryFiles(dir)
		if err != nil || !hasFiles {
			continue
		}
		seen[dir] = struct{}{}
		found = append(found, dir)
	}

	err = filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() != "index.txt" {
			return nil
		}
		dir := filepath.Dir(path)
		if _, ok := seen[dir]; ok {
			return nil
		}
		seen[dir] = struct{}{}
		found = append(found, dir)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover history directories: %w", err)
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("no saved history found under %s", baseDir)
	}
	sort.Strings(found)
	return found, nil
}

func loadResourcesFromFiles(baseDir string) ([]model.Resource, error) {
	resources := make([]model.Resource, 0)
	err := filepath.WalkDir(baseDir, func(current string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() == "index.txt" {
			return nil
		}

		body, err := os.ReadFile(current)
		if err != nil {
			return err
		}

		relative, err := filepath.Rel(baseDir, current)
		if err != nil {
			return err
		}
		urlValue := "file://" + filepath.ToSlash(relative)
		resources = append(resources, model.Resource{
			URL:         urlValue,
			FinalURL:    urlValue,
			TargetURL:   urlValue,
			StatusCode:  200,
			ContentType: inferContentTypeFromPath(current),
			Body:        body,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("load history files: %w", err)
	}
	if len(resources) == 0 {
		return nil, fmt.Errorf("no saved history found under %s", baseDir)
	}

	sort.Slice(resources, func(i, j int) bool {
		return resources[i].FinalURL < resources[j].FinalURL
	})
	return resources, nil
}

func inferContentTypeFromPath(current string) string {
	switch strings.ToLower(filepath.Ext(current)) {
	case ".html", ".htm", ".":
		return "text/html"
	case ".js", ".mjs":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".css":
		return "text/css"
	case ".txt":
		return "text/plain"
	case ".php":
		return "text/html"
	default:
		return "application/octet-stream"
	}
}

func dirHasHistoryFiles(baseDir string) (bool, error) {
	hasFiles := false
	err := filepath.WalkDir(baseDir, func(current string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() == "index.txt" {
			hasFiles = true
			return fs.SkipAll
		}
		hasFiles = true
		return fs.SkipAll
	})
	if err != nil && err != fs.SkipAll {
		return false, err
	}
	return hasFiles, nil
}

func dirHasImmediateHistoryFiles(baseDir string) (bool, error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		return true, nil
	}
	return false, nil
}
