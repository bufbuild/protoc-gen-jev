# Schema and client reference

For installation and a first example, see the [README](../README.md).

## Protobuf edition

Repository schemas use `edition = "2024"`. Existing scalar fields retain implicit presence via `features.field_presence = IMPLICIT`; fields that previously used `optional` now use `features.field_presence = EXPLICIT`. New Edition 2024 schemas use explicit presence by default and do not use the `optional` keyword.

Jev's types and the incident examples use Edition 2024's default Go Opaque API. The custom JSON unmarshaler populates messages through setters; generated clients use setters for opaque application responses and direct assignments for Open API responses. Open API schemas remain in the compatibility fixtures.

`features.default_symbol_visibility = EXPORT_ALL` preserves cross-file use of nested messages. Proto3 remains covered by compatibility tests.

## Generator options

Targets are `go`, `ts`/`typescript`, `python`/`py`, `json`, or `all` (the default). JSON specifications accompany language output. Repeat the option to combine languages: `targets=go,targets=ts`. Unknown targets are errors.

The native generators in the example configuration exclude `jev.v1` to omit annotation descriptors and their runtime imports. The Jev generator still needs those annotations, so it must not use that exclusion.

Imported application messages need their native bindings generated too. Set `go_package` to the generated package’s full import path, including the Go module prefix.

Python uses Buf’s `protoc-gen-py` and `protobuf-py` runtime. Standard `protoc --python_out` bindings are incompatible. Keep native bindings and Jev clients under the same output package. For `out: gen/jev`, add `gen` to `PYTHONPATH` and import clients through `jev`. Relative imports allow the output package to be renamed.

JSON specifications contain questions and mapping metadata. Use `BuildQuestions`/`buildQuestions`/`build_questions` to get the HTTP API’s question map.

## Field options

### `instructions`

Optional. Overrides the field comment; if neither is provided, the generator uses a generic prompt such as `Evaluate urgencyRating`. Instructions improve question quality but are not required.

```protobuf
string label = 1 [(jev.v1.field).instructions = "Choose the best label."];
```

### `skip`

Excludes the field or oneof member from decision questions.

```protobuf
string debug_note = 2 [(jev.v1.field).skip = true];
```

### `choice.choices`

Allowed string labels or enum names.

```protobuf
string status = 3 [(jev.v1.field).choice = {
  choices: ["OPEN", "CLOSED"]
}];
```

### `choice.not_in`

Excludes enum names from decision options.

```protobuf
Status status = 4 [(jev.v1.field).choice = {
  not_in: ["STATUS_INTERNAL"]
}];
```

### `choice.criteria`

Descriptions for allowed labels; cannot bypass filtering.

```protobuf
string status = 5 [(jev.v1.field).choice = {
  criteria: {
    key: "OPEN",
    value: "Work remains"
  }
}];
```

### `score.levels`

Specifies 2–10 ordered `{ value, description }` rubric levels.

```protobuf
int32 urgency = 6 [(jev.v1.field).score = {
  levels: [
    { value: 1, description: "Can wait" },
    { value: 5, description: "Act now" }
  ]
}];
```

`score.min`, `score.max`, and `score.scale` are not supported. Level values must be finite and strictly increasing. Integer fields require integral level values within the supported numeric domain. Provide domain-specific descriptions; numerical labels alone make weak rubrics.

### `noul.threshold`

Probability threshold from `0` to `1`; defaults to `0.5`.

```protobuf
bool page = 7 [(jev.v1.field).noul = {
  threshold: 0.85
}];
```

## Service and method options

An RPC is discovered automatically when its request or response has a Jev field annotation. An annotation is optional for naturally mapped fields: bools become Noul decisions, numeric fields become Score decisions, enums become Choice decisions, and supported oneofs become routing choices. These fields use generated prompts and default rubrics where needed. Plain strings are ignored unless configured with `choice` rules.

Set `(jev.v1.service).enabled = true` or `(jev.v1.method).enabled = true` to discover an RPC without field annotations. This opts the RPC into generation; its response must still contain at least one usable decision field. Use field annotations to customize instructions, thresholds, rubrics, allowed string choices, or skip fields. Explicit false disables the service or method. Standalone clients are independently generated for messages with naturally mapped decision fields, including nested declarations; disabling an RPC does not disable standalone clients.

```protobuf
service IncidentService {
  option (jev.v1.service).enabled = true;

  rpc Triage(TriageRequest) returns (TriageResponse) {
    option (jev.v1.method).enabled = true;
  }
}
```

