#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

echo "[1/4] go test ./..."
go test ./...

echo "[2/4] build binary"
go build -o ./comot ./cmd/comot

echo "[3/4] prepare integration fixture"
rm -rf /tmp/comot-fixture
mkdir -p /tmp/comot-fixture/assets /tmp/comot-fixture/data
mkdir -p /tmp/comot-offdomain

cat >/tmp/comot-fixture/index.html <<'EOF'
<!doctype html>
<html>
<head>
  <script src="/assets/app.js"></script>
  <script src="/assets/deep.js"></script>
  <script src="http://127.0.0.1:8124/offdomain.js"></script>
</head>
<body>
  <a href="/data/spec.json">spec</a>
  <div>admin@example.com</div>
</body>
</html>
EOF

cat >/tmp/comot-fixture/assets/app.js <<'EOF'
const api = "/api/users";
const spec = "http://127.0.0.1:8123/data/spec.json";
//# sourceMappingURL=/assets/app.js.map
EOF

cat >/tmp/comot-fixture/assets/deep.js <<'EOF'
const next = "/data/more.json";
const gql = "/graphql";
EOF

cat >/tmp/comot-fixture/assets/app.js.map <<'EOF'
{"version":3,"sources":["app.ts"],"mappings":"","names":[]}
EOF

cat >/tmp/comot-fixture/data/spec.json <<'EOF'
{
  "openapi": "3.0.0",
  "servers": [{"url": "http://localhost:65530/api"}],
  "paths": {
    "/pets": {"get": {}},
    "/owners": {"get": {}},
    "/nested": {"get": {}}
  }
}
EOF

cat >/tmp/comot-fixture/data/more.json <<'EOF'
{"items":["/audit/logs","support@example.org"]}
EOF

cat >/tmp/comot-offdomain/offdomain.js <<'EOF'
const external = "/external-api";
EOF

cat >/tmp/comot-fixture/data/dup.json <<'EOF'
{"items":["same-value","same-value","same-value"]}
EOF

cat >/tmp/comot-fixture/assets/slow.js <<'EOF'
const slow = "/slow-resource";
EOF

python3 -m http.server 8123 --directory /tmp/comot-fixture >/tmp/comot-http.log 2>&1 &
SERVER_PID=$!
python3 -m http.server 8124 --directory /tmp/comot-offdomain >/tmp/comot-http2.log 2>&1 &
SERVER_PID_2=$!
python3 - <<'PY' >/tmp/comot-slow.log 2>&1 &
from http.server import HTTPServer, BaseHTTPRequestHandler
import time

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        time.sleep(2.5)
        self.send_response(200)
        self.send_header("Content-Type", "application/javascript")
        self.end_headers()
        self.wfile.write(b'const delayed = "/timeout-hit";')

server = HTTPServer(("127.0.0.1", 8125), Handler)
server.serve_forever()
PY
SERVER_PID_3=$!
cleanup() {
  kill "$SERVER_PID" >/dev/null 2>&1 || true
  kill "$SERVER_PID_2" >/dev/null 2>&1 || true
  kill "$SERVER_PID_3" >/dev/null 2>&1 || true
}
trap cleanup EXIT
sleep 1

pass() { printf 'PASS %s\n' "$1"; }
fail() { printf 'FAIL %s\n' "$1"; exit 1; }

echo "[4/4] run integration checks"

./comot --help >/tmp/t1.txt 2>/dev/null
grep -q -- '--max-crawl' /tmp/t1.txt && grep -q 'pattern,pattern_name,resource_url,matched_value' /tmp/t1.txt && pass help || fail help

./comot -u http://127.0.0.1:8123/index.html -p 'example\.(com|org)' -f 'pattern,pattern_name,resource_url,matched_value' >/tmp/t2.txt 2>/tmp/t2.err
grep -q 'example.com' /tmp/t2.txt && pass single_url_plain || fail single_url_plain

printf 'http://127.0.0.1:8123/index.html\n' >/tmp/targets.txt
./comot -l /tmp/targets.txt -p '/api/[A-Za-z]+' -d >/tmp/t3.txt 2>/tmp/t3.err
grep -q '/api/users' /tmp/t3.txt && pass list_input_discovery || fail list_input_discovery

printf 'http://127.0.0.1:8123/index.html\n' | ./comot --stdin -p 'support@example.org' -d >/tmp/t4.txt 2>/tmp/t4.err
grep -q 'support@example.org' /tmp/t4.txt && pass stdin_input || fail stdin_input

