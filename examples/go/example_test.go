package main_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"

	incidentv1 "github.com/sudorandom/protoc-gen-jev/examples/go/gen/incident/v1"
	"github.com/sudorandom/protoc-gen-jev/pkg/jev"
)

func ExampleJevIncidentTriageService() {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"model": "jev-latest",
			"answers": {
				"routing_target": { "type": "choice", "choice": "oncall_engineer" },
				"requiresImmediatePaging": { "type": "noul", "noul": true },
				"priority": { "type": "choice", "choice": "PRIORITY_LEVEL_HIGH" },
				"urgencyRating": { "type": "score", "score": 3.0 },
				"blastRadiusPercentage": { "type": "score", "score": 2.0 },
				"complianceClassification": { "type": "choice", "choice": "INTERNAL_CONFIDENTIAL" }
			}
		}`))
	}))
	defer server.Close()

	jevClient := jev.NewClient("test-key")
	jevClient.BaseURL = server.URL
	jevClient.HTTPClient = server.Client()

	client := incidentv1.NewJevIncidentTriageService(jevClient)

	req := (incidentv1.TriageRequest_builder{
		IncidentId:  "INC-1234",
		Title:       "Database failover triggered",
		Description: "Primary node unresponsive; standby promoted successfully",
	}).Build()

	decision, err := client.Triage(ctx, req)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Route: %s\n", decision.GetOncallEngineer())
	fmt.Printf("Paging: %v\n", decision.GetRequiresImmediatePaging())
	fmt.Printf("Priority: %s\n", decision.GetPriority().String())
	fmt.Printf("Urgency: %d\n", decision.GetUrgencyRating())

	// Output:
	// Route: oncall_engineer
	// Paging: true
	// Priority: PRIORITY_LEVEL_HIGH
	// Urgency: 4
}

// Both RPCs map the same provider answers into their declared response types.
func TestTriageStyles(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
   "model":"test-model", "usage":{"input_tokens":12,"output_tokens":4},
   "answers":{
    "routing_target":{"type":"choice","choice":"oncall_engineer","confidence":0.9},
    "requiresImmediatePaging":{"type":"noul","noul":0.9,"confidence":0},
    "priority":{"type":"choice","choice":"PRIORITY_LEVEL_HIGH","probabilities":{"PRIORITY_LEVEL_HIGH":0.9}},
    "urgencyRating":{"type":"score","score":2.5,"probabilities":{"2":0.5,"3":0.5},"legend":{"2":"Needs attention soon","3":"Urgent"}},
    "blastRadiusPercentage":{"type":"score","score":1.5},
    "complianceClassification":{"type":"choice","choice":"INTERNAL_CONFIDENTIAL"}
   }
  }`))
	}))
	defer server.Close()
	transport := jev.NewClient("test")
	transport.BaseURL = server.URL
	transport.HTTPClient = server.Client()
	client := incidentv1.NewJevIncidentTriageService(transport)
	req := (incidentv1.TriageRequest_builder{Title: "Database outage"}).Build()
	scalar, err := client.Triage(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	require.True(t, scalar.GetRequiresImmediatePaging())
	require.Equal(t, int32(4), scalar.GetUrgencyRating())
	require.Equal(t, incidentv1.PriorityLevel_PRIORITY_LEVEL_HIGH, scalar.GetPriority())
	require.Equal(t, "oncall_engineer", scalar.GetOncallEngineer())
	details, err := client.TriageDetails(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.True(t, details.GetRequiresImmediatePaging().GetValue())
	require.InDelta(t, 0.9, details.GetRequiresImmediatePaging().GetProbability(), 1e-9)
	require.True(t, details.GetRequiresImmediatePaging().HasConfidence())
	require.Zero(t, details.GetRequiresImmediatePaging().GetConfidence())
	require.InDelta(t, 3.5, details.GetUrgencyRating().GetValue(), 1e-9)
	require.InDelta(t, 2.5, details.GetUrgencyRating().GetScore(), 1e-9)
	require.InDelta(t, 0.5, details.GetUrgencyRating().GetProbabilities()["2"], 1e-9)
	require.Equal(t, "Urgent", details.GetUrgencyRating().GetLegend()["3"])
	require.InDelta(t, float64(scalar.GetBlastRadiusPercentage()), details.GetBlastRadiusPercentage().GetValue(), 1e-9)
	require.Equal(t, scalar.GetPriority().String(), details.GetPriority().GetValue())
	require.Equal(t, scalar.GetOncallEngineer(), details.GetRoutingTarget().GetValue())
	require.False(t, details.GetComplianceClassification().HasConfidence())
	require.Equal(t, "test-model", details.GetMeta().GetModel())
	require.Equal(t, int32(12), details.GetMeta().GetUsage().GetInputTokens())
	// The result is ready for standard Protobuf serialization by the caller.
	_, err = protojson.Marshal(details)
	require.NoError(t, err)
}
