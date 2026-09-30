package client_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	contract "github.com/bufbuild/protoc-gen-jev/gen/jev/ai/contract/v1"
	shared "github.com/bufbuild/protoc-gen-jev/gen/jev/ai/shared/v1"
	"github.com/bufbuild/protoc-gen-jev/pkg/jev"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type responseCase struct {
	Name     string          `json:"name"`
	Response json.RawMessage `json:"response"`
	Expected json.RawMessage `json:"expected"`
	Error    bool            `json:"error"`
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCrossLanguageContract(t *testing.T) {
	data, err := os.ReadFile("../../testdata/behavior/responses.json")
	require.NoError(t, err)
	var cases []responseCase
	require.NoError(t, json.Unmarshal(data, &cases))
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			httpClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, "Bearer test-key", req.Header.Get("Authorization"))
				var payload map[string]json.RawMessage
				require.NoError(t, json.NewDecoder(req.Body).Decode(&payload))
				require.JSONEq(t, `{"inputText":"hello","inputCount":"5"}`, string(payload["state"]))
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(tc.Response)))}, nil
			})}
			jevClient := jev.NewClient("test-key")
			jevClient.HTTPClient = httpClient
			client := contract.NewJevContractService(jevClient)
			result, err := client.Evaluate(context.Background(), &contract.EvaluateRequest{InputText: "hello", InputCount: 5})
			if tc.Error {
				require.Error(t, err)
				require.Nil(t, result)
				return
			}
			require.NoError(t, err)
			expected := &contract.EvaluateResponse{}
			require.NoError(t, protojson.Unmarshal(tc.Expected, expected))
			require.True(t, proto.Equal(expected, result), "expected %s, got %s", expected, result)
			require.NotNil(t, result.Flag)
			require.NotNil(t, result.Rating)
		})
	}
}

func TestImportedAndNestedRPCs(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"answers":{"accepted":{"type":"noul","noul":0.8}}}`))}, nil
	})}
	jevClient := jev.NewClient("test")
	jevClient.HTTPClient = httpClient
	client := contract.NewJevContractService(jevClient)
	external, err := client.External(context.Background(), &shared.ExternalRequest{InputText: "hello"})
	require.NoError(t, err)
	require.True(t, external.Accepted)
	nested, err := client.Nested(context.Background(), &contract.Container_NestedRequest{InputText: "hello"})
	require.NoError(t, err)
	require.NotNil(t, nested.Accepted)
	require.True(t, nested.GetAccepted())
}
