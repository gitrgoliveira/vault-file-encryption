package rewrap

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/gitrgoliveira/vault-file-encryption/internal/vault"
)

// Statistics contains aggregated rewrap operation statistics.
type Statistics struct {
	TotalFiles    int                   `json:"total_files"`
	Successful    int                   `json:"successful"`
	Failed        int                   `json:"failed"`
	Skipped       int                   `json:"skipped"`           // Files already at minimum version
	VersionCounts map[int]int           `json:"version_counts"`    // Count by version number
	Results       []*vault.RewrapResult `json:"results,omitempty"` // Individual results (optional)
}

// Reporter generates statistics and reports from rewrap results.
type Reporter struct {
	stats *Statistics
}

// NewReporter creates a new statistics reporter.
func NewReporter() *Reporter {
	return &Reporter{
		stats: &Statistics{
			VersionCounts: make(map[int]int),
			Results:       make([]*vault.RewrapResult, 0),
		},
	}
}

// AddResult processes a rewrap result and updates statistics.
func (r *Reporter) AddResult(result *vault.RewrapResult) {
	r.stats.TotalFiles++
	r.stats.Results = append(r.stats.Results, result)

	// Track old version
	if result.OldVersion > 0 {
		r.stats.VersionCounts[result.OldVersion]++
	}

	// Categorize result
	if result.Error != nil {
		r.stats.Failed++
	} else if result.NewVersion == 0 {
		// NewVersion == 0 means no rewrap was performed (already at min version)
		r.stats.Skipped++
	} else {
		r.stats.Successful++
	}
}

// AddResults processes multiple results.
func (r *Reporter) AddResults(results []*vault.RewrapResult) {
	for _, result := range results {
		r.AddResult(result)
	}
}

// GetStatistics returns the current statistics.
func (r *Reporter) GetStatistics() *Statistics {
	return r.stats
}

// WriteText outputs statistics in human-readable text format.
func (r *Reporter) WriteText(w io.Writer, includeDetails bool) error {
	if _, err := fmt.Fprintf(w, "Rewrap Statistics\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "=================\n\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Total Files:   %d\n", r.stats.TotalFiles); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Successful:    %d\n", r.stats.Successful); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Failed:        %d\n", r.stats.Failed); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Skipped:       %d (already at minimum version)\n\n", r.stats.Skipped); err != nil {
		return err
	}

	// Version distribution
	if len(r.stats.VersionCounts) > 0 {
		if _, err := fmt.Fprintf(w, "Version Distribution:\n"); err != nil {
			return err
		}

		// Sort versions for consistent output
		versions := make([]int, 0, len(r.stats.VersionCounts))
		for v := range r.stats.VersionCounts {
			versions = append(versions, v)
		}
		sort.Ints(versions)

		for _, v := range versions {
			count := r.stats.VersionCounts[v]
			if _, err := fmt.Fprintf(w, "  v%-3d: %d files\n", v, count); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	// Detailed results if requested
	if includeDetails && len(r.stats.Results) > 0 {
		if _, err := fmt.Fprintf(w, "Detailed Results:\n"); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "-----------------\n"); err != nil {
			return err
		}
		for _, result := range r.stats.Results {
			status := "SUCCESS"
			if result.Error != nil {
				status = fmt.Sprintf("FAILED: %v", result.Error)
			} else if result.NewVersion == 0 {
				status = "SKIPPED"
			}

			if _, err := fmt.Fprintf(w, "  %s: v%d", result.FilePath, result.OldVersion); err != nil {
				return err
			}
			if result.NewVersion > 0 {
				if _, err := fmt.Fprintf(w, " -> v%d", result.NewVersion); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintf(w, " [%s]\n", status); err != nil {
				return err
			}
		}
	}

	return nil
}

// WriteJSON outputs statistics in JSON format.
func (r *Reporter) WriteJSON(w io.Writer, includeResults bool) error {
	stats := r.stats
	if !includeResults {
		// Create copy without results
		stats = &Statistics{
			TotalFiles:    r.stats.TotalFiles,
			Successful:    r.stats.Successful,
			Failed:        r.stats.Failed,
			Skipped:       r.stats.Skipped,
			VersionCounts: r.stats.VersionCounts,
		}
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(stats)
}

// WriteCSV outputs statistics in CSV format.
func (r *Reporter) WriteCSV(w io.Writer) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write header
	header := []string{"FilePath", "OldVersion", "NewVersion", "Status", "BackupCreated", "Error"}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write rows
	for _, result := range r.stats.Results {
		status := "success"
		errorMsg := ""

		if result.Error != nil {
			status = "failed"
			errorMsg = result.Error.Error()
		} else if result.NewVersion == 0 {
			status = "skipped"
		}

		backupCreated := "false"
		if result.BackupCreated {
			backupCreated = "true"
		}

		row := []string{
			result.FilePath,
			fmt.Sprintf("%d", result.OldVersion),
			fmt.Sprintf("%d", result.NewVersion),
			status,
			backupCreated,
			errorMsg,
		}

		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	return nil
}

// GetFailedFiles returns a list of files that failed to rewrap.
func (r *Reporter) GetFailedFiles() []string {
	failed := make([]string, 0)
	for _, result := range r.stats.Results {
		if result.Error != nil {
			failed = append(failed, result.FilePath)
		}
	}
	return failed
}

// GetSuccessfulFiles returns a list of files that were successfully rewrapped.
func (r *Reporter) GetSuccessfulFiles() []string {
	successful := make([]string, 0)
	for _, result := range r.stats.Results {
		if result.Error == nil && result.NewVersion > 0 {
			successful = append(successful, result.FilePath)
		}
	}
	return successful
}

// GetSkippedFiles returns a list of files that were skipped (already at min version).
func (r *Reporter) GetSkippedFiles() []string {
	skipped := make([]string, 0)
	for _, result := range r.stats.Results {
		if result.Error == nil && result.NewVersion == 0 {
			skipped = append(skipped, result.FilePath)
		}
	}
	return skipped
}
