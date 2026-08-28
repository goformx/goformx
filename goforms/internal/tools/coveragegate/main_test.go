package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCoverageWeightsStatements(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "coverage.out")
	require.NoError(t, os.WriteFile(path, []byte("mode: atomic\nexample.go:1.1,2.1 3 1\nexample.go:4.1,5.1 1 0\n"), 0o600))

	percentage, err := coverage(path)
	require.NoError(t, err)
	require.InDelta(t, 75, percentage, 0.01)
}

func TestCoverageRejectsMalformedProfiles(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "coverage.out")
	require.NoError(t, os.WriteFile(path, []byte("not a profile\n"), 0o600))

	_, err := coverage(path)
	require.ErrorContains(t, err, "header")
}
