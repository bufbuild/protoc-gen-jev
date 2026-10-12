#!/usr/bin/env bash

# Runs the Go example against Clef on Cloudflare Workers AI.
#
# Credentials come from CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID when
# set, and otherwise from the Cloudflare CLI (wrangler auth token) and the
# Cloudflare accounts API. Only the Go example runs: the TypeSafe Python and
# TypeScript SDKs hardcode the /v1/systemone path and cannot target Workers AI.

set -euo pipefail

DIR="$(CDPATH= cd "$(dirname "${0}")/.." && pwd)"
cd "${DIR}"

: "${CLEF_MODEL:=clef-omni}"
: "${CLOUDFLARE_API_TOKEN:=}"
: "${CLOUDFLARE_ACCOUNT_ID:=}"

CLOUDFLARE_API="https://api.cloudflare.com/client/v4"

wrangler_cmd() {
	if command -v wrangler >/dev/null 2>&1; then
		wrangler "${@}"
	else
		npx --yes wrangler "${@}"
	fi
}

if [ -z "${CLOUDFLARE_API_TOKEN}" ]; then
	echo "[INFO] CLOUDFLARE_API_TOKEN is not set; asking the Cloudflare CLI for the active token..."
	if ! CLOUDFLARE_API_TOKEN="$(wrangler_cmd auth token --json | jq -r '.token // empty')" || [ -z "${CLOUDFLARE_API_TOKEN}" ]; then
		echo "Could not get a token from the Cloudflare CLI. Run 'wrangler login' or set CLOUDFLARE_API_TOKEN." >&2
		exit 1
	fi
fi

if [ -z "${CLOUDFLARE_ACCOUNT_ID}" ]; then
	if ! ACCOUNTS_JSON="$(curl -fsS -H "Authorization: Bearer ${CLOUDFLARE_API_TOKEN}" "${CLOUDFLARE_API}/accounts?per_page=50")"; then
		echo "Could not list Cloudflare accounts with the token. Check the token or set CLOUDFLARE_ACCOUNT_ID." >&2
		exit 1
	fi
	ACCOUNT_COUNT="$(jq -r '.result | length' <<<"${ACCOUNTS_JSON}")"
	if [ "${ACCOUNT_COUNT}" = "1" ]; then
		CLOUDFLARE_ACCOUNT_ID="$(jq -r '.result[0].id' <<<"${ACCOUNTS_JSON}")"
	else
		echo "The token can see ${ACCOUNT_COUNT} accounts; set CLOUDFLARE_ACCOUNT_ID to one of:" >&2
		jq -r '.result[] | "  \(.id)  \(.name)"' <<<"${ACCOUNTS_JSON}" >&2
		exit 1
	fi
fi

export JEV_ENDPOINT="${CLOUDFLARE_API}/accounts/${CLOUDFLARE_ACCOUNT_ID}/ai/run/@cf/cloudflare/${CLEF_MODEL}"
export TYPESAFE_API_KEY="${CLOUDFLARE_API_TOKEN}"

echo "=== Running Go Example against Workers AI @cf/cloudflare/${CLEF_MODEL} ==="
go run examples/go/main.go
echo ""
echo "✔ Go example completed successfully against Clef."
