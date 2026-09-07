#!/bin/bash
#
# 本地冒烟：编译、启动服务、逐接口 curl 校验、退出时清理进程。
# 用法: bash scripts/smoke.sh            （默认端口 18080）
#       SMOKE_PORT=18090 bash scripts/smoke.sh

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PORT="${SMOKE_PORT:-18080}"
BASE="http://127.0.0.1:${PORT}"
BIN="$ROOT/bin/api-smoke"

cd "$ROOT"
go build -o "$BIN" ./cmd/api

HTTP_ADDR=":${PORT}" "$BIN" >/dev/null 2>&1 &
PID=$!
trap 'kill "$PID" 2>/dev/null || true; rm -f "$BIN"' EXIT

for _ in $(seq 1 50); do
    if curl -fs "$BASE/healthz" >/dev/null 2>&1; then
        break
    fi
    sleep 0.1
done

if ! kill -0 "$PID" 2>/dev/null; then
    echo "FAIL: 服务未能启动（端口 ${PORT} 可能被占用）"
    exit 1
fi

fail=0

# check METHOD PATH WANT_STATUS WANT_BODY_SUBSTR [JSON_BODY]
check() {
    local method="$1" path="$2" want_status="$3" want_body="$4" data="${5:-}"
    local out status body
    if [[ -n "$data" ]]; then
        out=$(curl -s -o /dev/stdout -w $'\n%{http_code}' -X "$method" -H 'Content-Type: application/json' --data "$data" "$BASE$path")
    else
        out=$(curl -s -o /dev/stdout -w $'\n%{http_code}' -X "$method" "$BASE$path")
    fi
    status="${out##*$'\n'}"
    body="${out%$'\n'*}"
    if [[ "$status" == "$want_status" && "$body" == *"$want_body"* ]]; then
        echo "OK    $method $path -> $status $body"
    else
        echo "FAIL  $method $path -> $status $body (want $want_status containing $want_body)"
        fail=1
    fi
}

check GET  /healthz   200 '"status":"ok"'
check GET  /v1/ping   200 '"message":"pong"'
check POST /v1/ping   405 ''
check GET  /not-found 404 ''

# 1.0.0 笔记：创建后用返回的 id 读取
created=$(curl -s -X POST -H 'Content-Type: application/json' --data '{"title":"smoke","content":"c"}' "$BASE/v1/notes")
note_id=$(printf '%s' "$created" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
check POST   /v1/notes                 201 '"id":"n_'                  '{"title":"smoke","content":"c"}'
check GET    "/v1/notes/${note_id}"    200 '"title":"smoke"'
check GET    /v1/notes/n_000000000000  404 '"code":"not_found"'
check POST   /v1/notes                 400 '"code":"invalid_argument"' '{"title":"   "}'
check DELETE "/v1/notes/${note_id}"    405 ''

if [[ "$fail" -ne 0 ]]; then
    echo "smoke FAILED"
    exit 1
fi
echo "smoke passed"
