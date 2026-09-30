package jev_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/sudorandom/protoc-gen-jev/pkg/jev"
)

func ExampleNewClient() {
	client := jev.NewClient("my-api-key")
	fmt.Println("BaseURL:", client.BaseURL)
	fmt.Println("Model:", client.Model)

	// Customizing the client is straightforward:
	client.BaseURL = "http://localhost:6660"
	client.Model = "custom-model"
	fmt.Println("Custom BaseURL:", client.BaseURL)
	fmt.Println("Custom Model:", client.Model)

	// Output:
	// BaseURL: https://api.typesafe.ai
	// Model: jev-latest
	// Custom BaseURL: http://localhost:6660
	// Custom Model: custom-model
}

func ExampleBatch() {
	ctx := context.Background()

	inputs := []string{"foo", "hello", "world"}
	lengths, err := jev.Batch(ctx, inputs, func(ctx context.Context, s string) (int, error) {
		return len(s), nil
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(lengths)

	// Output:
	// [3 5 5]
}

func ExampleClient_Evaluate() {
	ctx := context.Background()

	// Mock Jev server that returns a canonical answer
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"model": "jev-latest",
			"answers": {
				"page": { "type": "noul", "noul": true }
			}
		}`))
	}))
	defer server.Close()

	client := jev.NewClient("test-key")
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

	questions := []jev.Question{
		{
			Name:         "page",
			Type:         "noul",
			Field:        "page",
			Instructions: "Does this incident require paging?",
		},
	}

	out, err := client.Evaluate(ctx, "database down", questions)
	if err != nil {
		panic(err)
	}

	fmt.Println("Result:", out.Nouls["page"].GetValue())

	// Output:
	// Result: true
}