## Supported fields

| Feature | Support |
| --- | --- |
| Multiple files per package; imported request/response types | Supported |
| Nested message declarations and imported enums | Supported |
| Optional scalar decisions and custom JSON names | Supported; native Protobuf conversion preserves presence |
| Bool | Noul |
| Enum | Choice; zero/`_UNSPECIFIED` values excluded |
| String | Choice when explicitly configured; otherwise ignored in responses |
| Signed, unsigned, fixed and floating point numbers | Score |
| Decision oneof | String/bytes members only; selected label stored as the member value |
| Other oneof member kinds | Rejected during generation |
| Request fields | Scalars, enums and optional scalar fields |
| Repeated, map, message or bytes request fields | Rejected |
| Detailed decision messages (`jev.v1.Noul`, `Score`, `Choice`) | Supported as singular fields; see below |
| Other complex response fields | Ignored when unannotated; rejected when annotated unless skipped |
| Execution metadata (`jev.v1.Meta`) | Supported; automatically populated with model and usage |
| Response metadata (`jev.v1.Response`) | Supported; automatically populated with model execution details |
| Streaming RPCs | Rejected |

Integer score level values for 64-bit fields are restricted to `±(2^53−1)` (unsigned: `0..2^53−1`) so interpolation is consistent across languages. Native 64-bit responses still use Go integers, Python integers and TypeScript `bigint`. Request integers retain their full Protobuf range.

A decision oneof selects a route; it does not generate arbitrary member content. Prefer an enum if only a routing label is needed.

## Client methods

Generated client methods return native Protobuf messages directly:

| Operation | Go | TypeScript | Python |
| --- | --- | --- | --- |
| Service call | `Triage(ctx, req)` | `triage(req)` | `triage(req)` |
| Batch service call | `BatchTriage(ctx, reqs)` | `batchTriage(reqs)` | `batch_triage(reqs)` |
| Standalone evaluation | `Evaluate(ctx, req, ...)` | `evaluate(req)` | `evaluate(req)` |
| Batch evaluation | `Batch(ctx, reqs, ...)` | `batch(reqs)` | `batch(reqs)` |

TypeScript requests are created with `create(TriageRequestSchema, {...})`, imported from the native `_pb` module. Python returns native Protobuf messages, not dataclasses.

## Scalar decisions and detailed decision messages

Choose the response shape that your callers need. The [Go incident schema](../examples/go/proto/incident/v1/incident.proto) demonstrates both styles with `Triage` and `TriageDetails`. Each method returns its declared Protobuf response directly.

| Scalar style (`Triage`) | Detailed style (`TriageDetails`) | Details retained |
| --- | --- | --- |
| `bool` | `jev.v1.Noul` | Thresholded `value`, raw `probability`, optional `confidence` |
| Numeric field | `jev.v1.Score` | Interpolated `value`, raw `score` position, optional `confidence`, `probabilities`, `legend` |
| Enum or configured string | `jev.v1.Choice` | Selected string `value`, optional `confidence`, `probabilities` |
| Decision oneof | `jev.v1.Choice` with explicit labels | Routing label and its confidence/distribution |

Import `jev/v1/response.proto` to use these messages. Scalar and detailed fields can be mixed in one response. Detailed decision fields must be singular. They use the same field instructions, Noul threshold, and Score rubric options as scalar fields. A `Choice` needs explicit `choice` rules defining allowed labels through `choices` or `criteria`; it does not infer an enum from its string value.

```protobuf
import "jev/v1/options.proto";
import "jev/v1/response.proto";

message TriageDetailsResponse {
  jev.v1.Noul page = 1 [
    (jev.v1.field).instructions = "Does this incident require immediate paging?",
    (jev.v1.field).noul = { threshold: 0.85 }
  ];
  jev.v1.Score urgency = 2 [(jev.v1.field).score = {
    levels: [
      { value: 1, description: "Can wait" },
      { value: 5, description: "Act now" }
    ]
  }];
  jev.v1.Choice route = 3 [(jev.v1.field).choice = {
    choices: ["runbook", "oncall"]
  }];
  jev.v1.Meta meta = 99;
}
```

`Noul.value` is true when the probability is **greater than or equal to** the threshold (default `0.5`). Its probability is the binary decision probability, separate from the optional provider confidence. A legacy boolean answer is represented as probability `0` or `1`.

