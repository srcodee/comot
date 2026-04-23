# comot

`comot` is a CLI tool for fetching web targets, recursively traversing related text-based resources, applying regex patterns, and exporting structured findings.

## Overview

`comot` is designed for URL-centric inspection workflows where the input target may expose relevant matches in:

- the primary HTML response
- linked JavaScript bundles
- JSON, XML, and source map files
- additional text resources discovered during traversal

The tool supports interactive and non-interactive execution, multiple input sources, recursive discovery, repeatable regex patterns, and configurable output fields.

## Responsible Use

`comot` is intended for authorized security research, internal validation, debugging, and asset inspection on systems and resources you own or are explicitly permitted to assess.

Do not use this tool against third-party systems, services, or data without prior authorization. The operator is solely responsible for ensuring that every use of `comot` complies with applicable laws, regulations, contractual obligations, and internal policies.

The author and contributors accept no responsibility or liability for misuse, unauthorized activity, or any direct or indirect impact resulting from the use of this tool.

## Features

- Interactive mode by default when no arguments are provided
- Guided interactive continuation when an input source is provided without patterns
- Input from a single URL, file list, standard input, or saved history folders
- Repeatable regex patterns via CLI flags
- Built-in pattern selection from `.comot.data/patterns.txt`
- Recursive discovery of related resources with crawl limits and wildcard scope support
- Save fetched resources to disk for later replay
- Replay regex scans against saved history without refetching
- Exclude out-of-scope asset classes or wildcard URL patterns during discovery
- Plain terminal output with optional JSON or CSV export
- Custom output field ordering
- Response metadata capture for each finding
- Version flag for quick runtime identification
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

Saved history:

