package model

import (
	"net/http"
	"time"
)

const (
	OutputPlain = "plain"
	OutputJSON  = "json"
	OutputCSV   = "csv"
)

var AllowedFields = map[string]struct{}{
	"pattern":         {},
	"pattern_name":    {},
	"pattern_source":  {},
	"matched_value":   {},
	"context":         {},
	"target_url":      {},
	"resource_url":    {},
	"resource_kind":   {},
	"discovered_from": {},
	"line":            {},
	"status":          {},
	"content_type":    {},
}

type Config struct {
	URL            string
	ListPath       string
	UseStdin       bool
	HistoryDir     string
	HistoryDirs    []string
	Patterns       []string
	PatternDefs    []PatternDefinition
	Format         []string
	OutputType     string
	OutputFile     string
	OutputDir      string
	SaveDir        string
	SaveMode       string
	OutScope       []string
	Timeout        time.Duration
	Discover       bool
	DedupResults   bool
	MaxCrawl       int
	MaxRedirects   int
	Interactive    bool
	AllowOffDomain bool
}

type PatternDefinition struct {
	Name        string `json:"name"`
	Regex       string `json:"regex"`
	Description string `json:"description"`
	Source      string `json:"source,omitempty"`
}

type Resource struct {
	URL         string
	FinalURL    string
	TargetURL   string
	ParentURL   string
	StatusCode  int
	ContentType string
	Body        []byte
	Header      http.Header
}

type DiscoveredResource struct {
	URL            string
	DiscoveredFrom string
}

type ScanResult struct {
	Pattern        string `json:"pattern"`
	PatternName    string `json:"pattern_name,omitempty"`
	PatternSource  string `json:"pattern_source"`
	MatchedValue   string `json:"matched_value"`
	Context        string `json:"context,omitempty"`
	TargetURL      string `json:"target_url"`
	ResourceURL    string `json:"resource_url"`
	ResourceKind   string `json:"resource_kind,omitempty"`
	DiscoveredFrom string `json:"discovered_from,omitempty"`
	Line           int    `json:"line"`
	Status         int    `json:"status"`
	ContentType    string `json:"content_type"`
}
