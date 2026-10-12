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

#### 3. Clef (Cloudflare Workers AI)

- **Role**: Cloudflare's open-weight decision models, [Clef and Clef-flash](https://blog.cloudflare.com/clef-decision-models/) and the multimodal [Clef-omni](https://blog.cloudflare.com/clef-faster-cheaper-multimodal/), hosted on [Workers AI](https://developers.cloudflare.com/workers-ai/models/clef/). The request and response bodies match Jev, but Workers AI serves them at `/client/v4/accounts/{account}/ai/run/@cf/cloudflare/{model}` and wraps the response in a `result` envelope.
- **Endpoint**: `https://api.cloudflare.com/client/v4/accounts/<ACCOUNT_ID>/ai/run/@cf/cloudflare/clef-omni` (or `clef`, `clef-flash`)
- **Authentication**: A Cloudflare API token with Workers AI access. `scripts/run-examples-clef.sh` uses `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` when set, and otherwise asks the Cloudflare CLI for the token (`wrangler auth token --json`) and looks up the account with the Cloudflare accounts API.
- **Run Examples**:
  ```sh
  wrangler login              # once, unless CLOUDFLARE_API_TOKEN is set
  just run-examples-clef      # @cf/cloudflare/clef-omni
  just run-examples-clef clef-flash
  ```
- **Notes**: Only the Go example runs against Clef. The Go client in `pkg/jev` recognises a Workers AI URL in `BaseURL`, sends the model named by that URL, and unwraps the envelope. The TypeSafe Python and TypeScript SDKs hardcode the `/v1/systemone` path and parse the bare response, so they cannot target Workers AI without a proxy. Workers AI bills Clef per input token; see the model page for pricing.

### Environment Variables

| Variable | Description |
|---|---|
| `TYPESAFE_API_KEY` | Bearer token for the decision server: a TypeSafe API key for Jev, or a Cloudflare API token for Clef. Omit for Laya. |
| `JEV_ENDPOINT` | Base URL for the decision server (e.g. `http://127.0.0.1:8000` for Laya, or a Workers AI model URL for Clef). |
| `CLOUDFLARE_API_TOKEN` | Cloudflare API token used by `just run-examples-clef`. When unset, the token comes from `wrangler auth token`. |
| `CLOUDFLARE_ACCOUNT_ID` | Cloudflare account for `just run-examples-clef`. When unset, the single account visible to the token is used. |
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
