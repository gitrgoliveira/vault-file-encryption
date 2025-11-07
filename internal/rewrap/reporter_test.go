package rewrap

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/gitrgoliveira/vault-file-encryption/internal/vault"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewReporter(t *testing.T) {
	reporter := NewReporter()
	require.NotNil(t, reporter)
	assert.NotNil(t, reporter.stats)
	assert.Equal(t, 0, reporter.stats.TotalFiles)
	assert.NotNil(t, reporter.stats.VersionCounts)
	assert.NotNil(t, reporter.stats.Results)
}

func TestReporter_AddResult(t *testing.T) {
	tests := []struct {
		name     string
		results  []*vault.RewrapResult
		expected Statistics
	}{
		{
			name: "successful rewrap",
			results: []*vault.RewrapResult{
				{
					FilePath:   "/data/file1.key",
					OldVersion: 1,
					NewVersion: 3,
				},
			},
			expected: Statistics{
				TotalFiles: 1,
				Successful: 1,
				Failed:     0,
				Skipped:    0,
				VersionCounts: map[int]int{
					1: 1,
				},
			},
		},
		{
			name: "failed rewrap",
			results: []*vault.RewrapResult{
				{
					FilePath:   "/data/file2.key",
					OldVersion: 2,
					Error:      errors.New("vault error"),
				},
			},
			expected: Statistics{
				TotalFiles: 1,
				Successful: 0,
				Failed:     1,
				Skipped:    0,
				VersionCounts: map[int]int{
					2: 1,
				},
			},
		},
		{
			name: "skipped (already at min version)",
			results: []*vault.RewrapResult{
				{
					FilePath:   "/data/file3.key",
					OldVersion: 5,
					NewVersion: 0, // No rewrap performed
				},
			},
			expected: Statistics{
				TotalFiles: 1,
				Successful: 0,
				Failed:     0,
				Skipped:    1,
				VersionCounts: map[int]int{
					5: 1,
				},
			},
		},
		{
			name: "multiple results mixed",
			results: []*vault.RewrapResult{
				{FilePath: "/data/f1.key", OldVersion: 1, NewVersion: 3},
				{FilePath: "/data/f2.key", OldVersion: 1, NewVersion: 3},
				{FilePath: "/data/f3.key", OldVersion: 2, NewVersion: 3},
				{FilePath: "/data/f4.key", OldVersion: 3, NewVersion: 0},
				{FilePath: "/data/f5.key", OldVersion: 1, Error: errors.New("fail")},
			},
			expected: Statistics{
				TotalFiles: 5,
				Successful: 3,
				Failed:     1,
				Skipped:    1,
				VersionCounts: map[int]int{
					1: 3,
					2: 1,
					3: 1,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reporter := NewReporter()

			for _, result := range tt.results {
				reporter.AddResult(result)
			}

			stats := reporter.GetStatistics()
			assert.Equal(t, tt.expected.TotalFiles, stats.TotalFiles)
			assert.Equal(t, tt.expected.Successful, stats.Successful)
			assert.Equal(t, tt.expected.Failed, stats.Failed)
			assert.Equal(t, tt.expected.Skipped, stats.Skipped)
			assert.Equal(t, tt.expected.VersionCounts, stats.VersionCounts)
		})
	}
}

func TestReporter_WriteText(t *testing.T) {
	reporter := NewReporter()
	reporter.AddResults([]*vault.RewrapResult{
		{FilePath: "/data/f1.key", OldVersion: 1, NewVersion: 3},
		{FilePath: "/data/f2.key", OldVersion: 2, NewVersion: 3},
		{FilePath: "/data/f3.key", OldVersion: 3, NewVersion: 0},
		{FilePath: "/data/f4.key", OldVersion: 1, Error: errors.New("vault error")},
	})

	t.Run("summary only", func(t *testing.T) {
		var buf bytes.Buffer
		err := reporter.WriteText(&buf, false)
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "Total Files:   4")
		assert.Contains(t, output, "Successful:    2")
		assert.Contains(t, output, "Failed:        1")
		assert.Contains(t, output, "Skipped:       1")
		assert.Contains(t, output, "Version Distribution:")
		assert.Contains(t, output, "v1")
		assert.Contains(t, output, "v2")
		assert.Contains(t, output, "v3")

		// Should not contain detailed results
		assert.NotContains(t, output, "Detailed Results:")
	})

	t.Run("with details", func(t *testing.T) {
		var buf bytes.Buffer
		err := reporter.WriteText(&buf, true)
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "Detailed Results:")
		assert.Contains(t, output, "/data/f1.key")
		assert.Contains(t, output, "v1 -> v3")
		assert.Contains(t, output, "SUCCESS")
		assert.Contains(t, output, "FAILED: vault error")
		assert.Contains(t, output, "SKIPPED")
	})
}