./comot -u http://127.0.0.1:8123/index.html -p '/pets' -d -o /tmp/out.json >/tmp/t5.txt 2>/tmp/t5.err
grep -Eq '[0-9]{2}:[0-9]{2}:[0-9]{2}' /tmp/t5.txt && grep -q '"matched_value": "/pets"' /tmp/out.json && pass export_json_plus_terminal_plain || fail export_json_plus_terminal_plain

rm -f comot-*.csv
./comot -u http://127.0.0.1:8123/index.html -p '/owners' -d -o csv >/tmp/t6.txt 2>/tmp/t6.err
LATEST_CSV="$(ls -1t comot-*.csv | sed -n '1p')"
test -n "$LATEST_CSV" && grep -q '/owners' "$LATEST_CSV" && grep -Eq '[0-9]{2}:[0-9]{2}:[0-9]{2}' /tmp/t6.txt && pass auto_csv_export || fail auto_csv_export

./comot -u http://127.0.0.1:8123/index.html -p '/pets' -d -f 'pattern,pattern_name,resource_url,matched_value' >/tmp/t7.txt 2>/tmp/t7.err
grep -q 'custom' /tmp/t7.txt && grep -q 'spec.json' /tmp/t7.txt && pass default_format_fields || fail default_format_fields

./comot -u http://127.0.0.1:8123/index.html -b email >/tmp/t8.txt 2>/tmp/t8.err
grep -q 'email' /tmp/t8.txt && grep -q 'admin@example.com' /tmp/t8.txt && pass builtin_pattern_cli || fail builtin_pattern_cli

./comot -u http://127.0.0.1:8123/index.html -d -p '/[A-Za-z]+' --max-crawl 2 >/tmp/t9.txt 2>/tmp/t9.err
grep -q '2/2' /tmp/t9.err && pass max_crawl_limit || fail max_crawl_limit

./comot -u http://127.0.0.1:8123/index.html -d -b 'Swagger/OpenAPI path' >/tmp/t10.txt 2>/tmp/t10.err
grep -q '/pets' /tmp/t10.txt && grep -q '/owners' /tmp/t10.txt && pass swagger_builtin || fail swagger_builtin

./comot -u http://127.0.0.1:8123/index.html -d -p '/pets' >/tmp/t11.txt 2>/tmp/t11.err
grep -q '/pets' /tmp/t11.txt && ! grep -q 'connect: connection refused' /tmp/t11.err && pass skip_unreachable_discovered_resource || fail skip_unreachable_discovered_resource

./comot -u http://127.0.0.1:8123/data/dup.json -p 'same-value' -D >/tmp/t12.txt 2>/tmp/t12.err
test "$(wc -l </tmp/t12.txt)" -eq 1 && pass dedup_enabled || fail dedup_enabled

./comot -u http://127.0.0.1:8123/data/dup.json -p 'same-value' --dedup=false >/tmp/t13.txt 2>/tmp/t13.err
test "$(wc -l </tmp/t13.txt)" -ge 3 && pass dedup_disabled || fail dedup_disabled

./comot -u http://127.0.0.1:8123/index.html -d -p '/external-api' >/tmp/t14.txt 2>/tmp/t14.err
! grep -q '/external-api' /tmp/t14.txt && pass offdomain_blocked || fail offdomain_blocked

./comot -u http://127.0.0.1:8123/index.html -d -a -p '/external-api' >/tmp/t15.txt 2>/tmp/t15.err
grep -q '/external-api' /tmp/t15.txt && pass offdomain_allowed || fail offdomain_allowed

./comot -u http://127.0.0.1:8125/slow.js -p '/timeout-hit' -t 500ms >/tmp/t16.txt 2>/tmp/t16.err && fail timeout_enforced || true
grep -qi 'timeout' /tmp/t16.err && pass timeout_enforced || fail timeout_enforced

COMOT_SCRIPTED_PROMPTS='{"builtin_names":["email"],"output_type":"plain"}' \
  ./comot -u http://127.0.0.1:8123/index.html >/tmp/t17.txt 2>/tmp/t17.err
grep -q 'admin@example.com' /tmp/t17.txt && pass scripted_interactive_url || fail scripted_interactive_url

COMOT_SCRIPTED_PROMPTS='{"builtin_names":["Swagger/OpenAPI path"],"output_type":"plain"}' \
  ./comot -l /tmp/targets.txt -d >/tmp/t18.txt 2>/tmp/t18.err
grep -q '/pets' /tmp/t18.txt && ! grep -q 'Target URL:' /tmp/t18.err && pass scripted_interactive_list || fail scripted_interactive_list

echo "ALL TESTS PASSED"
