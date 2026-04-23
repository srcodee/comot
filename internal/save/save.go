package save

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/srcodee/comot/internal/model"
)

type Saver struct {
	baseDir string
	count   int
}

type Entry struct {
	Order        int
	URL          string
	RelativePath string
	ContentType  string
	StatusCode   int
}

const maxPathPartLen = 96

func New(baseDir string) (*Saver, error) {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		return nil, nil
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("create save directory: %w", err)
	}
	return &Saver{baseDir: baseDir}, nil
}

func (s *Saver) BaseDir() string {
	if s == nil {
		return ""
	}
	return s.baseDir
}

func (s *Saver) Save(resource model.Resource) error {
	if s == nil {
		return nil
	}

	parsed, err := url.Parse(resource.FinalURL)
	if err != nil {
		return fmt.Errorf("parse resource URL: %w", err)
	}

	relativePath := buildRelativePath(parsed, resource.ContentType)
	fullPath := filepath.Join(s.baseDir, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return fmt.Errorf("create resource directory: %w", err)
	}
	if err := os.WriteFile(fullPath, resource.Body, 0o644); err != nil {
		return fmt.Errorf("write saved resource: %w", err)
	}

	s.count++
	entry := Entry{
		Order:        s.count,
		URL:          resource.FinalURL,
		RelativePath: relativePath,
		ContentType:  resource.ContentType,
		StatusCode:   resource.StatusCode,
	}
	if err := s.appendIndex(entry); err != nil {
		return err
	}
	return nil
}

func (s *Saver) Close() error {
	if s == nil {
		return nil
	}
	return nil
}

func (s *Saver) appendIndex(entry Entry) error {
	indexPath := filepath.Join(s.baseDir, "index.txt")
	file, err := os.OpenFile(indexPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open save index: %w", err)
	}
	defer file.Close()

	line := fmt.Sprintf("%06d\t%d\t%s\t%s\t%s\n",
		entry.Order,
		entry.StatusCode,
		entry.ContentType,
		entry.URL,
		entry.RelativePath,
	)
	if _, err := file.WriteString(line); err != nil {
		return fmt.Errorf("append save index: %w", err)
	}
	return nil
}

func buildRelativePath(parsed *url.URL, contentType string) string {
	host := sanitizePart(parsed.Host)
	cleanPath := path.Clean(parsed.EscapedPath())
	if cleanPath == "." || cleanPath == "/" {
		return path.Join(host, "index"+defaultExtension(cleanPath, contentType))
	}

	dir, file := path.Split(cleanPath)
	file = sanitizePart(file)
	if file == "" {
		return path.Join(host, sanitizeDir(dir), "index"+defaultExtension(cleanPath, contentType))
	}

	if path.Ext(file) == "" {
		file += defaultExtension(cleanPath, contentType)
	}
	if parsed.RawQuery != "" {
		file = appendHashSuffix(file, parsed.RawQuery)
	}
	return path.Join(host, sanitizeDir(dir), file)
}

func sanitizeDir(raw string) string {
	raw = strings.Trim(raw, "/")
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, "/")
	for i, part := range parts {
		parts[i] = sanitizePart(part)
	}
	return path.Join(parts...)
}

func sanitizePart(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "_"
	}
	replacer := strings.NewReplacer(
		":", "_",
		"?", "_",
		"&", "_",
		"=", "_",
		"*", "_",
		"\\", "_",
	)
	raw = replacer.Replace(raw)
	raw = strings.Trim(raw, " .")
	if raw == "" {
		return "_"
	}
	return shortenPart(raw)
}

func appendHashSuffix(fileName string, query string) string {
	ext := path.Ext(fileName)
	base := strings.TrimSuffix(fileName, ext)
	sum := sha1.Sum([]byte(query))
	base = shortenPart(base)
	return base + "__" + hex.EncodeToString(sum[:4]) + ext
}

func defaultExtension(rawPath, contentType string) string {
	if ext := path.Ext(rawPath); ext != "" {
		return ext
	}
	ct := strings.ToLower(contentType)
	switch {
	case strings.Contains(ct, "html"):
		return ".html"
	case strings.Contains(ct, "javascript"), strings.Contains(ct, "ecmascript"):
		return ".js"
	case strings.Contains(ct, "json"):
		return ".json"
	case strings.Contains(ct, "xml"):
		return ".xml"
	case strings.Contains(ct, "css"):
		return ".css"
	case strings.Contains(ct, "plain"), strings.Contains(ct, "text"):
		return ".txt"
	default:
		return ".bin"
	}
}

func shortenPart(raw string) string {
	if len(raw) <= maxPathPartLen {
		return raw
	}
	sum := sha1.Sum([]byte(raw))
	suffix := "__" + hex.EncodeToString(sum[:4])
	keep := maxPathPartLen - len(suffix)
	if keep < 8 {
		keep = 8
	}
	return raw[:keep] + suffix
}
