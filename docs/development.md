# Development Guide

This guide covers building, testing, verifying, and contributing to `protoc-gen-jev`.

## Prerequisites and Toolchain

This repository relies on [mise](https://mise.jdx.dev/) to manage all developer runtimes, compilers, and plugins, and [just](https://github.com/casey/just) to automate tasks.

All required tools—including Go, Buf, Python, Node.js, uv, linters, and Protobuf plugins—are defined in [`mise.toml`](../mise.toml).

To install and activate the complete development environment:

```sh
mise install
```

## Build and Code Generation

Build the compiler plugin binary:

```sh
just build
```

This compiles `./protoc-gen-jev` to the repository root.

Re-generate code across testdata and all examples:

```sh
just generate
```

To regenerate the canonical Go bindings for `jev/v1` protobuf options in `pkg/jev/v1`:

```sh
just generate-options
```

## Testing

Run the full verification suite (generation, syntax/typing checks, example compilation, behavior fixtures, and unit tests):

```sh
just test
```

### Test Suite Breakdown

- **Unit Tests**:
  ```sh
  go test -v ./internal/... ./pkg/... .
  ```
  Runs Go unit tests, golden file verifications, and parser validations.

- **Cross-Language Syntax & Type Checking**:
  ```sh
  just test-syntax
  ```
  Verifies generated Go code (`go vet`), Python syntax and mypy types (`mypy`), and TypeScript type definitions (`tsc --noEmit`).

- **Cross-Language Behavior Fixtures**:
  ```sh
  just test-behavior
  ```
  Executes contract tests in Python and TypeScript against the shared response fixture (`testdata/behavior/responses.json`) to guarantee identical score interpolation, half-away-from-zero rounding, and thresholding across all runtimes.

- **Example Compilation**:
  ```sh
  just compile-examples
  ```
  Verifies that all language examples in `examples/` compile cleanly against their generated code.

## Running Examples and Decision Backends

`protoc-gen-jev` generated clients communicate over the standard Jev wire protocol (`POST /v1/systemone`). This transport-agnostic design allows generated clients in Go, Python, and TypeScript to target any compliant decision model backend without code changes.

### Decision Backends

#### 1. Jev (TypeSafe Cloud API)

- **Role**: TypeSafe's hosted, high-throughput non-autoregressive decision model API.
- **Default Endpoint**: `https://api.typesafe.ai`
- **Authentication**: Requires a TypeSafe API key passed via the `TYPESAFE_API_KEY` environment variable (or directly to client constructors).
- **Usage**:
  ```sh
  export TYPESAFE_API_KEY="your-api-key"
  just run-examples
  ```
- **Notes**: Suitable for production workloads and cloud-based evaluations.

#### 2. Laya (Local Decision Engine)

- **Role**: Open-weights local decision model engine from TypeSafe.
- **Default Endpoint**: `http://127.0.0.1:8000`
- **Authentication**: None required for local evaluation.
- **Setup & Start**:
  ```sh
  just setup-laya   # Creates .venv-laya and installs laya[serve]
  just start-laya   # Starts the server in the foreground
  ```
- **Run Examples**:
  ```sh
  just run-examples-laya
  ```
- **Automatic Fallback**: If neither `TYPESAFE_API_KEY` nor `JEV_ENDPOINT` is set, `just run-examples` automatically connects to an existing Laya server at `http://127.0.0.1:8000` or provisions a temporary one in `.venv-laya`.
- **Notes**: On first run, Laya downloads the model weights from Hugging Face, which may take several minutes depending on network bandwidth.

#### 3. Clef (Cloudflare Decision Models)

- **Role**: Cloudflare's non-autoregressive decision model architecture (see [Cloudflare Clef Decision Models](https://blog.cloudflare.com/clef-decision-models/)).
- **Default Endpoint**: `http://127.0.0.1:8787` (local test server)
- **Authentication**: None for the local test server; standard Cloudflare API tokens when querying live Workers AI endpoints.
- **Start Local Server**:
  ```sh
  just start-clef
  ```
- **Run Examples**:
  ```sh
  just run-examples-clef
  ```
- **Notes**: Clef shares the identical decision primitives (`noul`, `choice`, `score`) and `/v1/systemone` JSON schema. The bundled [`testdata/clef_serve.py`](file:///Users/kevin/Projects/protoc-gen-jev/testdata/clef_serve.py) server provides a zero-dependency, rapid mock evaluator for local development and CI testing.

### Environment Variables

| Variable | Description |
|---|---|
| `TYPESAFE_API_KEY` | API key for the TypeSafe cloud Jev service. Omit when using local backends (Laya or Clef). |
| `JEV_ENDPOINT` | Base URL for the decision server (e.g. `http://127.0.0.1:8000` for Laya, `http://127.0.0.1:8787` for Clef). |
| `JEV_MODEL` | Override the model name requested (e.g. `jev-latest`, `clef`). |
| `JEV_VERBOSE` | Set to `1` for detailed question schemas and raw response JSON output. |

## Golden Files

Test suites compare generated code against expected golden files in `testdata/golden/`.

When modifying code generation templates or options:

1. Update the golden files:
   ```sh
   just update-golden
   ```
2. Review the diff with `git diff testdata/golden/` to confirm that generated output changes are expected.
3. Run `just test` to verify.

To remove golden files:

```sh
just clean-golden
```

## Python Dependency Management

Python development dependencies are pinned in `requirements-dev.txt` from `requirements-dev.in`:

```sh
just update-python-lock
```

## Linting and Formatting

Run all linters (Buf lint, Go vet, and golangci-lint):

```sh
just lint
```

Format Go code:

```sh
just format
```

## Releases

Releases are managed with [GoReleaser](https://goreleaser.com/):

```sh
just release
```
