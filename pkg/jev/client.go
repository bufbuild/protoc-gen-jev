// Package jev provides the transport and response mapping used by generated Go clients.
package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Level associates a score position with a domain value and a rubric description.
type Level struct {
	Value       float64 `json:"value"`
	Description string  `json:"description"`
}

// Question contains the wire question and its Protobuf response mapping.
type Question struct {
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Field        string            `json:"field"`
	Kind         string            `json:"kind"`
	Choices      map[string]string `json:"choices,omitempty"`
	Levels       []Level           `json:"levels,omitempty"`
	Threshold    float64           `json:"threshold"`
	Oneof        map[string]string `json:"oneof,omitempty"`
	Typed        bool              `json:"typed,omitempty"`
}

// Questions builds a fresh API payload from the question definitions.
func Questions(rules []Question) map[string]any {
	out := make(map[string]any, len(rules))
	for _, q := range rules {
		item := map[string]any{"type": q.Type, "instructions": q.Instructions}
		switch q.Type {
		case "choice":
			choices := make(map[string]any, len(q.Choices))
			for k, v := range q.Choices {
				choices[k] = v
			}
			item["criteria"] = choices
		case "score":
			levels := make([]string, len(q.Levels))
			for i, l := range q.Levels {
				levels[i] = l.Description
			}
			item["criteria"] = levels
		}
		out[q.Name] = item
	}
	return out
}

// Client executes decision evaluations against the Jev API.
type Client struct {
	APIKey     string
	BaseURL    string
	Model      string
	HTTPClient *http.Client
}

// NewClient creates a new Client with standard defaults.
func NewClient(apiKey string) *Client {
	return &Client{
		APIKey:     apiKey,
		BaseURL:    "https://api.typesafe.ai",
		Model:      "jev-latest",
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) endpointURL() string {
	base := c.BaseURL
	if base == "" {
		base = "https://api.typesafe.ai"
	}
	base = strings.TrimRight(base, "/")
	if strings.HasSuffix(base, "/v1/systemone") {
		return base
	}
	return base + "/v1/systemone"
}

// Evaluate sends a request to Jev and returns validated protobuf decisions.
func (c *Client) Evaluate(ctx context.Context, state any, rules []Question) (*Decisions, error) {
	if c == nil {
		return nil, fmt.Errorf("jev client is nil")
	}
	if pm, ok := state.(proto.Message); ok {
		if !pm.ProtoReflect().IsValid() {
			return nil, fmt.Errorf("nil protobuf request")
		}
		b, err := protojson.Marshal(pm)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal proto state: %w", err)
		}
		state = json.RawMessage(b)
	}
	model := c.Model
	if model == "" {
		model = "jev-latest"
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	payload, err := json.Marshal(map[string]any{"model": model, "state": state, "questions": Questions(rules)})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Jev payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpointURL(), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jev request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read Jev response: %w", err)
	}
	if len(body) > 16*1024*1024 {
		return nil, fmt.Errorf("jev response exceeds 16 MiB")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jev API returned error status %d: %s", resp.StatusCode, body)
	}
	return Decode(body, rules)
}

// Batch sequentially evaluates a slice of requests, stopping at the first error.
func Batch[Req any, Res any](ctx context.Context, reqs []Req, fn func(context.Context, Req) (Res, error)) ([]Res, error) {
	results := make([]Res, len(reqs))
	for i, req := range reqs {
		val, err := fn(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("batch item %d: %w", i, err)
		}
		results[i] = val
	}
	return results, nil
}
