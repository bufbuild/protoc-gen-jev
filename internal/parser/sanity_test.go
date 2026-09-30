package parser_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"

	"github.com/bufbuild/protoc-gen-jev/internal/parser"
)

func compileProto(t *testing.T, protoSrc string) *protogen.Plugin {
	t.Helper()

	tmpDir := t.TempDir()
	protoFile := filepath.Join(tmpDir, "test.proto")
	descFile := filepath.Join(tmpDir, "test.bin")

	err := os.WriteFile(protoFile, []byte(protoSrc), 0600)
	require.NoError(t, err)

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)

	// Copy jev/v1/options.proto so buf can resolve it
	jevDir := filepath.Join(tmpDir, "jev", "v1")
	err = os.MkdirAll(jevDir, 0755)
	require.NoError(t, err)

	optionsBytes, err := os.ReadFile(filepath.Join(repoRoot, "proto", "jev", "v1", "options.proto"))
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(jevDir, "options.proto"), optionsBytes, 0600)
	require.NoError(t, err)

	responseBytes, err := os.ReadFile(filepath.Join(repoRoot, "proto", "jev", "v1", "response.proto"))
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(jevDir, "response.proto"), responseBytes, 0600)
	require.NoError(t, err)

	bufYaml := `version: v2
modules:
  - path: .
`
	err = os.WriteFile(filepath.Join(tmpDir, "buf.yaml"), []byte(bufYaml), 0600)
	require.NoError(t, err)

	cmd := exec.Command("buf", "build", tmpDir, "-o", descFile)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "buf build failed: %s", string(out))

	data, err := os.ReadFile(descFile)
	require.NoError(t, err)

	var set descriptorpb.FileDescriptorSet
	err = proto.Unmarshal(data, &set)
	require.NoError(t, err)

	req := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{"test.proto"},
		ProtoFile:      set.File,
	}

	gen, err := protogen.Options{}.New(req)
	require.NoError(t, err)

	return gen
}

func TestSanity_EmptyRequest(t *testing.T) {
	src := `syntax = "proto3";
package test.v1;
option go_package = "test/v1;testv1";
import "jev/v1/options.proto";

message EmptyReq {}
message TriageResp {
  bool flag = 1 [(jev.v1.field).noul = {}];
}
service TestService {
  rpc DoTest(EmptyReq) returns (TriageResp);
}`

	gen := compileProto(t, src)
	file := gen.FilesByPath["test.proto"]
	require.NotNil(t, file)
	require.Len(t, file.Services, 1)

	_, err := parser.ProcessService(file.Services[0])
	require.Error(t, err)
	require.Contains(t, err.Error(), "must have at least one input field")
}

func TestSanity_EmptyResponseDecisions(t *testing.T) {
	src := `syntax = "proto3";
package test.v1;
option go_package = "test/v1;testv1";
import "jev/v1/options.proto";

message Req {
  string id = 1;
}
message EmptyResp {
  string text_only = 1; // Not a decision primitive
}
service TestService {
  option (jev.v1.service).enabled = true;
  rpc DoTest(Req) returns (EmptyResp);
}`

	gen := compileProto(t, src)
	file := gen.FilesByPath["test.proto"]
	require.NotNil(t, file)
	require.Len(t, file.Services, 1)

	_, err := parser.ProcessService(file.Services[0])
	require.Error(t, err)
	require.Contains(t, err.Error(), "must contain at least one decision field")
}

func TestSanity_MultiplePrimitivesError(t *testing.T) {
	src := `syntax = "proto3";
package test.v1;
option go_package = "test/v1;testv1";
import "jev/v1/options.proto";

message BadField {
  int32 mixed = 1 [
    (jev.v1.field).score = { levels: [{value: 1, description: "A"}, {value: 2, description: "B"}] },
    (jev.v1.field).choice = { choices: ["A", "B"] }
  ];
}`

	gen := compileProto(t, src)
	file := gen.FilesByPath["test.proto"]
	require.NotNil(t, file)
	require.Len(t, file.Messages, 1)

	_, err := parser.ProcessMessage(file.Messages[0])
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot specify more than one Jev primitive")
}

