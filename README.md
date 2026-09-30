# protoc-gen-jev

[![Go CI](https://img.shields.io/github/actions/workflow/status/sudorandom/protoc-gen-jev/go.yml?label=Go%20CI)](https://github.com/sudorandom/protoc-gen-jev/actions/workflows/go.yml)
[![GitHub Release](https://img.shields.io/github/v/release/sudorandom/protoc-gen-jev)](https://github.com/sudorandom/protoc-gen-jev/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**Define AI decisions in Protobuf. Generate type-safe clients for Go, TypeScript, and Python.**

Instead of writing loose text prompts and parsing untyped JSON, `protoc-gen-jev` turns your Protocol Buffer schemas into strongly typed clients for [Jev](https://typesafe.ai)—a fast, non-autoregressive cognitive AI model designed for parallel, structured evaluation.

```
┌─────────────────────────────────┐       buf generate        ┌───────────────────────────────────┐
│         Protobuf Schema         │ ────────────────────────> │      Type-Safe Client SDKs        │
│ Questions · Rubrics · Thresholds│      protoc-gen-jev       │  Go  ·  TypeScript  ·  Python     │
└─────────────────────────────────┘                           └───────────────────────────────────┘
```

> **Experimental**: Schema options and generated APIs are under active development. Pin the generator and runtime dependencies to the same release.

---

## Core Decision Primitives

Your Protobuf response fields map directly to Jev's decision primitives:

| Decision | Protobuf Type | Jev Evaluation Semantics |
|---|---|---|
| **Yes / No** (Noul) | `bool` | Binary classification with calibrated probability and configurable threshold. |
| **Rubric / Rating** (Score) | Numeric (`int32`, `int64`, `float`) | Rating along an ordered rubric scale, linearly interpolated into domain numbers with rounding. |
| **Categorical** (Choice) | `enum`, `string`, routing `oneof` | Discrete selection among predefined choices or enum values. |
| **Execution Metadata** | `jev.v1.Response`, `jev.v1.Meta` | Automatically populated with model name, token usage, confidence scores, and probability distributions. |

---

## Quickstart

### 1. Define your decisions in Protobuf

Add the dependency to your `buf.yaml`:

```yaml
version: v2
modules:
  - path: proto
deps:
  - buf.build/sudo-random/protoc-gen-jev
```

Annotate your response message in `proto/triage/v1/triage.proto`:

```protobuf
edition = "2024";
option features.field_presence = IMPLICIT;
package triage.v1;
option go_package = "example.com/myapp/gen/jev/triage/v1;triagev1";
import "jev/v1/options.proto";

message TriageRequest {
  string description = 1;
}

message TriageResponse {
  // Yes/No decision with an 85% confidence threshold
  bool page = 1 [
    features.field_presence = EXPLICIT,
    (jev.v1.field).instructions = "Does this incident require immediate paging?",
    (jev.v1.field).noul = { threshold: 0.85 }
  ];

  // Rubric decision evaluated on a 1-5 scale
  int32 urgency = 2 [
    (jev.v1.field).instructions = "How urgently does this incident need attention?",
    (jev.v1.field).score = {
      levels: [
        { value: 1, description: "Minor; can wait for routine maintenance" },
        { value: 3, description: "Degraded service; a workaround exists" },
        { value: 5, description: "Active outage; immediate intervention required" }
      ]
    }
  ];
}

service TriageService {
  rpc Triage(TriageRequest) returns (TriageResponse);
}
```

### 2. Generate code with Buf

Configure `buf.gen.yaml`:

```yaml
version: v2
clean: true
inputs:
  - directory: proto
plugins:
  - remote: buf.build/protocolbuffers/go:v1.36.12
    out: gen
    opt: [paths=source_relative]
  - local: protoc-gen-jev
    out: gen
    opt: [targets=go, paths=source_relative]
```

Generate the client:

```sh
buf dep update && buf generate
```

### 3. Call your type-safe client

<details open>
<summary><b>Go</b></summary>

```go
client := triagev1.NewJevTriageService(jev.NewClient(os.Getenv("TYPESAFE_API_KEY")))

decision, err := client.Triage(ctx, &triagev1.TriageRequest{
    Description: "Database connection pool exhausted; requests failing with 500",
})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Page: %v, Urgency: %d\n", decision.GetPage(), decision.GetUrgency())
```
</details>

<details>
<summary><b>TypeScript</b></summary>

```ts
import { create } from "@bufbuild/protobuf";
import { JevTriageService } from "./gen/triage/v1/triage_jev.js";
import { TriageRequestSchema } from "./gen/triage/v1/triage_pb.js";

const client = new JevTriageService();

const decision = await client.triage(
  create(TriageRequestSchema, {
    description: "Database connection pool exhausted; requests failing with 500",
  })
);

console.log(`Page: ${decision.page}, Urgency: ${decision.urgency}`);
```
</details>

<details>
<summary><b>Python</b></summary>

```python
from jev.triage.v1.triage_pb import TriageRequest
from jev.triage.v1.triage_jev import JevTriageService

client = JevTriageService()

decision = client.triage(
    TriageRequest(description="Database connection pool exhausted; requests failing with 500")
)

print(f"Page: {decision.page}, Urgency: {decision.urgency}")
```
</details>

---

## Language Matrix

| Language | Generator Flag | Runtime Dependencies | Complete Working Example |
|---|---|---|---|
| **Go** | `targets=go` | `google.golang.org/protobuf`, `pkg/jev` | [examples/go/](examples/go/) |
| **TypeScript** | `targets=ts` | `@bufbuild/protobuf` (v2), `@typesafe-ai/sdk` | [examples/typescript/](examples/typescript/) |
| **Python** | `targets=python` | `protobuf-py` (v0.5+), `typesafe-sdk` | [examples/python/](examples/python/) |
| **JSON** | `targets=json` | None (emits language-agnostic `.jev.json` spec) | [examples/json/](examples/json/) |

Generate multiple targets simultaneously: `opt: [targets=go,targets=ts]`.

---

## Installation

- **Pre-compiled Binary**: Download from [GitHub Releases](https://github.com/sudorandom/protoc-gen-jev/releases/latest) and place on `PATH`.
- **Go Install**: `go install github.com/sudorandom/protoc-gen-jev@latest`
- **Mise**: `mise use github:sudorandom/protoc-gen-jev@latest`

---

## Documentation

- **[Schema & Client Reference](docs/reference.md)**: Field annotations (`instructions`, `score`, `noul`, `choice`, `skip`), routing oneofs, detailed decision types (`jev.v1.Noul`, `Score`, `Choice`), metadata inspection (`jev.v1.Response`), and error handling.
- **[Development Guide](docs/development.md)**: Local build instructions, unit and cross-language tests, mock FauxRPC/Laya containers, and golden file workflows.

---

## License

MIT licensed; see [LICENSE](LICENSE).
