package jevv1

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAnswerUnmarshalJSON(t *testing.T) {
	for _, tc := range []struct {
		name, input     string
		want            float64
		absent, invalid bool
	}{
		{name: "probability", input: `{"noul":0.85}`, want: 0.85},
		{name: "true with whitespace", input: "{\"noul\" : \n\t true}", want: 1},
		{name: "false", input: `{"noul":false}`},
		{name: "escaped field name", input: `{"no\u0075l":true}`, want: 1},
		{name: "zero", input: `{"noul":0}`},
		{name: "null", input: `{"noul":null}`, absent: true},
		{name: "missing", input: `{}`, absent: true},
		{name: "quoted probability", input: `{"noul":"0.85"}`, invalid: true},
		{name: "quoted bool", input: `{"noul":"true"}`, invalid: true},
		{name: "object", input: `{"noul":{}}`, invalid: true},
		{name: "array", input: `{"noul":[]}`, invalid: true},
		{name: "quoted score", input: `{"score":"0.5"}`, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Reuse must clear a previously present probability.
			answer := &Answer{}
			answer.SetNoul(0)
			err := json.Unmarshal([]byte(tc.input), answer)
			if tc.invalid {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tc.absent {
				require.False(t, answer.HasNoul())
				return
			}
			require.True(t, answer.HasNoul())
			require.InDelta(t, tc.want, answer.GetNoul(), 1e-9)
		})
	}
}

func TestWireResponseUnmarshalJSON(t *testing.T) {
	var response WireResponse
	require.NoError(t, json.Unmarshal([]byte(`{
  "model":"test", "usage":{"input_tokens":12,"output_tokens":4},
  "answers":{"page":{"type":"noul","noul":true,"confidence":0,
    "legend":{"0":"literal text: \"noul\": true"}}},
  "nouls":{"page":{"noul":false}}, "future_field":true
 }`), &response))
	require.Equal(t, int32(12), response.GetUsage().GetInputTokens())
	require.InDelta(t, 1.0, response.GetAnswers()["page"].GetNoul(), 1e-9)
	require.True(t, response.GetAnswers()["page"].HasConfidence())
	require.Equal(t, `literal text: "noul": true`, response.GetAnswers()["page"].GetLegend()["0"])
	require.InDelta(t, 0.0, response.GetNouls()["page"].GetNoul(), 1e-9)
	for _, input := range []string{`{"answers":null}`, `{"answers":{}}`} {
		require.NoError(t, json.Unmarshal([]byte(input), &response))
		require.NotNil(t, response.GetAnswers())
		require.Empty(t, response.GetAnswers())
		require.Nil(t, response.GetUsage())
	}
	require.NoError(t, json.Unmarshal([]byte(`{}`), &response))
	require.Nil(t, response.GetAnswers())
}
