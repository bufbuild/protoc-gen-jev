package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rulesv1 "github.com/sudorandom/protoc-gen-jev/gen/jev/ai/rules/v1"
	"github.com/sudorandom/protoc-gen-jev/pkg/jev"
)

func TestGeneratedClient_BuildQuestions(t *testing.T) {
	client := rulesv1.NewRuleTestRecordJevClient(nil)
	questions := client.BuildQuestions()

	// 1. Oneof delivery_method -> Choice
	require.Contains(t, questions, "delivery_method")
	dm, ok := questions["delivery_method"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "choice", dm["type"])
	crit, ok := dm["criteria"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, crit, "email")
	assert.Contains(t, crit, "sms")
	assert.Contains(t, crit, "push_notification")

	// 2. Enum.in executionMode -> Choice with only MODE_FAST and MODE_BALANCED
	require.Contains(t, questions, "executionMode")
	em, ok := questions["executionMode"].(map[string]any)
	require.True(t, ok)
	emCrit, ok := em["criteria"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, emCrit, 2)
	assert.Contains(t, emCrit, "MODE_FAST")
	assert.Contains(t, emCrit, "MODE_BALANCED")

	// 3. Enum.not_in filteredMode -> MODE_DEBUG must NOT be in criteria
	require.Contains(t, questions, "filteredMode")
	fm, ok := questions["filteredMode"].(map[string]any)
	require.True(t, ok)
	fmCrit, ok := fm["criteria"].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, fmCrit, "MODE_DEBUG")

	// 4. Integer small range ratingSmall -> 1..5
	require.Contains(t, questions, "ratingSmall")
	rs, ok := questions["ratingSmall"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []string{"1", "2", "3", "4", "5"}, rs["criteria"])

	// 5. Integer discrete allowed values discreteCode -> 10, 20, 50, 100
	require.Contains(t, questions, "discreteCode")
	dc, ok := questions["discreteCode"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []string{"10", "20", "50", "100"}, dc["criteria"])

	// 6. Large scale integer largeScale -> [0, 250, 500, 750, 1000]
	require.Contains(t, questions, "largeScale")
	ls, ok := questions["largeScale"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []string{"0", "250", "500", "750", "1000"}, ls["criteria"])

	// 7. Float continuous temperature -> [-40.0, -15.0, 10.0, 35.0, 60.0]
	require.Contains(t, questions, "temperature")
	temp, ok := questions["temperature"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []string{"-40.0", "-15.0", "10.0", "35.0", "60.0"}, temp["criteria"])

	// 8. String securityClearance -> Choice with PUBLIC, SECRET, TOP_SECRET
	require.Contains(t, questions, "securityClearance")
	sc, ok := questions["securityClearance"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "choice", sc["type"])
	scCrit, ok := sc["criteria"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, scCrit, 3)

	// 9. Custom Jev criteria decisionFlag -> Choice with APPROVE, REJECT
	require.Contains(t, questions, "decisionFlag")
	df, ok := questions["decisionFlag"].(map[string]any)
	require.True(t, ok)
	dfCrit, ok := df["criteria"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, dfCrit, "APPROVE")
	assert.Contains(t, dfCrit, "REJECT")

	// 10. Custom Jev min/max customBoundedScore -> [10.0, 20.0, 30.0, 40.0, 50.0]
	require.Contains(t, questions, "customBoundedScore")
	cbs, ok := questions["customBoundedScore"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []string{"10.0", "20.0", "30.0", "40.0", "50.0"}, cbs["criteria"])

	// 11. Skipped & freeform fields MUST NOT be present
	assert.NotContains(t, questions, "secretToken")
	assert.NotContains(t, questions, "freeformDescription")
	assert.NotContains(t, questions, "optionalNote")
}

func TestGeneratedClient_Evaluate(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": map[string]any{
				"delivery_method":   map[string]any{"choice": "email"},
				"executionMode":     map[string]any{"choice": "MODE_FAST"},
				"filteredMode":      map[string]any{"choice": "MODE_BALANCED"},
				"securityClearance": map[string]any{"choice": "TOP_SECRET"},
				"decisionFlag":      map[string]any{"choice": "APPROVE"},
			},
			"scores": map[string]any{
				"ratingSmall":        map[string]any{"score": 4.0}, // index 4 in [1,2,3,4,5] -> 5
				"ratingStrict":       map[string]any{"score": 2.0}, // index 2 in [1,2,3] -> 3
				"discreteCode":       map[string]any{"score": 3.0}, // index 3 in [10,20,50,100] -> 100
				"largeScale":         map[string]any{"score": 3.0}, // index 3 in [0,250,500,750,1000] -> 750
				"temperature":        map[string]any{"score": 3.0}, // index 3 in [-40,-15,10,35,60] -> 35.0
				"discreteRatio":      map[string]any{"score": 2.0}, // index 2 in [0.2,0.5,0.8] -> 0.8
				"customBoundedScore": map[string]any{"score": 3.0}, // index 3 in [10,20,30,40,50] -> 40.0
			},
		}
		_ = json.NewEncoder(w).Encode(completeResponse(resp))
	}))
	defer ts.Close()

	jevClient := jev.NewClient("dummy-key")
	jevClient.BaseURL = ts.URL
	jevClient.HTTPClient = ts.Client()
	client := rulesv1.NewRuleTestRecordJevClient(jevClient)

	decisions, err := client.Evaluate(context.Background(), map[string]any{"text": "Sample request"})
	require.NoError(t, err)

	assert.Equal(t, "email", decisions.GetEmail())
	assert.Equal(t, rulesv1.Mode_MODE_FAST, decisions.GetExecutionMode())
	assert.Equal(t, int32(5), decisions.GetRatingSmall())
	assert.Equal(t, "TOP_SECRET", decisions.GetSecurityClearance())
	assert.InDelta(t, 40.0, float64(decisions.GetCustomBoundedScore()), 0.001)
}

