#!/usr/bin/env bash
# Deterministic clean-checkout verification for the production Compose contract.
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
export COMPOSE_PROJECT_NAME="novelreader-e2e-${PPID}-$$"
export APP_PORT="${E2E_APP_PORT:-$((18000 + $$ % 1000))}"
export PUBLIC_URL="http://127.0.0.1:${APP_PORT}"
export NOVELREADER_IMAGE="${NOVELREADER_IMAGE:-novelreader:e2e}"
export NOVELREADER_WEBVIEW_IMAGE="${NOVELREADER_WEBVIEW_IMAGE:-novelreader-webview:e2e}"
export WEBVIEW_ENDPOINT="http://webview-worker:8787"

compose() {
  docker compose -f "$ROOT/compose.yaml" --profile webview --profile e2e "$@"
}

cleanup() {
  rm -f "$ROOT/.docker-e2e-search.json" "$ROOT/.docker-e2e-sources.json"
  compose down -v --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

compose config --quiet
compose build app webview-worker
compose up -d --no-build --wait --wait-timeout 180 app webview-worker fixture

base_url="http://127.0.0.1:${APP_PORT}"
index=$(curl -fsS "$base_url/")
grep -q 'NovelReader' <<<"$index"
grep -q '/assets/' <<<"$index"

health=$(curl -fsS "$base_url/api/healthz")
grep -q '"status":"ok"' <<<"$health"
worker_health=$(compose exec -T app wget -q -O - http://webview-worker:8787/healthz)
grep -q '"ok": true' <<<"$worker_health"
worker_container=$(compose ps -q webview-worker)
if docker inspect "$worker_container" --format '{{json .NetworkSettings.Ports}}' | grep -q 'HostPort'; then
  echo "webview worker unexpectedly publishes a host port" >&2
  exit 1
fi

curl -fsS -X POST -H 'Content-Type: application/json' -H "Origin: $PUBLIC_URL" \
  --data-binary "@$ROOT/testdata/docker/webview-source.json" \
  "$base_url/api/sources" >/dev/null
curl -fsS --get --data-urlencode 'q=Rendered Fixture Book' \
  "$base_url/api/search" >"$ROOT/.docker-e2e-search.json"
python3 - "$ROOT/.docker-e2e-search.json" <<'PY'
import json, sys
results = json.load(open(sys.argv[1], encoding="utf-8"))
assert any(item.get("name") == "Rendered Fixture Book" for item in results), results
PY
rm -f "$ROOT/.docker-e2e-search.json"

app_container=$(compose ps -q app)
compose stop -t 15 app >/dev/null
exit_code=$(docker inspect "$app_container" --format '{{.State.ExitCode}}')
if [[ "$exit_code" != 0 ]]; then
  echo "app exited with $exit_code after SIGTERM" >&2
  exit 1
fi
compose up -d --no-build --no-deps --force-recreate --wait --wait-timeout 60 app

curl -fsS "$base_url/api/sources" >"$ROOT/.docker-e2e-sources.json"
python3 - "$ROOT/.docker-e2e-sources.json" <<'PY'
import json, sys
sources = json.load(open(sys.argv[1], encoding="utf-8"))
assert any(item.get("bookSourceUrl") == "http://fixture:8000" for item in sources), sources
PY
rm -f "$ROOT/.docker-e2e-sources.json"
curl -fsS "$base_url/api/healthz" | grep -q '"status":"ok"'

echo "Docker E2E passed: frontend, readiness, private WebView, rendered search, graceful stop, persistence"