`Score.score` is a zero-based, potentially fractional rubric position. With the two levels above, position `0.375` produces `value = 2.5`. `Score.value` is a double and keeps the fraction; an `int32` decision would round it to `3`. Probability maps and legends are retained as supplied by the provider; they are not used to recompute the domain value.

`Choice.value` is a label, including when labels happen to be enum names. It is not an enum-typed field or a selected oneof member. Confidence is optional: absence differs from an explicit zero. Missing distributions and legends remain empty maps.

`jev.v1.Meta` carries only `model` and `usage` (`input_tokens`, `output_tokens`). A singular field is automatically populated without generating a question. At most one Meta field is allowed per response. Use `jev.v1.Response` below when you also want the full answer map, including alongside scalar decisions.

For the Go incident example, consume the returned protobuf directly:

```go
details, err := client.TriageDetails(ctx, req)
if err != nil {
    return err
}
page := details.GetRequiresImmediatePaging()
fmt.Println(page.GetValue(), page.GetProbability())
fmt.Println(details.GetUrgencyRating().GetValue())
fmt.Println(details.GetPriority().GetValue())
fmt.Println(details.GetMeta().GetModel())
```

The Go runtime deserializes the HTTP body once into `jev.v1.WireResponse`, validates the answers, and applies thresholds and rubric interpolation. Generated Go code then assigns these decisions directly to the declared response fields, including enums, oneofs, and optional scalars. The runtime does not reflect over application messages. It does not marshal intermediate JSON and deserialize it again. Callers receive the populated response and need no additional deserialization or metadata envelope.

## Provider metadata and execution details

To access provider metadata such as model name, token usage, and individual question answers (including confidence scores and probability distributions), add a field of type `jev.v1.Response` to your response message:

```protobuf
import "jev/v1/options.proto";
import "jev/v1/response.proto";

message TriageResponse {
  bool page = 1 [
    (jev.v1.field).instructions = "Does this incident require immediate paging?",
    (jev.v1.field).noul = { threshold: 0.85 }
  ];
  int32 urgency = 2 [
    (jev.v1.field).instructions = "How urgently does this incident need attention?",
    (jev.v1.field).score = {
      levels: [
        { value: 1, description: "Minor; can wait for routine maintenance" },
        { value: 5, description: "Active outage; immediate intervention required" }
      ]
    }
  ];

  // Automatically populated with execution metadata and question answers:
  jev.v1.Response jev_response = 99;
}
```

When present on a decision response message:

- The generator skips the field during question extraction. At most one `jev.v1.Response` field is permitted per message; multiple response fields or repeated response fields are rejected with an error.
- The runtime automatically populates it with:
  - `model`: The model used by Jev.
  - `usage`: `input_tokens` and `output_tokens`.
  - `answers`: A map from decision field name to `jev.v1.Answer`, containing the question type, selected choice, raw score or noul probability, confidence, probability distribution, and rubric legend.

This eliminates the need for separate metadata wrapper envelopes or tuple returns—your application works entirely with standard Protobuf messages.

## Response conversion and errors

Every requested decision must be present. Unknown choice labels, wrong answer types, missing/null values, and out-of-range scores or probabilities cause an error. Canonical `answers` require the correct `type` discriminator. Legacy `choices`/`scores`/`nouls` groups are accepted when `answers` is absent; an invalid canonical answer never falls back to a legacy group. Boolean Noul values are accepted for compatibility with backends that have already thresholded them.

Scores are zero-based rubric positions. A fractional position is linearly interpolated between adjacent domain values. Integer results round halfway **away from zero** in all languages. This conversion is not the same as computing an expected domain value from the full distribution when levels are unevenly spaced; inspect answer probabilities via `jev.v1.Response` if that is what your application needs.

Protobuf requests use standard Protobuf JSON names in every language. Standalone raw JSON is passed as supplied. Generated batch methods execute sequentially, preserve order, and stop on the first failure. They do not return partial results or start later requests after failure.

Go supports context cancellation, an injectable HTTP client, a default 30-second timeout, and a 16 MiB response limit. Configure the client using `jev.NewClient(apiKey)` and its `BaseURL`, `Model`, and `HTTPClient` fields. Go does not automatically retry; TypeScript/Python use their SDK's retry configuration. Inject a configured client to control endpoint, model, timeouts, retry policy and lifetime.

## Development and testing

For contributor workflows, building, running tests, golden file updates, and mock/Laya environments, see the [Development Guide](development.md).