func TestReporter_WriteJSON(t *testing.T) {
	reporter := NewReporter()
	reporter.AddResults([]*vault.RewrapResult{
		{FilePath: "/data/f1.key", OldVersion: 1, NewVersion: 3},
		{FilePath: "/data/f2.key", OldVersion: 2, Error: errors.New("error")},
	})

	t.Run("without results", func(t *testing.T) {
		var buf bytes.Buffer
		err := reporter.WriteJSON(&buf, false)
		require.NoError(t, err)

		var stats Statistics
		err = json.Unmarshal(buf.Bytes(), &stats)
		require.NoError(t, err)

		assert.Equal(t, 2, stats.TotalFiles)
		assert.Equal(t, 1, stats.Successful)
		assert.Equal(t, 1, stats.Failed)
		assert.Nil(t, stats.Results) // Should not include results
	})

	t.Run("with results", func(t *testing.T) {
		var buf bytes.Buffer
		err := reporter.WriteJSON(&buf, true)
		require.NoError(t, err)

		// Verify JSON is valid
		var result map[string]interface{}
		err = json.Unmarshal(buf.Bytes(), &result)
		require.NoError(t, err)

		// Verify basic stats
		assert.Equal(t, float64(2), result["total_files"])
		assert.Equal(t, float64(1), result["successful"])
		assert.Equal(t, float64(1), result["failed"])

		// Verify results array exists
		results, ok := result["results"].([]interface{})
		require.True(t, ok, "results should be an array")
		assert.Equal(t, 2, len(results))
	})
}

func TestReporter_WriteCSV(t *testing.T) {
	reporter := NewReporter()
	reporter.AddResults([]*vault.RewrapResult{
		{FilePath: "/data/f1.key", OldVersion: 1, NewVersion: 3, BackupCreated: true},
		{FilePath: "/data/f2.key", OldVersion: 2, NewVersion: 0},
		{FilePath: "/data/f3.key", OldVersion: 1, Error: errors.New("test error")},
	})

	var buf bytes.Buffer
	err := reporter.WriteCSV(&buf)
	require.NoError(t, err)

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Verify header
	assert.Equal(t, "FilePath,OldVersion,NewVersion,Status,BackupCreated,Error", lines[0])

	// Verify data rows
	assert.Equal(t, 4, len(lines)) // Header + 3 data rows
	assert.Contains(t, output, "/data/f1.key,1,3,success,true,")
	assert.Contains(t, output, "/data/f2.key,2,0,skipped,false,")
	assert.Contains(t, output, "/data/f3.key,1,0,failed,false,test error")
}

func TestReporter_GetFailedFiles(t *testing.T) {
	reporter := NewReporter()
	reporter.AddResults([]*vault.RewrapResult{
		{FilePath: "/data/f1.key", OldVersion: 1, NewVersion: 3},
		{FilePath: "/data/f2.key", OldVersion: 1, Error: errors.New("error1")},
		{FilePath: "/data/f3.key", OldVersion: 2, Error: errors.New("error2")},
		{FilePath: "/data/f4.key", OldVersion: 3, NewVersion: 0},
	})

	failed := reporter.GetFailedFiles()
	assert.Equal(t, 2, len(failed))
	assert.Contains(t, failed, "/data/f2.key")
	assert.Contains(t, failed, "/data/f3.key")
}

func TestReporter_GetSuccessfulFiles(t *testing.T) {
	reporter := NewReporter()
	reporter.AddResults([]*vault.RewrapResult{
		{FilePath: "/data/f1.key", OldVersion: 1, NewVersion: 3},
		{FilePath: "/data/f2.key", OldVersion: 2, NewVersion: 3},
		{FilePath: "/data/f3.key", OldVersion: 1, Error: errors.New("error")},
		{FilePath: "/data/f4.key", OldVersion: 3, NewVersion: 0},
	})

	successful := reporter.GetSuccessfulFiles()
	assert.Equal(t, 2, len(successful))
	assert.Contains(t, successful, "/data/f1.key")
	assert.Contains(t, successful, "/data/f2.key")
}

func TestReporter_GetSkippedFiles(t *testing.T) {
	reporter := NewReporter()
	reporter.AddResults([]*vault.RewrapResult{
		{FilePath: "/data/f1.key", OldVersion: 1, NewVersion: 3},
		{FilePath: "/data/f2.key", OldVersion: 3, NewVersion: 0},
		{FilePath: "/data/f3.key", OldVersion: 5, NewVersion: 0},
		{FilePath: "/data/f4.key", OldVersion: 1, Error: errors.New("error")},
	})

	skipped := reporter.GetSkippedFiles()
	assert.Equal(t, 2, len(skipped))
	assert.Contains(t, skipped, "/data/f2.key")
	assert.Contains(t, skipped, "/data/f3.key")
}

func TestReporter_AddResults(t *testing.T) {
	reporter := NewReporter()

	results := []*vault.RewrapResult{
		{FilePath: "/data/f1.key", OldVersion: 1, NewVersion: 3},
		{FilePath: "/data/f2.key", OldVersion: 2, NewVersion: 3},
	}

	reporter.AddResults(results)

	stats := reporter.GetStatistics()
	assert.Equal(t, 2, stats.TotalFiles)
	assert.Equal(t, 2, stats.Successful)
}
