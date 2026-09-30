package parser_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/sudorandom/protoc-gen-jev/internal/model"
	"github.com/sudorandom/protoc-gen-jev/internal/parser"
)

func TestInvalidDecisionDefinitions(t *testing.T) {
	cases := map[string]string{
		"choice on int":      `int32 value = 1 [(jev.v1.field).choice = {choices:["A","B"]}];`,
		"noul on string":     `string value = 1 [(jev.v1.field).noul = {}];`,
		"score on bool":      `bool value = 1 [(jev.v1.field).score = {levels:[{value:1},{value:2}]}];`,
		"negative threshold": `bool value = 1 [(jev.v1.field).noul = {threshold:-1}];`,
		"large threshold":    `bool value = 1 [(jev.v1.field).noul = {threshold:2}];`,
		"nan threshold":      `bool value = 1 [(jev.v1.field).noul = {threshold:nan}];`,
		"unordered rubric":   `int32 value = 1 [(jev.v1.field).score = {levels:[{value:2},{value:1}]}];`,
		"duplicate levels":   `int32 value = 1 [(jev.v1.field).score = {levels:[{value:1},{value:1}]}];`,
		"nonfinite level":    `float value = 1 [(jev.v1.field).score = {levels:[{value:0},{value:inf}]}];`,
		"overflow":           `int32 value = 1 [(jev.v1.field).score = {levels:[{value:0},{value:2147483648}]}];`,
		"negative unsigned":  `uint32 value = 1 [(jev.v1.field).score = {levels:[{value:-1},{value:1}]}];`,
		"unsafe integer":     `int64 value = 1 [(jev.v1.field).score = {levels:[{value:0},{value:9007199254740992}]}];`,
		"fractional integer": `int32 value = 1 [(jev.v1.field).score = {levels:[{value:0},{value:1.5}]}];`,
		"numeric oneof":      `oneof selected { int32 count = 1; string label = 2; }`,
		"unknown enum":       `E value = 1 [(jev.v1.field).choice = {criteria:{key:"TYPO",value:"bad"}}];`,
		"excluded enum":      `E value = 1 [(jev.v1.field).choice = {not_in:["E_A"],criteria:{key:"E_A",value:"bad"}}];`,
		"outside whitelist":  `string value = 1 [(jev.v1.field).choice = {choices:["A"],criteria:{key:"B",value:"bad"}}];`,
	}
	var levels, choices []string
	for i := range 11 {
		levels = append(levels, fmt.Sprintf("{value:%d}", i))
	}
	cases["too many levels"] = `float value = 1 [(jev.v1.field).score = {levels:[` + strings.Join(levels, ",") + `]}];`
	for i := range 256 {
		choices = append(choices, fmt.Sprintf("%q", fmt.Sprintf("C%d", i)))
	}
	cases["too many choices"] = `string value = 1 [(jev.v1.field).choice = {choices:[` + strings.Join(choices, ",") + `]}];`
	for name, fields := range cases {
		t.Run(name, func(t *testing.T) {
			src := `syntax="proto3"; package test.v1; option go_package="test/v1;testv1"; import "jev/v1/options.proto"; enum E { E_UNSPECIFIED=0; E_A=1; E_B=2; } message Test {` + fields + `}`
			gen := compileProto(t, src)
			_, err := parser.ProcessMessage(gen.FilesByPath["test.proto"].Messages[0])
			require.Error(t, err)
		})
	}
}

func TestZeroThresholdAndOptionalFields(t *testing.T) {
	gen := compileProto(t, `syntax="proto3"; package test.v1; option go_package="test/v1;testv1"; import "jev/v1/options.proto"; message Test { optional bool flag=1 [(jev.v1.field).noul={threshold:0}]; uint32 count=2; fixed64 fixed=3; }`)
	spec, err := parser.ProcessMessage(gen.FilesByPath["test.proto"].Messages[0])
	require.NoError(t, err)
	require.Zero(t, spec.Questions["flag"].Threshold)
	require.Equal(t, "uint32", spec.Questions["count"].FieldType)
	require.Equal(t, "uint64", spec.Questions["fixed"].FieldType)
}

func TestUnknownTargets(t *testing.T) {
	for _, s := range []string{"golang", "go,typo", ",go", "go,"} {
		_, err := model.ParseTargets(s)
		require.Error(t, err)
	}
}

func TestQuestionNameCollision(t *testing.T) {
	for _, fields := range []string{
		`oneof route { string email=1; string sms=2; } bool flag=3 [json_name="route"];`,
		`bool flag=3 [json_name="route"]; oneof route { string email=1; string sms=2; }`,
	} {
		gen := compileProto(t, `syntax="proto3"; package test.v1; option go_package="test/v1;testv1"; message Test {`+fields+`}`)
		_, err := parser.ProcessMessage(gen.FilesByPath["test.proto"].Messages[0])
		require.ErrorContains(t, err, `duplicate question name "route"`)
	}
}

func TestOneofInstructions(t *testing.T) {
	gen := compileProto(t, `syntax="proto3"; package test.v1; option go_package="test/v1;testv1";
import "jev/v1/options.proto";
message Test {
  oneof route {
    // This comment must be overridden.
    string email=1 [(jev.v1.field).instructions="Route here for billing issues"];
    // Use SMS for urgent issues.
    string sms=2;
    bytes binary=3;
    string skipped=4 [(jev.v1.field).skip=true, (jev.v1.field).instructions="Omit this route"];
  }
}`)
	spec, err := parser.ProcessMessage(gen.FilesByPath["test.proto"].Messages[0])
	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"email":  "Route here for billing issues",
		"sms":    "Use SMS for urgent issues.",
		"binary": "binary",
	}, spec.Questions["route"].Criteria)
}

func TestEdition2024ExplicitPresence(t *testing.T) {
	gen := compileProto(t, `edition="2024"; package test.v1;
 option go_package="test/v1;testv1";
 import "jev/v1/options.proto";
 message Test {
  bool flag=1 [(jev.v1.field).noul={threshold:0}];
  oneof route { string email=2; string sms=3; }
 }`)
	message := gen.FilesByPath["test.proto"].Messages[0]
	require.True(t, message.Fields[0].Desc.HasPresence())
	require.Nil(t, message.Fields[0].Oneof)
	spec, err := parser.ProcessMessage(message)
	require.NoError(t, err)
	require.Len(t, spec.Questions, 2)
	require.Equal(t, model.TypeNoul, spec.Questions["flag"].Type)
	require.Zero(t, spec.Questions["flag"].Threshold)
	require.True(t, spec.Questions["route"].IsOneof)
}
