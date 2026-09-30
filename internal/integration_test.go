package integration_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	incidentv1 "github.com/bufbuild/protoc-gen-jev/examples/go/gen/incident/v1"
	"github.com/bufbuild/protoc-gen-jev/pkg/jev"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestIncidentJevClient_WithFauxRPC(t *testing.T) {
	ctx := context.Background()

	// Locate OpenAPI schema and stub files
	absSchemaDir, err := filepath.Abs("../testdata/openapi")
	require.NoError(t, err)

	// Spin up FauxRPC container with Testcontainers
	req := testcontainers.ContainerRequest{
		Image:        "docker.io/sudorandom/fauxrpc:v0.29.1",
		ExposedPorts: []string{"6660/tcp"},
		Cmd: []string{
			"run",
			"--schema=/openapi/typesafe-jev.yaml",
			"--stubs=/openapi/stubs.jev.yaml",
			"--addr=0.0.0.0:6660",
		},
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      filepath.Join(absSchemaDir, "typesafe-jev.yaml"),
				ContainerFilePath: "/openapi/typesafe-jev.yaml",
				FileMode:          0644,
			},
			{
				HostFilePath:      filepath.Join(absSchemaDir, "stubs.jev.yaml"),
				ContainerFilePath: "/openapi/stubs.jev.yaml",
				FileMode:          0644,
			},
		},
		WaitingFor: wait.ForHTTP("/fauxrpc/openapi-docs/").WithPort("6660/tcp").WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err, "failed to start fauxrpc testcontainer")
	t.Cleanup(func() {
		_ = container.Terminate(ctx)
	})

	endpoint, err := container.PortEndpoint(ctx, "6660/tcp", "http")
	require.NoError(t, err)

	// Point the generated Jev client to the FauxRPC container
	jevClient := jev.NewClient("mock-api-key")
	jevClient.BaseURL = endpoint
	client := incidentv1.NewJevIncidentTriageService(jevClient)

	// 1. Test BuildTriageQuestions
	questions := client.BuildTriageQuestions()
	assert.Len(t, questions, 6)
	assert.Contains(t, questions, "routing_target")
	assert.Contains(t, questions, "requiresImmediatePaging")
	assert.Contains(t, questions, "priority")
	assert.Contains(t, questions, "urgencyRating")
	assert.Contains(t, questions, "blastRadiusPercentage")
	assert.Contains(t, questions, "complianceClassification")

	// 2. Test Triage against FauxRPC HTTP OpenAPI endpoint
	triageReq := (incidentv1.TriageRequest_builder{
		IncidentId:  "INC-9912",
		Title:       "Database connection pool exhausted",
		Description: "API latency increased to 4500ms and 500 errors spike to 12%",
	}).Build()

	decision, err := client.Triage(ctx, triageReq)
	require.NoError(t, err)
	require.NotNil(t, decision)

	// Validate typed responses from FauxRPC OpenAPI stubs
	assert.Equal(t, "oncall_engineer", decision.GetOncallEngineer())
	assert.True(t, decision.GetRequiresImmediatePaging())
	assert.Equal(t, incidentv1.PriorityLevel_PRIORITY_LEVEL_HIGH, decision.GetPriority())
	assert.Equal(t, int32(4), decision.GetUrgencyRating())
	assert.InDelta(t, 45.0, float64(decision.GetBlastRadiusPercentage()), 0.001)
	assert.Equal(t, "INTERNAL_CONFIDENTIAL", decision.GetComplianceClassification())

	// 3. Test BatchTriage against FauxRPC
	batchReqs := []*incidentv1.TriageRequest{
		(incidentv1.TriageRequest_builder{IncidentId: "INC-1", Title: "Crash 1"}).Build(),
		(incidentv1.TriageRequest_builder{IncidentId: "INC-2", Title: "Crash 2"}).Build(),
		(incidentv1.TriageRequest_builder{IncidentId: "INC-3", Title: "Crash 3"}).Build(),
	}

	decisions, err := client.BatchTriage(ctx, batchReqs)
	require.NoError(t, err)
	require.Len(t, decisions, 3)

	for i, d := range decisions {
		assert.Equal(t, "oncall_engineer", d.GetOncallEngineer(), "batch item %d", i)
		assert.True(t, d.GetRequiresImmediatePaging(), "batch item %d", i)
		assert.Equal(t, incidentv1.PriorityLevel_PRIORITY_LEVEL_HIGH, d.GetPriority(), "batch item %d", i)
	}

	// 4. Failing test case: Calling an invalid endpoint returns error status
	invalidJev := jev.NewClient("mock-api-key")
	invalidJev.BaseURL = fmt.Sprintf("%s/v1/nonexistent", endpoint)
	invalidClient := incidentv1.NewJevIncidentTriageService(invalidJev)
	_, err = invalidClient.Triage(ctx, triageReq)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}
