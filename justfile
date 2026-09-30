# Justfile for protoc-gen-jev
set shell := ["bash", "-c"]
export TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE := env_var_or_default("TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE", "/var/run/docker.sock")

# Default recipe: lint and run all tests
default: lint test

# ------------------------------------------------------------------------------
# Build & Generation
# ------------------------------------------------------------------------------

# Compile the protoc-gen-jev binary
build:
	@echo "Building protoc-gen-jev..."
	go build -o protoc-gen-jev .
	@echo "✔ Binary built: ./protoc-gen-jev"

# Generate code using buf
generate: build
	@echo "Running buf generate for testdata..."
	buf generate
	@echo "Running buf generate for examples..."
	for dir in examples/go examples/typescript examples/python examples/json; do \
		(cd "$dir" && PATH="{{ justfile_directory() }}:$PATH" buf generate) || exit 1; \
	done
	@echo "✔ Code generation complete."

# Re-generate Go bindings for jev/v1 options
generate-options:
	@echo "Regenerating Go bindings for jev/v1/options.proto..."
	buf generate proto --template '{"version":"v2","plugins":[{"local":"protoc-gen-go","out":".","opt":["module=github.com/bufbuild/protoc-gen-jev"]}]}'
	@echo "✔ Options bindings regenerated."


# ------------------------------------------------------------------------------
# Testing & Golden Files
# ------------------------------------------------------------------------------

# Install the dependencies used to compile generated TypeScript and the example
install-typescript-deps:
	npm --prefix examples/typescript ci --no-audit

# Run syntax and type checks across all generated languages
test-syntax: generate install-typescript-deps install-python-deps
	@echo "Checking generated Go code..."
	go vet ./gen/jev/...
	@echo "Checking generated Python syntax..."
	python -c "import py_compile, glob; [py_compile.compile(f, doraise=True) for f in glob.glob('gen/jev/**/*.py', recursive=True)]"
	@echo "Checking generated TypeScript type signatures..."
	npx --prefix examples/typescript tsc --noEmit -p testdata/tsconfig.json
	.venv/bin/mypy gen/jev testdata/behavior/check.py
	@echo "✔ Generated Go, Python, and TypeScript code verified."

# Compile and verify all examples without executing them
compile-examples: generate install-typescript-deps
	@echo "Compiling Go example..."
	go build -o /dev/null ./examples/go
	@echo "Compiling Python example..."
	@if [ ! -d ".venv" ]; then \
		echo "Creating Python virtualenv via uv..."; \
		uv venv .venv; \
	fi
	uv pip install -q -r examples/python/requirements.txt
	uv run python -c "import py_compile; py_compile.compile('examples/python/main.py', doraise=True)"
	@echo "Compiling TypeScript example..."
	npx --prefix examples/typescript tsc --noEmit -p examples/typescript/tsconfig.json
	@echo "✔ All examples compiled successfully."

# Run all unit tests, syntax checks, golden file verification, and compile examples
test: generate test-syntax compile-examples test-behavior
	@echo "Running Go tests..."
	go test -v ./internal/... ./pkg/... .

# Run real end-to-end examples across all languages (Go, Python, TypeScript)
run-examples: compile-examples
	#!/usr/bin/env bash
	set -euo pipefail
	if [ -z "${TYPESAFE_API_KEY:-}" ] && [ -z "${JEV_ENDPOINT:-}" ]; then
		echo "[INFO] Neither TYPESAFE_API_KEY nor JEV_ENDPOINT is set."
		echo "[INFO] Starting temporary FauxRPC mock container on port 6660..."
		CID=$(docker run -d --rm -p 6660:6660 -v "{{ justfile_directory() }}/testdata/openapi:/openapi:ro" docker.io/sudorandom/fauxrpc:v0.29.1 run --schema=/openapi/typesafe-jev.yaml --stubs=/openapi/stubs.jev.yaml --addr=0.0.0.0:6660)
		cleanup() {
			echo "[INFO] Stopping FauxRPC mock container..."
			docker stop "$CID" >/dev/null 2>&1 || true
		}
		trap cleanup EXIT INT TERM
		for i in {1..30}; do
			if curl -sf http://localhost:6660/fauxrpc/openapi-docs/ >/dev/null; then
				break
			fi
			sleep 0.2
		done
		export JEV_ENDPOINT="http://localhost:6660"
	fi

	echo "=== Running Go Example ==="
	go run examples/go/main.go
	echo ""
	echo "=== Running Python Example ==="
	uv run python examples/python/main.py
	echo ""
	echo "=== Running TypeScript Example ==="
	NODE_PATH="{{ justfile_directory() }}/examples/typescript/node_modules" npx --prefix examples/typescript tsx examples/typescript/index.ts
	echo ""
	echo "✔ All language examples completed successfully!"

