#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

if [ -z "${TYPESAFE_API_KEY:-}" ] && [ -z "${JEV_ENDPOINT:-}" ]; then
	LAYA_URL="http://127.0.0.1:8000"
	if curl -fsS --max-time 3 "$LAYA_URL/health" >/dev/null 2>&1; then
		echo "[INFO] TYPESAFE_API_KEY is not set; using the Laya server already running at $LAYA_URL."
	else
		echo "[INFO] TYPESAFE_API_KEY is not set; starting a temporary Laya server at $LAYA_URL..."
		if [ ! -x ".venv-laya/bin/laya-serve" ]; then
			if command -v just >/dev/null 2>&1; then
				just --quiet setup-laya
			else
				if [ ! -d ".venv-laya" ]; then
					uv venv .venv-laya --python 3.12
				fi
				uv pip install --python .venv-laya/bin/python "laya[serve]"
			fi
		fi
		LAYA_LOG=$(mktemp)
		.venv-laya/bin/laya-serve --host 127.0.0.1 --port 8000 >"$LAYA_LOG" 2>&1 &
		LAYA_PID=$!
		cleanup() {
			echo "[INFO] Stopping temporary Laya server..."
			kill "$LAYA_PID" >/dev/null 2>&1 || true
			rm -f "$LAYA_LOG"
		}
		trap cleanup EXIT INT TERM
		# The first start downloads model weights, which can take several minutes.
		LAYA_TIMEOUT="${LAYA_STARTUP_TIMEOUT:-600}"
		echo "[INFO] Waiting up to ${LAYA_TIMEOUT}s for Laya to become healthy (first start downloads model weights)..."
		for (( waited=0; waited < LAYA_TIMEOUT; waited++ )); do
			if curl -fsS --max-time 1 "$LAYA_URL/health" >/dev/null 2>&1; then
				break
			fi
			if ! kill -0 "$LAYA_PID" 2>/dev/null; then
				echo "Laya server exited before becoming healthy:" >&2
				cat "$LAYA_LOG" >&2
				exit 1
			fi
			sleep 1
		done
		if ! curl -fsS --max-time 3 "$LAYA_URL/health" >/dev/null 2>&1; then
			echo "Laya server did not become healthy within ${LAYA_TIMEOUT}s:" >&2
			cat "$LAYA_LOG" >&2
			exit 1
		fi
	fi
	export JEV_ENDPOINT="$LAYA_URL"
fi

echo "=== Running Go Example ==="
go run examples/go/main.go
echo ""
echo "=== Running Python Example ==="
uv run python examples/python/main.py
echo ""
echo "=== Running TypeScript Example ==="
NODE_PATH="$REPO_ROOT/examples/typescript/node_modules" npx --prefix examples/typescript tsx examples/typescript/index.ts
echo ""
echo "✔ All language examples completed successfully!"
