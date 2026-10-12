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

// workersAIRunPath marks a Cloudflare Workers AI model URL such as
// https://api.cloudflare.com/client/v4/accounts/{account}/ai/run/@cf/cloudflare/clef.
const workersAIRunPath = "/ai/run/"

// Client executes decision evaluations against the Jev API.
type Client struct {
	// APIKey is sent as a bearer token. For Workers AI it is a Cloudflare API token.
	APIKey string
	// BaseURL is the API root; the Jev path /v1/systemone is appended unless
	// present. A Cloudflare Workers AI model URL (containing /ai/run/) is used
	// as is, and the response envelope is unwrapped automatically.
	BaseURL string
	// Model is the model requested in the payload. It is ignored for Workers AI,
	// where the model is the last segment of BaseURL.
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
	if strings.HasSuffix(base, "/v1/systemone") || isWorkersAIURL(base) {
		return base
	}
	return base + "/v1/systemone"
}

// modelName returns the model to request: the Workers AI URL's model for
// Workers AI endpoints, otherwise the configured model or the Jev default.
func (c *Client) modelName() string {
	endpoint := c.endpointURL()
	if isWorkersAIURL(endpoint) {
		return endpoint[strings.LastIndex(endpoint, "/")+1:]
	}
	if c.Model == "" {
		return "jev-latest"
	}
	return c.Model
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
	model := c.modelName()
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
	if isWorkersAIURL(c.endpointURL()) {
		body, err = unwrapWorkersAIEnvelope(body)
		if err != nil {
			return nil, err
		}
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

// workersAIEnvelope is the wrapper Cloudflare Workers AI puts around a Jev response.
type workersAIEnvelope struct {
	Result  json.RawMessage `json:"result"`
	Success bool            `json:"success"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

// isWorkersAIURL reports whether the URL addresses a Cloudflare Workers AI model.
func isWorkersAIURL(url string) bool {
	return strings.Contains(url, workersAIRunPath)
}

// unwrapWorkersAIEnvelope extracts the Jev response from a Workers AI envelope.
func unwrapWorkersAIEnvelope(body []byte) ([]byte, error) {
	var envelope workersAIEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("failed to decode Workers AI envelope: %w", err)
	}
	if !envelope.Success {
		messages := make([]string, 0, len(envelope.Errors))
		for _, apiError := range envelope.Errors {
			messages = append(messages, fmt.Sprintf("%d: %s", apiError.Code, apiError.Message))
		}
		return nil, fmt.Errorf("workers ai request failed: %s", strings.Join(messages, "; "))
	}
	if len(envelope.Result) == 0 {
		return nil, fmt.Errorf("workers ai response has no result")
	}
	return envelope.Result, nil
}