# Setup local Laya environment in .venv-laya
setup-laya:
	@if [ ! -d ".venv-laya" ]; then \
		echo "Creating .venv-laya with Python 3.12..."; \
		uv venv .venv-laya --python 3.12; \
	fi
	@echo "Installing laya[serve]..."
	uv pip install --python .venv-laya/bin/python "laya[serve]"
	@echo "✔ Laya environment ready."

# Start local laya-serve in the foreground on port 8000
start-laya: setup-laya
	@echo "Starting Laya server on http://127.0.0.1:8000 ..."
	.venv-laya/bin/laya-serve --host 127.0.0.1 --port 8000

# Run multi-language examples targeting a running local Laya server
run-examples-laya:
	#!/usr/bin/env bash
	set -euo pipefail
	if ! curl -fsS --max-time 3 http://127.0.0.1:8000/health >/dev/null 2>&1; then
	    echo "Laya is not running at http://127.0.0.1:8000. Start it with: just start-laya" >&2
	    exit 1
	fi
	echo "Laya · http://127.0.0.1:8000"
	echo "Preparing examples…"
	setup_log=$(mktemp)
	trap 'rm -f "$setup_log"' EXIT
	if ! just --quiet compile-examples >"$setup_log" 2>&1; then
	    echo "Example preparation failed:" >&2
	    cat "$setup_log" >&2
	    exit 1
	fi
	if [ "${JEV_VERBOSE:-}" = "1" ]; then cat "$setup_log"; fi
	echo
	JEV_ENDPOINT="http://127.0.0.1:8000" go run examples/go/main.go
	echo
	JEV_ENDPOINT="http://127.0.0.1:8000" .venv/bin/python examples/python/main.py
	echo
	JEV_ENDPOINT="http://127.0.0.1:8000" NODE_PATH="{{ justfile_directory() }}/examples/typescript/node_modules" ./examples/typescript/node_modules/.bin/tsx examples/typescript/index.ts
	echo
	echo "Completed: Go, Python, TypeScript · 4 incidents each"


# Update golden files with newly generated files
update-golden: generate
	@echo "Updating golden files..."
	mkdir -p testdata/golden
	rm -rf testdata/golden/*
	cp -r gen/jev/* testdata/golden/
	@echo "✔ Golden files updated in testdata/golden"

# Delete existing golden files
clean-golden:
	@echo "Deleting golden files..."
	rm -rf testdata/golden
	@echo "✔ Golden files removed."

# ------------------------------------------------------------------------------
# Linting & Formatting
# ------------------------------------------------------------------------------

# Run all linters
lint: generate
	@echo "Linting protobuf files..."
	buf lint
	for dir in examples/typescript examples/python examples/json; do \
		(cd "$dir" && buf lint) || exit 1; \
	done
	@echo "Running go vet..."
	go vet ./internal/... ./pkg/... ./examples/... .
	@echo "Running golangci-lint..."
	golangci-lint run ./...

# Run goreleaser to create a release or check configuration
release *args="release --clean":
	goreleaser {{ args }}

# Format all Go code
format:
	@echo "Formatting Go code..."
	golangci-lint run --fix ./...
	go fmt ./...


# ------------------------------------------------------------------------------
# Cleanup
# ------------------------------------------------------------------------------

# Clean build artifacts and generated files
clean:
	@echo "Cleaning artifacts..."
	rm -rf gen/ examples/*/gen/ protoc-gen-jev .venv/ .venv-laya/
	@echo "✔ Clean complete."

# Resolve and pin Python development dependencies (intentional dependency update).
update-python-lock:
    uv pip compile requirements-dev.in -o requirements-dev.txt

install-python-deps:
    @if [ ! -d ".venv" ]; then uv venv .venv; fi
    uv pip install --python .venv/bin/python -r requirements-dev.txt

# Shared malformed-response, mapping, and cross-language regression cases.
test-behavior: generate install-typescript-deps install-python-deps
    NODE_PATH="{{ justfile_directory() }}/examples/typescript/node_modules" node --require ./examples/typescript/node_modules/tsx/dist/cjs/index.cjs testdata/behavior/check.ts
    .venv/bin/python testdata/behavior/check.py
