// Command atl-jira is a true-to-API command-line interface for Atlassian Jira.
package main

import (
	"os"

	"github.com/aurokin/atlassian-cli/internal/atljiracmd"
)

// Build metadata, overridable at release time via:
//
//	-ldflags "-X main.version=... -X main.commit=... -X main.date=..."
var (
	version = ""
	commit  = ""
	date    = ""
)

func main() {
	os.Exit(atljiracmd.Run(version, commit, date))
}
