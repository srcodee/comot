# comot

`comot` is a CLI tool for fetching web targets, recursively traversing related text-based resources, applying regex patterns, and exporting structured findings.

## Overview

`comot` is designed for URL-centric inspection workflows where the input target may expose relevant matches in:

- the primary HTML response
- linked JavaScript bundles
- JSON, XML, and source map files
- additional text resources discovered during traversal

The tool supports interactive and non-interactive execution, multiple input sources, recursive discovery, repeatable regex patterns, and configurable output fields.

## Features

- Interactive mode by default when no arguments are provided
- Guided interactive continuation when an input source is provided without patterns
- Input from a single URL, file list, or standard input
- Repeatable regex patterns via CLI flags
- Built-in pattern selection from `.comot.data/patterns.txt`
- Recursive discovery of related resources with crawl limits
- Plain terminal output with optional JSON or CSV export
- Custom output field ordering
- Response metadata capture for each finding
- Integrated test runner for unit and integration coverage

## Installation

### Requirements

- Go 1.21 or newer

### Install

```bash
go install github.com/srcodee/comot/cmd/comot@latest
```

### Build From Source

```bash
git clone git@github.com:srcodee/comot.git
cd comot
go mod tidy
go build -o comot ./cmd/comot
```

## Usage

### Interactive

Start interactive mode:

```bash
./comot
```

Start with an input source and complete the remaining options interactively:

```bash
./comot -u https://example.com
./comot -l targets.txt
cat targets.txt | ./comot --stdin
```

### Non-Interactive

Single URL:

```bash
./comot -u https://example.com -p "https://[^\"' ]+"
```

File list:

```bash
./comot -l targets.txt -p "regex"
```

Standard input:

```bash
cat targets.txt | ./comot --stdin -p "regex"
```

Recursive discovery:

```bash
./comot -u https://example.com -d -p "/api/[A-Za-z0-9_./-]+"
```

Export to JSON:

```bash
./comot -u https://example.com -d -p "regex" -o result.json
```

Export to CSV:

```bash
./comot -u https://example.com -d -p "regex" -o result.csv
```

Automatic export file generation by type:

```bash
./comot -u https://example.com -p "regex" -o json
./comot -u https://example.com -p "regex" -o csv
./comot -u https://example.com -p "regex" -o plain
```

Terminal output remains plain even when export is enabled.

## Input Sources

- `-u`, `--url`
  Single target URL.

- `-l`, `--list`
  File containing one target URL per line.

- `-I`, `--stdin`
  Read target URLs from standard input.

## Pattern Sources

### Custom CLI Patterns

Patterns can be repeated:

```bash
./comot -u https://example.com -p "regex1" -p "regex2"
```

### Built-In Patterns

Built-in patterns are defined in:

- [patterns.txt](\\?\UNC\wsl.localhost\kali-linux\home\xcode\tools\comot\.comot.data\patterns.txt)

Format:

```text
Pattern Name || regex
```

Example:

```text
JSON endpoint || https?://[^\s\"']+\.json(?:\?[^\s\"']*)?
Swagger/OpenAPI path || "(?:/[^"\s{}]+)+":\s*\{
```

Use built-in patterns directly:

```bash
./comot -u https://example.com -b email -b "Swagger/OpenAPI path"
```

## Discovery Behavior

Without `-d`, `comot` scans only the primary response body.

With `-d`, `comot` recursively discovers and scans related resources until the queue is exhausted or `--max-crawl` is reached.

Discovery targets include relevant references found in:

- HTML attributes such as `script src`, `link href`, and `a href`
- JavaScript, JSON, XML, source maps, and other text-based resources

Discovery remains conservative by default:

- binary and media assets are skipped
- off-domain traversal is disabled unless explicitly enabled

### Crawl Limit

Recursive discovery is limited by:

```text
-m, --max-crawl
```

Default:

```text
10000
```

Example:

```bash
./comot -u https://developer.mozilla.org -d --max-crawl 2000
```

## Output Model

### Terminal Output

- Terminal output is always plain
- Plain terminal output includes a timestamp prefix
- Color is applied only to terminal output

### Export Output

Export is controlled by `-o`, `--output`.

Accepted values:

- `plain`
- `json`
- `csv`
- a full filename or path such as `result.json`

