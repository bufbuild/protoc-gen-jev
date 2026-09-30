package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"

	incidentv1 "github.com/bufbuild/protoc-gen-jev/examples/go/gen/incident/v1"
	"github.com/bufbuild/protoc-gen-jev/pkg/jev"
)

func main() {
	verbose := os.Getenv("JEV_VERBOSE") == "1"
	logf := func(format string, args ...any) {
		if verbose {
			fmt.Printf(format, args...)
		}
	}
	logln := func(args ...any) {
		if verbose {
			fmt.Println(args...)
		}
	}
	if !verbose {
		fmt.Println("Go")
	}

	logln("==================================================")
	logln("  protoc-gen-jev: Go Client End-to-End Example   ")
	logln("==================================================")
	apiKey := os.Getenv("TYPESAFE_API_KEY")
	customEndpoint := os.Getenv("JEV_ENDPOINT")

	jevClient := jev.NewClient(apiKey)
	if customEndpoint != "" {
		jevClient.BaseURL = customEndpoint
		logf("\n[INFO] Using custom Jev/Laya endpoint: %s\n", customEndpoint)
	} else if apiKey != "" {
		logln("\n[INFO] Using live TypeSafe AI Jev API: https://api.typesafe.ai/v1/systemone")
	} else {
		logln("\n[INFO] Using default TypeSafe AI endpoint")
	}

	client := incidentv1.NewJevIncidentTriageService(jevClient)

	// 1. Inspect generated questions
	questions := client.BuildTriageDetailsQuestions()
	qJSON, _ := json.MarshalIndent(questions, "", "  ")
	logf("\n1. Built Jev Questions (%d total):\n%s\n", len(questions), string(qJSON))

	// 2. Detailed decisions: use the returned protobuf fields directly.
	req := (incidentv1.TriageRequest_builder{
		IncidentId:  "INC-8891",
		Title:       "Database connection pool exhausted",
		Description: "API latency increased to 4500ms and 500 errors spike to 12%",
		RawLogs:     "Connection refused on port 5432 after 100 pool max connections",
	}).Build()

	logln("\n2. Evaluating Single Incident State...")
	ctx := context.Background()
	decision, err := client.TriageDetails(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error evaluating state: %v\n", err)
		os.Exit(1)
	}

	decBytes, _ := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(decision)
	fmt.Println(string(decBytes))

	// 3. Scalar decisions: the batch method returns ordinary bool/enum/numeric fields.
	logln("\n3. Batch Evaluating 3 Incident States...")
	batchReqs := []*incidentv1.TriageRequest{
		(incidentv1.TriageRequest_builder{
			IncidentId:  "INC-8892",
			Title:       "Ingress 502 bad gateway spikes across region us-east-1",
			Description: "Edge proxy reports connection reset by peer from upstream cluster",
			RawLogs:     "HTTP 502 Bad Gateway - upstream connect error or disconnect/reset before headers",
		}).Build(),
		(incidentv1.TriageRequest_builder{
			IncidentId:  "INC-8893",
			Title:       "Low-priority deprecation warning logged in analytics service",
			Description: "Client library using deprecated v1 query endpoint; scheduled for removal in Q3",
			RawLogs:     "WARN [analytics-worker] Endpoint /v1/query is deprecated, migrate to /v2/query",
		}).Build(),
		(incidentv1.TriageRequest_builder{
			IncidentId:  "INC-8894",
			Title:       "Routine memory compaction completed without customer impact",
			Description: "Background compaction cycle reclaimed 4.2GB memory; latency within SLO",
			RawLogs:     "INFO [compactor] Compaction cycle finished in 45s, 0 errors, 4200MB reclaimed",
		}).Build(),
	}

	batchDecisions, err := client.BatchTriage(ctx, batchReqs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error in BatchTriage: %v\n", err)
		os.Exit(1)
	}

	if !verbose {
		fmt.Println("\n  Triage · scalar decisions")
		fmt.Printf("  %-10s %-5s %-8s %-7s %-7s %-20s %s\n", "INCIDENT", "PAGE", "PRIORITY", "URGENCY", "BLAST", "ROUTE", "CLASSIFICATION")
	}
	logf("✔ Successfully evaluated %d batch items.\n", len(batchDecisions))
	for i, d := range batchDecisions {
		if !verbose {
			printDecision(batchReqs[i].GetIncidentId(), d)
			continue
		}
		target := routingTarget(d)
		logf("  - Item [%d]: RoutingTarget=%s, Paging=%t, Priority=%s, Urgency=%d, BlastRadius=%.1f%%\n",
			i+1, target, d.GetRequiresImmediatePaging(), d.GetPriority().String(), d.GetUrgencyRating(), d.GetBlastRadiusPercentage())
	}

	logln("\n✔ Go End-to-End Test PASSED successfully!")
}

func printDecision(id string, d *incidentv1.TriageResponse) {
	route := routingTarget(d)
	page := "no"
	if d.GetRequiresImmediatePaging() {
		page = "yes"
	}
	fmt.Printf("  %-10s %-5s %-8s %-7d %-7s %-20s %s\n", id, page, strings.TrimPrefix(d.GetPriority().String(), "PRIORITY_LEVEL_"), d.GetUrgencyRating(), fmt.Sprintf("%.1f%%", d.GetBlastRadiusPercentage()), route, d.GetComplianceClassification())
}

func routingTarget(d *incidentv1.TriageResponse) string {
	switch {
	case d.HasAutomatedRunbook():
		return d.GetAutomatedRunbook()
	case d.HasOncallEngineer():
		return d.GetOncallEngineer()
	case d.HasIncidentCommander():
		return d.GetIncidentCommander()
	default:
		return "none"
	}
}
