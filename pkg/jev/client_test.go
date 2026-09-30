package jev

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestNewClient_Defaults(t *testing.T) {
	c := NewClient("secret-key")
	assert.Equal(t, "secret-key", c.APIKey)
	assert.Equal(t, "https://api.typesafe.ai", c.BaseURL)
	assert.Equal(t, "jev-latest", c.Model)
	require.NotNil(t, c.HTTPClient)
	assert.Equal(t, 30*time.Second, c.HTTPClient.Timeout)
}

func TestClient_EndpointURL(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		expected string
	}{
		{
			name:     "empty defaults to typesafe API with path",
			baseURL:  "",
			expected: "https://api.typesafe.ai/v1/systemone",
		},
		{
			name:     "standard base URL without path",
			baseURL:  "http://localhost:6660",
			expected: "http://localhost:6660/v1/systemone",
		},
		{
			name:     "base URL with trailing slash",
			baseURL:  "http://localhost:6660/",
			expected: "http://localhost:6660/v1/systemone",
		},
		{
			name:     "base URL already containing /v1/systemone",
			baseURL:  "http://localhost:6660/v1/systemone",
			expected: "http://localhost:6660/v1/systemone",
		},
		{
			name:     "base URL already containing /v1/systemone with trailing slash",
			baseURL:  "http://localhost:6660/v1/systemone/",
			expected: "http://localhost:6660/v1/systemone",
		},
		{
			name:     "custom domain with path prefix",
			baseURL:  "https://gateway.internal/ai",
			expected: "https://gateway.internal/ai/v1/systemone",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{BaseURL: tt.baseURL}
			assert.Equal(t, tt.expected, c.endpointURL())
		})
	}
}

func TestQuestions_BuildPayload(t *testing.T) {
	rules := []Question{
		{
			Name:         "target",
			Type:         "choice",
			Instructions: "Pick one",
			Choices: map[string]string{
				"option_a": "First option",
				"option_b": "Second option",
			},
		},
		{
			Name:         "urgency",
			Type:         "score",
			Instructions: "Rate urgency",
			Levels: []Level{
				{Value: 1, Description: "Low"},
				{Value: 2, Description: "Medium"},
				{Value: 3, Description: "High"},
			},
		},
		{
			Name:         "paging",
			Type:         "noul",
			Instructions: "Should page?",
			Threshold:    0.8,
		},
	}

	payload := Questions(rules)
	require.Len(t, payload, 3)

	choiceQ, ok := payload["target"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "choice", choiceQ["type"])
	assert.Equal(t, "Pick one", choiceQ["instructions"])
	assert.Equal(t, map[string]any{"option_a": "First option", "option_b": "Second option"}, choiceQ["criteria"])

	scoreQ, ok := payload["urgency"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "score", scoreQ["type"])
	assert.Equal(t, "Rate urgency", scoreQ["instructions"])
	assert.Equal(t, []string{"Low", "Medium", "High"}, scoreQ["criteria"])

	noulQ, ok := payload["paging"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "noul", noulQ["type"])
	assert.Equal(t, "Should page?", noulQ["instructions"])
	assert.Nil(t, noulQ["criteria"])
}

func TestClient_Evaluate_NilClient(t *testing.T) {
	var c *Client
	_, err := c.Evaluate(context.Background(), "state", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client is nil")
}

func TestClient_Evaluate_NilProtobufMessage(t *testing.T) {
	c := NewClient("test-key")
	var msg *structpb.Struct // typed nil
	_, err := c.Evaluate(context.Background(), msg, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil protobuf request")
}

func TestClient_Evaluate_HeadersAndModel(t *testing.T) {
	var capturedAuth string
	var capturedContentType string
	var capturedBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		capturedContentType = r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"answers":{}}`))
	}))
	defer server.Close()

	c := NewClient("secret-token")
	c.BaseURL = server.URL
	c.Model = "custom-model"
	c.HTTPClient = server.Client()

	_, err := c.Evaluate(context.Background(), map[string]any{"key": "value"}, nil)
	require.NoError(t, err)

	assert.Equal(t, "Bearer secret-token", capturedAuth)
	assert.Equal(t, "application/json", capturedContentType)
	assert.Equal(t, "custom-model", capturedBody["model"])
	assert.Equal(t, map[string]any{"key": "value"}, capturedBody["state"])
}

func TestClient_Evaluate_OmitAuthWhenKeyEmpty(t *testing.T) {
	var capturedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"answers":{}}`))
	}))
	defer server.Close()

	c := NewClient("")
	c.BaseURL = server.URL
	c.HTTPClient = server.Client()

	_, err := c.Evaluate(context.Background(), "state", nil)
	require.NoError(t, err)
	assert.Empty(t, capturedAuth)
}

func TestClient_Evaluate_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_api_key"}`))
	}))
	defer server.Close()

	c := NewClient("bad-key")
	c.BaseURL = server.URL
	c.HTTPClient = server.Client()

	_, err := c.Evaluate(context.Background(), "state", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
	assert.Contains(t, err.Error(), "invalid_api_key")
}

func TestClient_Evaluate_ResponseTooLarge(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Write slightly more than 16 MiB of zeros
		buf := make([]byte, 1024*1024)
		for i := 0; i < 17; i++ {
			_, _ = w.Write(buf)
		}
	}))
	defer server.Close()

	c := NewClient("key")
	c.BaseURL = server.URL
	c.HTTPClient = server.Client()

	_, err := c.Evaluate(context.Background(), "state", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds 16 MiB")
}

func TestBatch(t *testing.T) {
	ctx := context.Background()

	t.Run("empty slice", func(t *testing.T) {
		results, err := Batch(ctx, []int{}, func(ctx context.Context, item int) (int, error) {
			return item * 2, nil
		})
		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("success", func(t *testing.T) {
		items := []string{"apple", "banana", "cherry"}
		results, err := Batch(ctx, items, func(ctx context.Context, item string) (int, error) {
			return len(item), nil
		})
		require.NoError(t, err)
		assert.Equal(t, []int{5, 6, 6}, results)
	})

	t.Run("stops at first error", func(t *testing.T) {
		calls := 0
		items := []int{10, 0, 20}
		results, err := Batch(ctx, items, func(ctx context.Context, item int) (int, error) {
			calls++
			if item == 0 {
				return 0, io.ErrUnexpectedEOF
			}
			return 100 / item, nil
		})
		require.Error(t, err)
		assert.Nil(t, results)
		assert.Equal(t, 2, calls) // did not evaluate third item
		assert.Contains(t, err.Error(), "batch item 1: unexpected EOF")
	})
}