func TestSanity_ScoreTooFewLevels(t *testing.T) {
	src := `syntax = "proto3";
package test.v1;
option go_package = "test/v1;testv1";
import "jev/v1/options.proto";

message BadScore {
  int32 val = 1 [
    (jev.v1.field).score = {
      levels: [
        { value: 1, description: "OnlyOne" }
      ]
    }
  ];
}`

	gen := compileProto(t, src)
	file := gen.FilesByPath["test.proto"]
	require.NotNil(t, file)
	require.Len(t, file.Messages, 1)

	_, err := parser.ProcessMessage(file.Messages[0])
	require.Error(t, err)
	require.Contains(t, err.Error(), "score question must specify at least 2 levels")
}

func TestSanity_ChoiceNoChoices(t *testing.T) {
	src := `syntax = "proto3";
package test.v1;
option go_package = "test/v1;testv1";
import "jev/v1/options.proto";

message BadChoice {
  string val = 1 [
    (jev.v1.field).choice = {}
  ];
}`

	gen := compileProto(t, src)
	file := gen.FilesByPath["test.proto"]
	require.NotNil(t, file)
	require.Len(t, file.Messages, 1)

	_, err := parser.ProcessMessage(file.Messages[0])
	require.Error(t, err)
	require.Contains(t, err.Error(), "choice question has no valid choices")
}

func TestSanity_NonJevServiceSkipped(t *testing.T) {
	src := `syntax = "proto3";
package test.v1;
option go_package = "test/v1;testv1";

message LoginReq {
  string username = 1;
}
message LoginResp {
  string token = 1;
}
service AuthService {
  rpc Login(LoginReq) returns (LoginResp);
}`

	gen := compileProto(t, src)
	file := gen.FilesByPath["test.proto"]
	require.NotNil(t, file)
	require.Len(t, file.Services, 1)

	spec, err := parser.ProcessService(file.Services[0])
	require.NoError(t, err)
	require.Empty(t, spec.Methods, "Non-Jev service should have 0 methods processed")
}

func TestSanity_ExplicitDisableOverridesAutoDiscovery(t *testing.T) {
	src := `syntax = "proto3";
package test.v1;
option go_package = "test/v1;testv1";
import "jev/v1/options.proto";

message Req {
  string id = 1;
}
message Resp {
  bool flag = 1 [(jev.v1.field).noul = {}];
}
service MixedService {
  rpc JevMethod(Req) returns (Resp);
  rpc NonJevMethod(Req) returns (Resp) {
    option (jev.v1.method).enabled = false;
  }
}`

	gen := compileProto(t, src)
	file := gen.FilesByPath["test.proto"]
	require.NotNil(t, file)
	require.Len(t, file.Services, 1)

	spec, err := parser.ProcessService(file.Services[0])
	require.NoError(t, err)
	require.Len(t, spec.Methods, 1)
	require.Equal(t, "JevMethod", spec.Methods[0].Name)
}

func TestSanity_ServiceExplicitDisable(t *testing.T) {
	src := `syntax = "proto3";
package test.v1;
option go_package = "test/v1;testv1";
import "jev/v1/options.proto";

message Req {
  string id = 1;
}
message Resp {
  bool flag = 1 [(jev.v1.field).noul = {}];
}
service DisabledService {
  option (jev.v1.service).enabled = false;
  rpc Method1(Req) returns (Resp);
}
`

	gen := compileProto(t, src)
	file := gen.FilesByPath["test.proto"]
	require.NotNil(t, file)
	require.Len(t, file.Services, 1)

	spec, err := parser.ProcessService(file.Services[0])
	require.NoError(t, err)
	require.Empty(t, spec.Methods)
}

func TestSanity_UnsupportedRepeatedField(t *testing.T) {
	src := `syntax = "proto3";
package test.v1;
option go_package = "test/v1;testv1";
import "jev/v1/options.proto";

message BadMessage {
  repeated string tags = 1 [(jev.v1.field).instructions = "tag list"];
}
`

	gen := compileProto(t, src)
	file := gen.FilesByPath["test.proto"]
	require.NotNil(t, file)
	require.Len(t, file.Messages, 1)

	_, err := parser.ProcessMessage(file.Messages[0])
	require.Error(t, err)
	require.Contains(t, err.Error(), "repeated fields are not yet supported")
}

func TestSanity_UnsupportedMapField(t *testing.T) {
	src := `syntax = "proto3";
package test.v1;
option go_package = "test/v1;testv1";
import "jev/v1/options.proto";

message BadMessage {
  map<string, string> metadata = 1 [(jev.v1.field).instructions = "meta map"];
}
`

	gen := compileProto(t, src)
	file := gen.FilesByPath["test.proto"]
	require.NotNil(t, file)
	require.Len(t, file.Messages, 1)

	_, err := parser.ProcessMessage(file.Messages[0])
	require.Error(t, err)
	require.Contains(t, err.Error(), "map fields are not yet supported")
}