func TestGeneratedClient_BatchEvaluate(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		resp := map[string]any{
			"choices": map[string]any{
				"delivery_method": map[string]any{"choice": "sms"},
			},
			"scores": map[string]any{
				"ratingSmall": map[string]any{"score": 3.0}, // index 3 in [1,2,3,4,5] -> 4
			},
		}
		_ = json.NewEncoder(w).Encode(completeResponse(resp))
	}))
	defer ts.Close()

	batchJevClient := jev.NewClient("dummy-key")
	batchJevClient.BaseURL = ts.URL
	batchJevClient.HTTPClient = ts.Client()
	client := rulesv1.NewRuleTestRecordJevClient(batchJevClient)

	states := []any{
		"first text",
		"second text",
		"third text",
	}

	results, err := client.BatchEvaluate(context.Background(), states)
	require.NoError(t, err)
	require.Len(t, results, 3)
	assert.Equal(t, 3, callCount)

	for _, dec := range results {
		assert.Equal(t, "sms", dec.GetSms())
		assert.Equal(t, int32(4), dec.GetRatingSmall())
	}
}

func TestGeneratedClient_Evaluate_HTTPError400(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": "invalid questions payload: missing type"}`))
	}))
	defer ts.Close()

	jevClient := jev.NewClient("dummy-key")
	jevClient.BaseURL = ts.URL
	jevClient.HTTPClient = ts.Client()
	client := rulesv1.NewRuleTestRecordJevClient(jevClient)

	_, err := client.Evaluate(context.Background(), map[string]any{"text": "Sample request"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
	assert.Contains(t, err.Error(), "invalid questions payload")
}

func TestGeneratedClient_Evaluate_HTTPError500(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`internal server error`))
	}))
	defer ts.Close()

	jevClient := jev.NewClient("dummy-key")
	jevClient.BaseURL = ts.URL
	jevClient.HTTPClient = ts.Client()
	client := rulesv1.NewRuleTestRecordJevClient(jevClient)

	_, err := client.Evaluate(context.Background(), map[string]any{"text": "Sample request"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestGeneratedClient_Evaluate_MalformedJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not a valid json`))
	}))
	defer ts.Close()

	jevClient := jev.NewClient("dummy-key")
	jevClient.BaseURL = ts.URL
	jevClient.HTTPClient = ts.Client()
	client := rulesv1.NewRuleTestRecordJevClient(jevClient)

	_, err := client.Evaluate(context.Background(), map[string]any{"text": "Sample request"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode Jev response")
}

func TestGeneratedClient_Evaluate_NetworkError(t *testing.T) {
	jevClient := jev.NewClient("dummy-key")
	jevClient.BaseURL = "http://127.0.0.1:1"
	client := rulesv1.NewRuleTestRecordJevClient(jevClient)

	_, err := client.Evaluate(context.Background(), map[string]any{"text": "Sample request"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "jev request failed")
}

// Fill every requested question; partial responses are now rejected.
func completeResponse(overrides map[string]any) map[string]any {
	out := map[string]any{"choices": map[string]any{}, "nouls": map[string]any{}, "scores": map[string]any{}}
	for name, raw := range rulesv1.NewRuleTestRecordJevClient(nil).BuildQuestions() {
		q := raw.(map[string]any)
		switch q["type"] {
		case "choice":
			var labels []string
			for label := range q["criteria"].(map[string]any) {
				labels = append(labels, label)
			}
			sort.Strings(labels)
			out["choices"].(map[string]any)[name] = map[string]any{"choice": labels[0]}
		case "score":
			out["scores"].(map[string]any)[name] = map[string]any{"score": 0}
		case "noul":
			out["nouls"].(map[string]any)[name] = map[string]any{"noul": 0}
		}
	}
	for group, entries := range overrides {
		for key, value := range entries.(map[string]any) {
			out[group].(map[string]any)[key] = value
		}
	}
	return out
}
