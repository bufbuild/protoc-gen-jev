package jev

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"

	jevv1 "github.com/sudorandom/protoc-gen-jev/pkg/jev/v1"
)

func TestDecode_Choice(t *testing.T) {
	rules := []Question{
		{
			Name:    "status",
			Type:    "choice",
			Field:   "status",
			Choices: map[string]string{"OPEN": "Open item", "CLOSED": "Closed item"},
		},
	}

	t.Run("valid choice", func(t *testing.T) {
		body := []byte(`{
			"model": "jev-latest",
			"usage": { "input_tokens": 12, "output_tokens": 4 },
			"answers": {
				"status": { "type": "choice", "choice": "OPEN" }
			}
		}`)
		out, err := Decode(body, rules)
		require.NoError(t, err)
		assert.Equal(t, "OPEN", out.Choices["status"].GetValue())
	})

	t.Run("unknown choice rejected", func(t *testing.T) {
		body := []byte(`{
			"answers": {
				"status": { "type": "choice", "choice": "INVALID_CHOICE" }
			}
		}`)
		_, err := Decode(body, rules)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown choice")
	})
}

func TestDecode_Oneof(t *testing.T) {
	rules := []Question{
		{
			Name:    "route",
			Type:    "choice",
			Kind:    "oneof",
			Choices: map[string]string{"email": "", "sms": ""},
			Oneof:   map[string]string{"email": "string", "sms": "bytes"},
		},
	}

	t.Run("string oneof", func(t *testing.T) {
		body := []byte(`{"answers":{"route":{"type":"choice","choice":"email"}}}`)
		out, err := Decode(body, rules)
		require.NoError(t, err)
		assert.Equal(t, "email", out.Choices["route"].GetValue())
	})

	t.Run("bytes oneof label", func(t *testing.T) {
		body := []byte(`{"answers":{"route":{"type":"choice","choice":"sms"}}}`)
		out, err := Decode(body, rules)
		require.NoError(t, err)
		// Generated code converts the selected label to bytes.
		assert.Equal(t, "sms", out.Choices["route"].GetValue())
	})
}

func TestDecode_Noul(t *testing.T) {
	rules := []Question{
		{
			Name:      "paged",
			Type:      "noul",
			Field:     "paged",
			Threshold: 0.8,
		},
	}

	t.Run("bool true", func(t *testing.T) {
		body := []byte(`{"answers":{"paged":{"type":"noul","noul":true}}}`)
		out, err := Decode(body, rules)
		require.NoError(t, err)
		assert.True(t, out.Nouls["paged"].GetValue())
	})

	t.Run("float above threshold", func(t *testing.T) {
		body := []byte(`{"answers":{"paged":{"type":"noul","noul":0.85}}}`)
		out, err := Decode(body, rules)
		require.NoError(t, err)
		assert.True(t, out.Nouls["paged"].GetValue())
	})

	t.Run("float below threshold", func(t *testing.T) {
		body := []byte(`{"answers":{"paged":{"type":"noul","noul":0.75}}}`)
		out, err := Decode(body, rules)
		require.NoError(t, err)
		assert.False(t, out.Nouls["paged"].GetValue())
	})

	t.Run("invalid probability out of bounds", func(t *testing.T) {
		body := []byte(`{"answers":{"paged":{"type":"noul","noul":1.5}}}`)
		_, err := Decode(body, rules)
		require.Error(t, err)
	})
}

func TestDecode_Score(t *testing.T) {
	rules := []Question{
		{
			Name:  "rating",
			Type:  "score",
			Field: "rating",
			Kind:  "int32",
			Levels: []Level{
				{Value: 1, Description: "One"},
				{Value: 3, Description: "Three"},
				{Value: 5, Description: "Five"},
			},
		},
	}

	t.Run("exact index", func(t *testing.T) {
		body := []byte(`{"answers":{"rating":{"type":"score","score":0.0}}}`)
		out, err := Decode(body, rules)
		require.NoError(t, err)
		assert.InDelta(t, 1.0, out.Scores["rating"].GetValue(), 1e-9)
	})

	t.Run("interpolated and rounded", func(t *testing.T) {
		body := []byte(`{"answers":{"rating":{"type":"score","score":0.6}}}`)
		out, err := Decode(body, rules)
		require.NoError(t, err)
		assert.InDelta(t, 2.0, out.Scores["rating"].GetValue(), 1e-9)
	})

	t.Run("score position out of bounds", func(t *testing.T) {
		body := []byte(`{"answers":{"rating":{"type":"score","score":2.5}}}`)
		_, err := Decode(body, rules)
		require.Error(t, err)
	})
}

