package model

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
)

// QuestionType represents Jev's primitive question types.
type QuestionType string

const (
	TypeChoice QuestionType = "Choice"
	TypeNoul   QuestionType = "Noul"
	TypeScore  QuestionType = "Score"
)

// ScoreLevel represents a single rubric tier for Score questions.
type ScoreLevel struct {
	Value       float64 `json:"value"`
	Description string  `json:"description"`
}

// Question represents a single Jev decision question mapped from a Protobuf field.
type Question struct {
	Type           QuestionType      `json:"type"`
	Instructions   string            `json:"instructions"`
	Criteria       any               `json:"criteria,omitempty"`
	Threshold      float64           `json:"threshold,omitempty"`
	ScoreLevels    []ScoreLevel      `json:"score_levels,omitempty"`
	ProtoField     string            `json:"proto_field"`
	JSONField      string            `json:"json_field"`
	GoField        string            `json:"go_field"`
	PyField        string            `json:"py_field"`
	FieldType      string            `json:"field_type"` // "string", "bool", "int32", "int64", "float32", "float64", "enum", "oneof"
	IsEnum         bool              `json:"is_enum,omitempty"`
	EnumTypeName   string            `json:"enum_type_name,omitempty"`
	IsOneof        bool              `json:"is_oneof,omitempty"`
	OneofName      string            `json:"oneof_name,omitempty"`
	OneofCases     map[string]string `json:"oneof_cases,omitempty"`      // proto_field -> Go struct suffix / TS case
	OneofCaseKinds map[string]string `json:"oneof_case_kinds,omitempty"` // proto_field -> proto kind (e.g. "bytes", "string")
	IsTypedAnswer  bool              `json:"is_typed_answer,omitempty"`
}

// MessageSpec represents all Jev questions extracted from a single Protobuf message.
type MessageSpec struct {
	Ref           *protogen.Message   `json:"-"`
	MessageName   string              `json:"message_name"`
	Package       string              `json:"package"`
	FileBase      string              `json:"file_base,omitempty"`
	Questions     map[string]Question `json:"questions"`
	Order         []string            `json:"order,omitempty"`
	ResponseField string              `json:"response_field,omitempty"`
	MetaField     string              `json:"meta_field,omitempty"`
}

// FieldSpec describes a field in an input request message.
type FieldSpec struct {
	Name     string `json:"name"`
	JSONName string `json:"json_name"`
	GoName   string `json:"go_name"`
	Type     string `json:"type"`
}

// MethodSpec represents an RPC method on a Service.
type MethodSpec struct {
	Input        *protogen.Message `json:"-"`
	Name         string            `json:"name"`
	InputType    string            `json:"input_type"`
	InputFields  []FieldSpec       `json:"input_fields"`
	OutputType   string            `json:"output_type"`
	QuestionSpec MessageSpec       `json:"question_spec"`
}

// ServiceSpec represents a Protobuf service containing Jev RPC methods.
type ServiceSpec struct {
	ServiceName string       `json:"service_name"`
	Package     string       `json:"package"`
	FileBase    string       `json:"file_base,omitempty"`
	Methods     []MethodSpec `json:"methods"`
}

// TargetOptions controls which code generation targets to produce.
type TargetOptions struct {
	GenerateJSON       bool
	GenerateGo         bool
	GenerateTypeScript bool
	GeneratePython     bool
}

// ParseTargets parses comma-separated target flags like "go,ts,python,json" or "all".
func ParseTargets(targetOpt string) (TargetOptions, error) {
	if targetOpt == "all" || targetOpt == "" {
		return TargetOptions{
			GenerateJSON:       true,
			GenerateGo:         true,
			GenerateTypeScript: true,
			GeneratePython:     true,
		}, nil
	}

	var opts TargetOptions
	opts.GenerateJSON = true // always generate declarative JSON spec

	for _, t := range strings.Split(targetOpt, ",") {
		t = strings.TrimSpace(t)
		switch t {
		case "go":
			opts.GenerateGo = true
		case "ts", "typescript":
			opts.GenerateTypeScript = true
		case "py", "python":
			opts.GeneratePython = true
		case "json":
			opts.GenerateJSON = true
		case "all":
			opts.GenerateGo = true
			opts.GenerateTypeScript = true
			opts.GeneratePython = true
			opts.GenerateJSON = true
		default:
			return opts, fmt.Errorf("unknown target %q (expected go, ts, python, json, or all)", t)
		}
	}
	return opts, nil
}