func TestSanity_UnsupportedNestedMessage(t *testing.T) {
	src := `syntax = "proto3";
package test.v1;
option go_package = "test/v1;testv1";
import "jev/v1/options.proto";

message SubMessage {
  string id = 1;
}

message BadMessage {
  SubMessage sub = 1 [(jev.v1.field).instructions = "sub msg"];
}
`

	gen := compileProto(t, src)
	file := gen.FilesByPath["test.proto"]
	require.NotNil(t, file)
	require.Len(t, file.Messages, 2)

	_, err := parser.ProcessMessage(file.Messages[1])
	require.Error(t, err)
	require.Contains(t, err.Error(), "nested message fields are not yet supported")
}

func TestSanity_UnsupportedBytesField(t *testing.T) {
	src := `syntax = "proto3";
package test.v1;
option go_package = "test/v1;testv1";
import "jev/v1/options.proto";

message BadMessage {
  bytes payload = 1 [(jev.v1.field).instructions = "raw bytes"];
}
`

	gen := compileProto(t, src)
	file := gen.FilesByPath["test.proto"]
	require.NotNil(t, file)
	require.Len(t, file.Messages, 1)

	_, err := parser.ProcessMessage(file.Messages[0])
	require.Error(t, err)
	require.Contains(t, err.Error(), "bytes fields are not yet supported")
}

func TestSanity_MultipleResponseFieldsError(t *testing.T) {
	src := `syntax = "proto3";
package test.v1;
option go_package = "test/v1;testv1";
import "jev/v1/options.proto";
import "jev/v1/response.proto";

message BadResponse {
  bool flag = 1 [(jev.v1.field).noul = {}];
  jev.v1.Response meta1 = 2;
  jev.v1.Response meta2 = 3;
}
`

	gen := compileProto(t, src)
	file := gen.FilesByPath["test.proto"]
	require.NotNil(t, file)
	require.Len(t, file.Messages, 1)

	_, err := parser.ProcessMessage(file.Messages[0])
	require.Error(t, err)
	require.Contains(t, err.Error(), "has multiple jev.v1.Response fields")
}

func TestSanity_RepeatedResponseFieldError(t *testing.T) {
	src := `syntax = "proto3";
package test.v1;
option go_package = "test/v1;testv1";
import "jev/v1/options.proto";
import "jev/v1/response.proto";

message BadResponse {
  bool flag = 1 [(jev.v1.field).noul = {}];
  repeated jev.v1.Response meta = 2;
}
`

	gen := compileProto(t, src)
	file := gen.FilesByPath["test.proto"]
	require.NotNil(t, file)
	require.Len(t, file.Messages, 1)

	_, err := parser.ProcessMessage(file.Messages[0])
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot be repeated")
}

func TestSanity_ValidResponseField(t *testing.T) {
	src := `syntax = "proto3";
package test.v1;
option go_package = "test/v1;testv1";
import "jev/v1/options.proto";
import "jev/v1/response.proto";

message GoodResponse {
  bool flag = 1 [(jev.v1.field).noul = {}];
  jev.v1.Response meta = 2;
}
`

	gen := compileProto(t, src)
	file := gen.FilesByPath["test.proto"]
	require.NotNil(t, file)
	require.Len(t, file.Messages, 1)

	spec, err := parser.ProcessMessage(file.Messages[0])
	require.NoError(t, err)
	require.Equal(t, "meta", spec.ResponseField)
	require.Len(t, spec.Questions, 1)
	require.Contains(t, spec.Questions, "flag")
}

func TestSanity_MultipleMetaFieldsError(t *testing.T) {
	src := `syntax = "proto3";
package test.v1;
option go_package = "test/v1;testv1";
import "jev/v1/options.proto";
import "jev/v1/response.proto";

message BadMeta {
  bool flag = 1 [(jev.v1.field).noul = {}];
  jev.v1.Meta meta1 = 2;
  jev.v1.Meta meta2 = 3;
}
`

	gen := compileProto(t, src)
	file := gen.FilesByPath["test.proto"]
	require.NotNil(t, file)
	require.Len(t, file.Messages, 1)

	_, err := parser.ProcessMessage(file.Messages[0])
	require.Error(t, err)
	require.Contains(t, err.Error(), "has multiple jev.v1.Meta fields")
}

