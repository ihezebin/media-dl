#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
project_dir="$PWD"
command -v yarn >/dev/null || { echo "Please install yarn before running make dev." >&2; exit 1; }
command -v go >/dev/null || { echo "Please install Go before running make dev." >&2; exit 1; }
export MEDIA_DL_PORT="${MEDIA_DL_PORT:-8080}"
export MEDIA_DL_WEB_PORT="${MEDIA_DL_WEB_PORT:-3000}"
[[ -x webui/node_modules/.bin/vite ]] || (cd webui && yarn install --frozen-lockfile)
air_bin="$project_dir/.package/tools/air-${AIR_VERSION:-v1.61.7}"
if [[ ! -x "$air_bin" ]]; then
  mkdir -p "$project_dir/.package/tools"
  GOBIN="$project_dir/.package/tools" go install "github.com/air-verse/air@${AIR_VERSION:-v1.61.7}"
  mv "$project_dir/.package/tools/air" "$air_bin"
fi
mkdir -p .package/dev
echo "WebUI: http://127.0.0.1:$MEDIA_DL_WEB_PORT (Vite HMR)"
echo "API:   http://127.0.0.1:$MEDIA_DL_PORT (Air hot reload)"
# Separate process groups let cleanup stop Air's server and Vite's children too.
set -m
pids=()
cleanup() {
  trap - EXIT INT TERM
  for pid in "${pids[@]}"; do
    kill -TERM -- "-$pid" 2>/dev/null || true
  done
  wait || true
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
"$air_bin" -c .air.toml & pids+=("$!")
(cd webui && exec yarn dev) & pids+=("$!")
while kill -0 "${pids[0]}" 2>/dev/null && kill -0 "${pids[1]}" 2>/dev/null; do
  sleep 1
done
exit 1
