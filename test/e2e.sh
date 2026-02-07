#!/bin/bash
set -euo pipefail

BINARY="./bin/docker-stats-exporter"
PORT=9200
PID=""

cleanup() {
    if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
        kill "$PID"
        wait "$PID" 2>/dev/null || true
    fi
}
trap cleanup EXIT

echo "=== Building ==="
make build

echo "=== Starting exporter ==="
$BINARY &
PID=$!
sleep 2

echo "=== Testing /health ==="
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:$PORT/health)
if [ "$HTTP_CODE" != "200" ]; then
    echo "FAIL: /health returned $HTTP_CODE"
    exit 1
fi
echo "OK: /health returned 200"

echo "=== Testing /version ==="
VERSION_RESP=$(curl -s http://localhost:$PORT/version)
echo "$VERSION_RESP" | grep -q '"version"' || { echo "FAIL: /version missing version field"; exit 1; }
echo "OK: /version returned valid JSON"

echo "=== Testing /metrics ==="
METRICS=$(curl -s http://localhost:$PORT/metrics)

# Check for expected metric families
for metric in "exporter_build_info" "exporter_up"; do
    echo "$METRICS" | grep -q "$metric" || { echo "FAIL: missing $metric"; exit 1; }
    echo "OK: found $metric"
done

# Check for container metrics if Docker is available
if echo "$METRICS" | grep -q "container_memory_usage_bytes"; then
    echo "OK: found container metrics (Docker available)"
else
    echo "INFO: no container metrics (Docker may not be available)"
fi

echo ""
echo "=== All e2e tests passed ==="
