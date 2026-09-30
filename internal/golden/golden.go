package golden

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var Update = flag.Bool("update", false, "update .golden files")

// AssertMatches compares actual bytes against the golden file.
// If -update flag or UPDATE_GOLDEN=1 is set, it updates the golden file.
func AssertMatches(t *testing.T, goldenPath string, actualBytes []byte) {
	t.Helper()

	shouldUpdate := *Update || os.Getenv("UPDATE_GOLDEN") == "1" || os.Getenv("UPDATE_GOLDEN") == "true"

	if shouldUpdate {
		err := os.MkdirAll(filepath.Dir(goldenPath), 0755)
		require.NoError(t, err, "failed to create directory for golden file %s", goldenPath)

		err = os.WriteFile(goldenPath, actualBytes, 0644)
		require.NoError(t, err, "failed to write golden file %s", goldenPath)

		t.Logf("Updated golden file: %s", goldenPath)
		return
	}

	goldenBytes, err := os.ReadFile(goldenPath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Fatalf("golden file %s does not exist. Run with -update to create it.", goldenPath)
		}
		require.NoError(t, err, "failed to read golden file %s", goldenPath)
	}

	// Normalize CRLF to LF
	actualClean := bytes.ReplaceAll(actualBytes, []byte("\r\n"), []byte("\n"))
	goldenClean := bytes.ReplaceAll(goldenBytes, []byte("\r\n"), []byte("\n"))

	// If JSON, compare formatted JSON
	if filepath.Ext(goldenPath) == ".json" {
		assert.JSONEq(t, string(goldenClean), string(actualClean), "Golden JSON mismatch for %s", goldenPath)
		return
	}

	assert.Equal(t, string(goldenClean), string(actualClean), "Golden mismatch for %s", goldenPath)
}

// DeleteAll removes the golden directory.
func DeleteAll(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(dir)
}

// VerifyDir compares an entire output directory against a golden directory.
func VerifyDir(t *testing.T, actualDir, goldenDir string) {
	t.Helper()

	actualFiles := map[string]bool{}
	err := filepath.Walk(actualDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".pyc") || strings.Contains(path, "__pycache__") || strings.HasPrefix(filepath.Base(path), ".") {
			return nil
		}
		rel, err := filepath.Rel(actualDir, path)
		if err != nil {
			return err
		}

		actualFiles[rel] = true
		goldenFile := filepath.Join(goldenDir, rel)
		data, err := os.ReadFile(path)
		require.NoError(t, err, "failed to read actual file %s", path)

		t.Run(rel, func(t *testing.T) {
			AssertMatches(t, goldenFile, data)
		})
		return nil
	})

	require.NoError(t, err, "failed walking directory %s", actualDir)
	if !*Update && os.Getenv("UPDATE_GOLDEN") != "1" && os.Getenv("UPDATE_GOLDEN") != "true" {
		err = filepath.Walk(goldenDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || strings.HasSuffix(path, ".pyc") || strings.Contains(path, "__pycache__") || strings.HasPrefix(filepath.Base(path), ".") {
				return nil
			}
			rel, err := filepath.Rel(goldenDir, path)
			if err != nil {
				return err
			}
			assert.True(t, actualFiles[rel], "expected generated file is missing: %s", rel)
			return nil
		})
		require.NoError(t, err)
	}
}