func TestDecode_ResponseField(t *testing.T) {
	rules := []Question{
		{Name: "status", Type: "choice", Field: "status", Choices: map[string]string{"OPEN": ""}},
	}
	body := []byte(`{
		"model": "jev-latest",
		"usage": { "input_tokens": 12, "output_tokens": 4 },
		"answers": {
			"status": { "type": "choice", "choice": "OPEN" }
		}
	}`)
	var resp jevv1.Response
	err := protojson.Unmarshal(body, &resp)
	require.NoError(t, err)
	assert.Equal(t, "jev-latest", resp.GetModel())
	assert.Equal(t, int32(12), resp.GetUsage().GetInputTokens())
	assert.Equal(t, int32(4), resp.GetUsage().GetOutputTokens())
	require.Contains(t, resp.GetAnswers(), "status")
	assert.Equal(t, "OPEN", resp.GetAnswers()["status"].GetChoice())

	out, err := Decode(body, rules)
	require.NoError(t, err)
	assert.Equal(t, "OPEN", out.Choices["status"].GetValue())
}

func TestDecode_MetaField(t *testing.T) {
	rules := []Question{
		{Name: "status", Type: "choice", Field: "status", Choices: map[string]string{"OPEN": ""}},
	}
	body := []byte(`{
		"model": "test-model",
		"usage": { "input_tokens": 10, "output_tokens": 5 },
		"answers": {
			"status": { "type": "choice", "choice": "OPEN" }
		}
	}`)
	out, err := Decode(body, rules)
	require.NoError(t, err)
	meta := out.Meta
	require.NotNil(t, meta)
	assert.Equal(t, "test-model", meta.GetModel())
	usage := meta.GetUsage()
	require.NotNil(t, usage)
	assert.InDelta(t, 10.0, usage.GetInputTokens(), 1e-9)
	assert.InDelta(t, 5.0, usage.GetOutputTokens(), 1e-9)
}

func TestDecode_TypedAnswers(t *testing.T) {
	rules := []Question{
		{Name: "flag", Type: "noul", Field: "flag", Threshold: 0.8, Typed: true},
		{Name: "urgency", Type: "score", Field: "urgency", Kind: "float64", Levels: []Level{{Value: 1}, {Value: 5}}, Typed: true},
		{Name: "route", Type: "choice", Field: "route", Choices: map[string]string{"A": ""}, Typed: true},
	}
	body := []byte(`{
		"answers": {
			"flag": { "type": "noul", "noul": 0.85, "confidence": 0.95 },
			"urgency": { "type": "score", "score": 0.5, "confidence": 0.88, "probabilities": { "1": 0.5, "5": 0.5 } },
			"route": { "type": "choice", "choice": "A", "confidence": 0.92 }
		}
	}`)
	out, err := Decode(body, rules)
	require.NoError(t, err)

	flag := out.Nouls["flag"]
	require.NotNil(t, flag)
	assert.True(t, flag.GetValue())
	assert.InDelta(t, 0.85, flag.GetProbability(), 1e-9)
	assert.InDelta(t, 0.95, flag.GetConfidence(), 1e-9)

	urgency := out.Scores["urgency"]
	require.NotNil(t, urgency)
	assert.InDelta(t, 3.0, urgency.GetValue(), 1e-9)
	assert.InDelta(t, 0.5, urgency.GetScore(), 1e-9)
	assert.InDelta(t, 0.88, urgency.GetConfidence(), 1e-9)

	route := out.Choices["route"]
	require.NotNil(t, route)
	assert.Equal(t, "A", route.GetValue())
	assert.InDelta(t, 0.92, route.GetConfidence(), 1e-9)
}

func TestDecode_LegacyGroupFallback(t *testing.T) {
	rules := []Question{
		{
			Name:    "cat",
			Type:    "choice",
			Field:   "cat",
			Choices: map[string]string{"A": "A", "B": "B"},
		},
	}
	body := []byte(`{"choices":{"cat":{"choice":"B"}}}`)
	out, err := Decode(body, rules)
	require.NoError(t, err)
	assert.Equal(t, "B", out.Choices["cat"].GetValue())
}

func TestDecode_MissingAnswer(t *testing.T) {
	rules := []Question{
		{Name: "q1", Type: "choice", Field: "q1", Choices: map[string]string{"A": "A"}},
	}
	body := []byte(`{"answers":{}}`)
	_, err := Decode(body, rules)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing or invalid answer")
}

func TestDecode_InvalidJSON(t *testing.T) {
	rules := []Question{{Name: "q", Type: "noul", Field: "q"}}
	_, err := Decode([]byte(`not json`), rules)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode Jev response")
}
