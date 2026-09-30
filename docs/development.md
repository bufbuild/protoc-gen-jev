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

## Running Examples

### 1. Automatic (Default)

To run the Go, Python, and TypeScript examples:

```sh
just run-examples
```

If `TYPESAFE_API_KEY` is set, the examples call the TypeSafe API. Otherwise the recipe uses the Laya server at `http://127.0.0.1:8000` if one is already running, or starts a temporary one (installing it into `.venv-laya` on first use) and stops it when the examples finish. Set `JEV_ENDPOINT` to point at any other server.

### 2. Local Laya Server

To run against a local Laya evaluation server:

1. In one terminal, start the Laya server:
   ```sh
   just start-laya
   ```
2. In another terminal, run the multi-language examples:
   ```sh
   just run-examples-laya
   ```

Set `JEV_VERBOSE=1` for detailed question schemas and response JSON output.

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