func TestSanity_RepeatedMetaFieldError(t *testing.T) {
	src := `syntax = "proto3";
package test.v1;
option go_package = "test/v1;testv1";
import "jev/v1/options.proto";
import "jev/v1/response.proto";

message BadMeta {
  bool flag = 1 [(jev.v1.field).noul = {}];
  repeated jev.v1.Meta meta = 2;
}
`

	gen := compileProto(t, src)
	file := gen.FilesByPath["test.proto"]
	require.NotNil(t, file)
	require.Len(t, file.Messages, 1)

	_, err := parser.ProcessMessage(file.Messages[0])
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot be repeated")
}

func TestSanity_ValidMetaField(t *testing.T) {
	src := `syntax = "proto3";
package test.v1;
option go_package = "test/v1;testv1";
import "jev/v1/options.proto";
import "jev/v1/response.proto";

message GoodMeta {
  bool flag = 1 [(jev.v1.field).noul = {}];
  jev.v1.Meta meta = 2;
}
`

	gen := compileProto(t, src)
	file := gen.FilesByPath["test.proto"]
	require.NotNil(t, file)
	require.Len(t, file.Messages, 1)

	spec, err := parser.ProcessMessage(file.Messages[0])
	require.NoError(t, err)
	require.Equal(t, "meta", spec.MetaField)
	require.Len(t, spec.Questions, 1)
	require.Contains(t, spec.Questions, "flag")
}

func TestSanity_TypedAnswersValid(t *testing.T) {
	src := `syntax = "proto3";
package test.v1;
option go_package = "test/v1;testv1";
import "jev/v1/options.proto";
import "jev/v1/response.proto";

message TypedAnswersResponse {
  jev.v1.Noul urgent = 1 [(jev.v1.field).noul = { threshold: 0.7 }];
  jev.v1.Score priority = 2 [(jev.v1.field).score = {
    levels: [
      { value: 1.0, description: "low" },
      { value: 5.0, description: "high" }
    ]
  }];
  jev.v1.Choice category = 3 [(jev.v1.field).choice = {
    choices: ["bug", "feature"]
  }];
  jev.v1.Meta meta = 4;
}
`

	gen := compileProto(t, src)
	file := gen.FilesByPath["test.proto"]
	require.NotNil(t, file)
	require.Len(t, file.Messages, 1)

	spec, err := parser.ProcessMessage(file.Messages[0])
	require.NoError(t, err)
	require.Equal(t, "meta", spec.MetaField)
	require.Len(t, spec.Questions, 3)

	qUrgent := spec.Questions["urgent"]
	require.True(t, qUrgent.IsTypedAnswer)
	require.InDelta(t, 0.7, qUrgent.Threshold, 1e-6)

	qPriority := spec.Questions["priority"]
	require.True(t, qPriority.IsTypedAnswer)
	require.Len(t, qPriority.ScoreLevels, 2)

	qCat := spec.Questions["category"]
	require.True(t, qCat.IsTypedAnswer)
	require.Len(t, qCat.Criteria, 2)
}

func TestSanity_TypedAnswersConflictingRulesError(t *testing.T) {
	src := `syntax = "proto3";
package test.v1;
option go_package = "test/v1;testv1";
import "jev/v1/options.proto";
import "jev/v1/response.proto";

message ConflictingTyped {
  jev.v1.Noul urgent = 1 [(jev.v1.field).score = {
    levels: [
      { value: 1.0, description: "low" },
      { value: 5.0, description: "high" }
    ]
  }];
}
`

	gen := compileProto(t, src)
	file := gen.FilesByPath["test.proto"]
	require.NotNil(t, file)
	require.Len(t, file.Messages, 1)

	_, err := parser.ProcessMessage(file.Messages[0])
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot be used with choice or score rules")
}

func TestSanity_TypedAnswersChoiceMissingRulesError(t *testing.T) {
	src := `syntax = "proto3";
package test.v1;
option go_package = "test/v1;testv1";
import "jev/v1/options.proto";
import "jev/v1/response.proto";

message MissingChoiceRules {
  jev.v1.Choice category = 1;
}
`

	gen := compileProto(t, src)
	file := gen.FilesByPath["test.proto"]
	require.NotNil(t, file)
	require.Len(t, file.Messages, 1)

	_, err := parser.ProcessMessage(file.Messages[0])
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires choice rules specifying choices")
}
