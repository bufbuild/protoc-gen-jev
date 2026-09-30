package golden_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sudorandom/protoc-gen-jev/internal/golden"
)

func TestGoldenOutputs(t *testing.T) {
	// Root project directories
	actualJevDir := filepath.Join("..", "..", "gen", "jev")
	goldenJevDir := filepath.Join("..", "..", "testdata", "golden")

	require.DirExists(t, actualJevDir, "generated directory %s does not exist. Run buf generate first.", actualJevDir)

	golden.VerifyDir(t, actualJevDir, goldenJevDir)
}
