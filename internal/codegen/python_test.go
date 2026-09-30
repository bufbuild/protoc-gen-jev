package codegen

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPythonImport(t *testing.T) {
	for _, tc := range []struct{ source, target, module, name string }{
		{"service.proto", "contract.proto", ".", "contract_pb"},
		{"ai/contract/v1/service.proto", "ai/contract/v1/contract.proto", ".", "contract_pb"},
		{"ai/contract/v1/service.proto", "ai/shared/v1/shared.proto", "...shared.v1", "shared_pb"},
		{"ai/service.proto", "contract.proto", "..", "contract_pb"},
		{"service.proto", "ai/contract.proto", ".ai", "contract_pb"},
	} {
		t.Run(tc.source+"/"+tc.target, func(t *testing.T) {
			module, name, err := pythonImport(tc.source, tc.target)
			require.NoError(t, err)
			require.Equal(t, tc.module, module)
			require.Equal(t, tc.name, name)
		})
	}
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Triage", "triage"},
		{"TriageDetails", "triage_details"},
		{"Evaluate", "evaluate"},
		{"APIKey", "api_key"},
		{"HTTPUrl", "http_url"},
		{"URLPath", "url_path"},
		{"GetHTTPResponse", "get_http_response"},
		{"ID", "id"},
		{"", ""},
	}
	for _, tt := range tests {
		require.Equal(t, tt.expected, ToSnakeCase(tt.input))
	}
}
