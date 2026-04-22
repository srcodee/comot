package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/srcodee/comot/internal/model"
)

type Writer struct {
	out        io.Writer
	format     []string
	outputType string
	timestamp  bool
	color      bool
	started    bool
	jsonCount  int
	csvWriter  *csv.Writer
}

func NewWriter(w io.Writer, format []string, outputType string, timestamp bool, color bool) (*Writer, error) {
	writer := &Writer{
		out:        w,
		format:     format,
		outputType: outputType,
		timestamp:  timestamp,
		color:      color,
	}

	switch outputType {
	case model.OutputPlain:
		return writer, nil
	case model.OutputJSON:
		if _, err := fmt.Fprint(w, "["); err != nil {
			return nil, err
		}
		writer.started = true
		return writer, nil
	case model.OutputCSV:
		csvWriter := csv.NewWriter(w)
		if err := csvWriter.Write(format); err != nil {
			return nil, err
		}
		csvWriter.Flush()
		if err := csvWriter.Error(); err != nil {
			return nil, err
		}
		writer.csvWriter = csvWriter
		writer.started = true
		return writer, nil
	default:
		return nil, fmt.Errorf("unsupported output type %q", outputType)
	}
}

func (w *Writer) WriteResults(results []model.ScanResult) error {
	switch w.outputType {
	case model.OutputPlain:
		return w.writePlain(results)
	case model.OutputJSON:
		return w.writeJSON(results)
	case model.OutputCSV:
		return w.writeCSV(results)
	default:
		return fmt.Errorf("unsupported output type %q", w.outputType)
	}
}

func (w *Writer) Close() error {
	switch w.outputType {
	case model.OutputPlain:
		return nil
	case model.OutputJSON:
		if w.started {
			_, err := fmt.Fprintln(w.out, "]")
			return err
		}
		return nil
	case model.OutputCSV:
		if w.csvWriter != nil {
			w.csvWriter.Flush()
			return w.csvWriter.Error()
		}
		return nil
	default:
		return fmt.Errorf("unsupported output type %q", w.outputType)
	}
}

func (w *Writer) writePlain(results []model.ScanResult) error {
	for _, result := range results {
		row := make([]string, 0, len(w.format))
		for _, field := range w.format {
			value := fieldValue(result, field)
			if w.color {
				value = colorize(field, value)
			}
			row = append(row, value)
		}
		line := strings.Join(row, "\t")
		if w.timestamp {
			ts := time.Now().Format("15:04:05")
			if w.color {
				ts = ansiDim + ts + ansiReset
			}
			line = ts + " " + line
		}
		if _, err := fmt.Fprintln(w.out, line); err != nil {
			return err
		}
	}
	return nil
}

const (
	ansiReset = "\033[0m"
	ansiDim   = "\033[2m"
	ansiBlue  = "\033[38;5;75m"
	ansiAmber = "\033[38;5;214m"
	ansiGreen = "\033[38;5;78m"
	ansiPink  = "\033[38;5;213m"
	ansiCyan  = "\033[38;5;81m"
	ansiGray  = "\033[38;5;245m"
)

func colorize(field, value string) string {
	if value == "" {
		return value
	}

	color := ansiGray
	switch field {
	case "target_url":
		color = ansiBlue
	case "resource_url":
		color = ansiAmber
	case "matched_value":
		color = ansiGreen
	case "pattern", "pattern_name", "pattern_source":
		color = ansiPink
	case "status", "line":
		color = ansiCyan
	case "content_type", "discovered_from":
		color = ansiGray
	}

	return color + value + ansiReset
}

func (w *Writer) writeJSON(results []model.ScanResult) error {
	enc := json.NewEncoder(w.out)
	enc.SetIndent("", "  ")

	for _, result := range results {
		if w.jsonCount > 0 {
			if _, err := fmt.Fprint(w.out, ","); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w.out); err != nil {
			return err
		}
		row := make(map[string]string, len(w.format))
		for _, field := range w.format {
			row[field] = fieldValue(result, field)
		}
		if err := enc.Encode(row); err != nil {
			return err
		}
		w.jsonCount++
	}
	return nil
}

func (w *Writer) writeCSV(results []model.ScanResult) error {
	for _, result := range results {
		row := make([]string, 0, len(w.format))
		for _, field := range w.format {
			row = append(row, fieldValue(result, field))
		}
		if err := w.csvWriter.Write(row); err != nil {
			return err
		}
	}
	w.csvWriter.Flush()
	return w.csvWriter.Error()
}

func fieldValue(result model.ScanResult, field string) string {
	switch field {
	case "pattern":
		return result.Pattern
	case "pattern_name":
		return result.PatternName
	case "pattern_source":
		return result.PatternSource
	case "matched_value":
		return result.MatchedValue
	case "context":
		return result.Context
	case "target_url":
		return result.TargetURL
	case "resource_url":
		return result.ResourceURL
	case "resource_kind":
		return result.ResourceKind
	case "discovered_from":
		return result.DiscoveredFrom
	case "line":
		return strconv.Itoa(result.Line)
	case "status":
		return strconv.Itoa(result.Status)
	case "content_type":
		return result.ContentType
	default:
		return ""
	}
}
