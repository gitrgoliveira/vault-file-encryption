package version

import (
	"fmt"
	"testing"
)

func TestFullVersion(t *testing.T) {
	// Store original values
	originalVersion := Version
	originalGitCommit := GitCommit
	originalBuildDate := BuildDate

	// Defer cleanup
	defer func() {
		Version = originalVersion
		GitCommit = originalGitCommit
		BuildDate = originalBuildDate
	}()

	testCases := []struct {
		name      string
		version   string
		gitCommit string
		buildDate string
		expected  string
	}{
		{
			name:      "version only",
			version:   "1.0.0",
			gitCommit: "",
			buildDate: "",
			expected:  "1.0.0",
		},
		{
			name:      "version and short git commit",
			version:   "1.1.0",
			gitCommit: "abcdef123456",
			buildDate: "",
			expected:  "1.1.0 (abcdef1)",
		},
		{
			name:      "version and long git commit",
			version:   "1.2.0",
			gitCommit: "abcdef1234567890",
			buildDate: "",
			expected:  "1.2.0 (abcdef1)",
		},
		{
			name:      "version and build date",
			version:   "1.3.0",
			gitCommit: "",
			buildDate: "2023-01-01",
			expected:  "1.3.0 built on 2023-01-01",
		},
		{
			name:      "full version string",
			version:   "2.0.0",
			gitCommit: "fedcba987654",
			buildDate: "2023-02-14",
			expected:  "2.0.0 (fedcba9) built on 2023-02-14",
		},
		{
			name:      "empty values",
			version:   "",
			gitCommit: "",
			buildDate: "",
			expected:  "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			Version = tc.version
			GitCommit = tc.gitCommit
			BuildDate = tc.buildDate

			actual := FullVersion()
			if actual != tc.expected {
				t.Errorf("expected version string '%s', but got '%s'", tc.expected, actual)
			}
		})
	}
}

func ExampleFullVersion() {
	// Store original values and defer cleanup
	originalVersion, originalGitCommit, originalBuildDate := Version, GitCommit, BuildDate
	defer func() {
		Version, GitCommit, BuildDate = originalVersion, originalGitCommit, originalBuildDate
	}()

	// Example 1: Version only
	Version = "1.0.0"
	GitCommit = ""
	BuildDate = ""
	fmt.Println(FullVersion())

	// Example 2: Version with Git commit
	Version = "1.1.0"
	GitCommit = "abcdef123456"
	BuildDate = ""
	fmt.Println(FullVersion())

	// Example 3: Full version details
	Version = "2.0.0"
	GitCommit = "fedcba987654"
	BuildDate = "2023-02-14"
	fmt.Println(FullVersion())

	// Output:
	// 1.0.0
	// 1.1.0 (abcdef1)
	// 2.0.0 (fedcba9) built on 2023-02-14
}
