# comot

`comot` adalah CLI tool Go untuk fetch target URL, menemukan resource terkait yang relevan, menjalankan regex match, lalu menampilkan atau mengekspor hasil dalam format custom.

## Fitur MVP

- Default interactive mode saat `comot` dijalankan tanpa argumen
- Interactive continuation saat `comot -u https://target.com`
- Non-interactive mode dengan short dan full flags
- Input dari `--url`, `--list`, dan `--stdin`
- Built-in pattern dari file `.comot.data/patterns.txt`
- Multi-select built-in pattern di interactive mode
- Terminal output selalu `plain`
- Ekspor tambahan mendukung `plain`, `json`, dan `csv`
- Field output custom: `pattern`, `pattern_name`, `pattern_source`, `matched_value`, `target_url`, `resource_url`, `discovered_from`, `url`, `line`, `status`, `content_type`
- Discovery resource HTML untuk `script src`, `link href`, dan `a href` yang relevan
- Progress bar dan elapsed time live di `stderr`
- Opsi discovery resource diatur lewat flag, tidak ditanyakan di interactive flow
- Default output ke terminal adalah `plain` agar hasil live lebih rapi
- `-o` dipakai untuk ekspor tambahan ke file, sementara hasil di terminal tetap tampil plain

## Struktur Project

- `cmd/comot/main.go`
- `internal/cli/root.go`
- `internal/interactive/interactive.go`
- `internal/fetch/fetch.go`
- `internal/discover/discover.go`
- `internal/patterns/patterns.go`
- `internal/scan/scan.go`
- `internal/output/output.go`
- `internal/model/model.go`
- `.comot.data/patterns.txt`

## Build

```bash
go mod tidy
go build -o comot ./cmd/comot
```

## Test

Jalankan semua test sekali:

```bash
make test
```

Atau langsung:

```bash
./scripts/test.sh
```

## Run

Interactive default:

```bash
./comot
```

Interactive continuation dari URL:

```bash
./comot -u https://target.com
```

Non-interactive dengan custom regex:

```bash
./comot -u https://target.com -p "https://[^\"' ]+" -f "target_url,resource_url,matched_value,pattern" -o json
```

```bash
./comot --url https://target.com --pattern "eyJ[A-Za-z0-9._-]+" --format "matched_value" --output csv
```

Input dari stdin:

```bash
cat urls.txt | ./comot --stdin -p "regex" -f "target_url,resource_url,matched_value"
```

Input dari list file:

```bash
./comot -l urls.txt -p "regex" -o plain -f "target_url,resource_url,matched_value,line"
```

Pilih built-in pattern langsung dari CLI:

```bash
./comot -u https://target.com -b JWT -b email -o csv
```

Scan resource terkait secara recursive sampai habis:

```bash
./comot -u https://target.com -p "/api/[A-Za-z0-9_./-]+" -d
```

Progress bar akan tampil seperti:

```text
[===========>............] 3/7 elapsed 1.8s current scan https://target.com/app.js
```

Simpan ke file:

```bash
./comot -u https://target.com -p "regex" -o result.json
./comot -u https://target.com -p "regex" -o hasil.csv
./comot -u https://target.com -p "regex" -o hasil.txt
./comot -u https://target.com -p "regex" -o csv
```

## Flag Utama

- `-u`, `--url`: target URL tunggal
- `-l`, `--list`: file daftar URL
- `-I`, `--stdin`: baca URL dari stdin
- `-p`, `--pattern`: regex pattern, repeatable
- `-b`, `--builtin`: built-in pattern name, repeatable
- `-f`, `--format`: urutan field output, default `pattern,pattern_name,resource_url,matched_value`
- `-o`, `--output`: ekspor ke file dengan `plain`, `json`, `csv`, atau nama file/path seperti `hasil.csv`
- `-t`, `--timeout`: timeout HTTP
- `-d`, `--discover`: telusuri dan scan semua resource terkait sampai habis
- `-m`, `--max-crawl`: batas maksimum jumlah resource yang dicrawl saat discovery, default `10000`
- `-D`, `--dedup`: deduplicate result identik
- `-a`, `--allow-off-domain`: izinkan discovery keluar host awal

## Catatan MVP

- Discovery resource saat ini konservatif dan fokus pada HTML references yang umum serta file yang relevan untuk text matching.
- Tanpa `-d`, hanya response target utama yang discan. Dengan `-d`, discovery berjalan recursive sampai tidak ada resource relevan baru.
- Discovery dibatasi oleh `--max-crawl` agar crawl besar tidak berjalan tak terbatas. Default `10000`.
- `resource_url` menunjukkan file yang benar-benar discan dan tempat match ditemukan. Jika match ada di JS hasil discovery, field ini akan berisi URL file JS tersebut.
- `discovered_from` menunjukkan halaman induk yang mereferensikan resource hasil discovery.
- Terminal selalu menampilkan hasil dalam bentuk plain.
- `-o/--output` sekarang fleksibel: jika nilainya `plain/json/csv`, hasil diekspor ke file default otomatis seperti `comot-20260422-201500.csv`. Jika nilainya nama file/path, hasil ditulis ke file tersebut dan tipe diambil dari extension file. Extension selain `.json` dan `.csv` dianggap `plain`.
- Built-in pattern dibaca dari `.comot.data/patterns.txt` dengan format `Nama || regex`, jadi saat binary dipindahkan sebaiknya file data ikut tersedia di lokasi kerja atau di samping binary.