Rules:

- `-o json` creates a default JSON export file
- `-o csv` creates a default CSV export file
- `-o plain` creates a default plain text export file
- `-o result.json` writes directly to `result.json`
- terminal output remains plain in all cases

## Output Fields

Available fields:

- `pattern`
- `pattern_name`
- `pattern_source`
- `matched_value`
- `target_url`
- `resource_url`
- `discovered_from`
- `url`
- `line`
- `status`
- `content_type`
- `match`

Default field order:

```text
pattern,pattern_name,resource_url,matched_value
```

Custom field order:

```bash
./comot -u https://example.com -p "regex" -f "pattern,resource_url,matched_value,status"
```

### Field Notes

- `pattern`
  The regex used for the match.

- `pattern_name`
  The built-in pattern name or `custom`.

- `resource_url`
  The resource that was actually scanned when the match was found.

- `target_url`
  The original input target.

- `discovered_from`
  The parent resource from which the current resource was discovered.

- `matched_value`
  The actual matched string.

## Flags

- `-u`, `--url`
- `-l`, `--list`
- `-I`, `--stdin`
- `-p`, `--pattern`
- `-b`, `--builtin`
- `-f`, `--format`
- `-o`, `--output`
- `-t`, `--timeout`
- `-d`, `--discover`
- `-m`, `--max-crawl`
- `-D`, `--dedup`
- `-a`, `--allow-off-domain`

See the current runtime help for the authoritative flag list:

```bash
./comot --help
```

### Flag Details

#### `-D`, `--dedup`

Deduplicates identical findings within a run.

When enabled, repeated findings with the same pattern, matched value, resource URL, and line number are emitted once.

Examples:

```bash
./comot -u https://example.com -p "token" -D
./comot -u https://example.com -p "token" --dedup=false
```

#### `-a`, `--allow-off-domain`

Allows recursive discovery to follow relevant resources outside the original host.

Default behavior keeps discovery constrained to the original hostname. This flag only affects discovered resources. It does not change the original input targets.

Examples:

```bash
./comot -u https://example.com -d -p "regex"
./comot -u https://example.com -d -a -p "regex"
```

Use this flag only when cross-domain assets, hosted API specifications, or external script bundles are required for the scan.

#### `-t`, `--timeout`

Sets the per-request HTTP timeout used by the fetch client.

Accepted values use Go duration syntax, for example:

- `500ms`
- `5s`
- `30s`

Examples:

```bash
./comot -u https://example.com -p "regex" -t 5s
./comot -u https://example.com -d -p "regex" --timeout 1500ms
```

## Examples

Detect OpenAPI paths from a Swagger UI target:

```bash
./comot -u https://sdi.babelprov.go.id/swagger -d -b "Swagger/OpenAPI path"
```

Scan a list and export JSON:

```bash
./comot -l targets.txt -d -p "https?://[^\"' ]+" -o results.json
```

Read from stdin and emit selected fields:

```bash
cat targets.txt | ./comot --stdin -p "regex" -f "pattern,resource_url,matched_value"
```

## Testing

Run the full test suite:

```bash
make test
```

Or run the test script directly:

```bash
./scripts/test.sh
```

The unified test runner covers:

- Go unit tests
- CLI build verification
- URL, file list, and stdin execution paths
- recursive discovery
- export behavior
- built-in pattern execution
- crawl limiting
- deduplication
- off-domain discovery control
- timeout enforcement
- scripted interactive completion

## Repository Layout

- `cmd/comot/main.go`
- `internal/cli`
- `internal/interactive`
- `internal/fetch`
- `internal/discover`
- `internal/patterns`
- `internal/scan`
- `internal/output`
- `internal/model`
- `.comot.data/patterns.txt`
- `scripts/test.sh`
- `Makefile`

## Notes

- Recursive discovery can expand quickly on large documentation or asset-heavy sites. Use `--max-crawl` to control crawl volume.
- Built-in pattern data should remain available alongside the binary or in the working directory.
- For Swagger and OpenAPI targets, the most relevant findings are often discovered inside linked spec files rather than in the initial HTML response.

The previous project README has been preserved at [README.legacy.md](\\?\UNC\wsl.localhost\kali-linux\home\xcode\tools\comot\docs\README.legacy.md).