```bash
./comot --history-dir hasil -p "regex"
./comot --hd hasil -b "API key-like string"
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
  Target URL or wildcard scope.

- `-l`, `--list`
  File containing one target URL per line.

- `-I`, `--stdin`
  Read target URLs from standard input.

- `--history-dir`, `--hd`
  Scan previously saved resources from a `--save-dir` folder without refetching.

## Pattern Sources

### Custom CLI Patterns

Patterns can be repeated:

```bash
./comot -u https://example.com -p "regex1" -p "regex2"
```

### Built-In Patterns

Built-in patterns are bundled with the binary by default.

On the first run, `comot` creates a user-scoped pattern file at:

- `~/.comot.data/patterns.txt`

That generated file becomes the primary source for built-in pattern definitions on subsequent runs. You can edit it directly to add, remove, or customize local built-in patterns.

For source-based or workspace-local runs, `comot` also checks:

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

Lookup order:

- local `.comot.data/patterns.txt`
- user-scoped `~/.comot.data/patterns.txt`
- embedded built-in set

If no external pattern file exists, `comot` loads the embedded built-in set and writes the default generated file to `~/.comot.data/patterns.txt` automatically.

Use built-in patterns directly:

```bash
./comot -u https://example.com -b email -b "Swagger/OpenAPI path"
```

## Discovery Behavior

Without `-d`, `comot` scans only the primary response body.

With `-d`, `comot` discovers URLs from responses and recursively scans only resources that still match the target scope until the queue is exhausted or `--max-crawl` is reached.

Wildcard target scopes are supported in `-u`, for example:

- `*.example.com`
- `*.example.com/assets/*`
- `api.*.example.com`
- `*.*.*.example.com`

If the target is provided without a scheme, `comot` tries `https` first and then `http`.

If the target contains `*`, recursive discovery is enabled automatically even without `-d`.

Example wildcard flow:

```bash
./comot -u '*.example.com/*' -p 'regex'
```

Behavior:

- `comot` bootstraps the target and fetches the initial page
- internal URL discovery looks for URLs in HTML and text-based resources
- if a discovered URL still matches `*.example.com/*`, it is added to the crawl queue
- if that fetched resource reveals more matching URLs, they are followed too
- recursion stops when the queue is exhausted or `--max-crawl` is reached

Discovery targets include relevant references found in:

- HTML attributes such as `script src`, `link href`, and `a href`
- JavaScript, JSON, XML, source maps, and other text-based resources

Discovery remains conservative by default for non-wildcard targets:

- binary and media assets are skipped
- out-of-scope traversal is disabled unless explicitly enabled with `--allow-off-domain`

For wildcard targets, discovery is more aggressive inside the matched scope and can follow paths such as `.php`, `.html`, and extensionless routes as long as they are not skipped as binary/media assets.

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

To save exports into a specific folder with an automatic filename, use `--output-dir`.

To save fetched resource bodies such as HTML, JS, JSON, and text files into a folder tree, use `--save-dir`.

To exclude certain discovered URLs from crawling, use `--out-scope` or `--os`.

To rescan previously saved resources without making network requests, use `--history-dir` or `--hd`.

Accepted values:

- `plain`
- `json`
- `csv`
- a full filename or path such as `result.json`

Rules:

- `-o json` creates a default JSON export file
- `-o csv` creates a default CSV export file
- `-o plain` creates a default plain text export file
- `--output-dir results -o json` creates a timestamped JSON file inside `results/`
- `--output-dir loot` creates a timestamped plain text file inside `loot/`
- `-o result.json` writes directly to `result.json`
- terminal output remains plain in all cases

Examples:

```bash
./comot -u '*.example.com/*' -p 'regex' --save-dir dumps
./comot -u '*.example.com/*' -p 'regex' --save-dir scope:dumps
./comot -u https://example.com -d -p 'regex' --sd full:dumps
```

When `--save-dir` is enabled, `comot` writes fetched resources under the chosen folder and creates `index.txt` there as a crawl manifest.

Save modes:

- `--save-dir dumps` defaults to `scope` mode
- `--save-dir scope:dumps` saves only fetched resources that still match the target scope
- `--save-dir full:dumps` saves every fetched resource, including bootstrap and out-of-scope resources fetched with `--allow-off-domain`

History replay notes:

- `--history-dir hasil` scans files previously saved under `hasil/`
- if `index.txt` is available, `comot` uses it as the preferred history manifest
- if `index.txt` is missing or damaged, `comot` falls back to scanning the files directly from the saved folder tree
- if the selected history root contains multiple saved scan folders, interactive mode offers a checkbox list with `[all]` or individual folder selection

Out-scope examples:

```bash
./comot -u '*.example.com/*' -p 'regex' --out-scope images,css,video
./comot -u '*.example.com/*' -p 'regex' --os '*.svg.*.img'
```

## Output Fields

Available fields:

- `pattern`
- `pattern_name`
- `pattern_source`
- `matched_value`
- `target_url`
- `resource_url`
- `discovered_from`
- `resource_kind`
- `context`
- `line`
- `status`
- `content_type`

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

- `resource_kind`
  The detected type of the scanned resource, such as `html`, `js`, `json`, or `xml`.

- `context`
  A source snippet surrounding the matched value.

## Flags

- `-u`, `--url`
- `-l`, `--list`
- `-I`, `--stdin`
- `--history-dir`, `--hd`
- `-p`, `--pattern`
- `-b`, `--builtin`
- `-f`, `--format`
- `-o`, `--output`
- `--output-dir`
- `--save-dir`, `--sd`
- `--out-scope`, `--os`
- `-t`, `--timeout`
- `-d`, `--discover`
- `-m`, `--max-crawl`
- `-D`, `--dedup`
- `-a`, `--allow-off-domain`
- `-v`, `--version`

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

Allows recursive discovery to follow relevant resources outside the target scope.

Default behavior keeps discovery constrained to the current target scope or wildcard expression. This flag only affects discovered resources. It does not change the original input targets.

Examples:

```bash
./comot -u https://example.com -d -p "regex"
./comot -u '*.example.com/*' -a -p "regex"
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

Replay a new regex over saved crawl history:

```bash
./comot --hd hasil -p "AIza[0-9A-Za-z\\-_]+"
```

Save fetched resources while crawling:

```bash
./comot -u 'example.com/*' -d --save-dir hasil -b URL
```

Exclude asset classes during discovery:

```bash
./comot -u 'example.com/*' -d --out-scope images,css,video -p "regex"
```

Show the current CLI version:

```bash
./comot --version
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
- save-dir history replay and fallback loading without index manifests
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
- `internal/save`
- `scripts/test.sh`
- `Makefile`

## Notes

- Recursive discovery can expand quickly on large documentation or asset-heavy sites. Use `--max-crawl` to control crawl volume.
- Saved history folders can be rescanned later with `--history-dir` to try new regex patterns without touching the network again.
- The current CLI version is `v1.0.2`.
- Built-in pattern data should remain available alongside the binary or in the working directory.
- For Swagger and OpenAPI targets, the most relevant findings are often discovered inside linked spec files rather than in the initial HTML response.

The previous project README has been preserved at [README.legacy.md](\\?\UNC\wsl.localhost\kali-linux\home\xcode\tools\comot\docs\README.legacy.md).
