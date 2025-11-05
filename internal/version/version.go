package version

import "fmt"

var (
	// Version is the main version number that is being run at the moment.
	Version = "0.1.0"

	// GitCommit is the git commit that was compiled. This will be filled in by the compiler.
	GitCommit string

	// BuildDate is the date the binary was built
	BuildDate string
)

// FullVersion returns the full version string
func FullVersion() string {
	version := Version
	if GitCommit != "" {
		version += fmt.Sprintf(" (%s)", GitCommit[:7])
	}
	if BuildDate != "" {
		version += fmt.Sprintf(" built on %s", BuildDate)
	}
	return version
}
