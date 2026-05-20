// Command atl-bb is a true-to-API command-line interface for Atlassian
// Bitbucket Cloud.
package main

import (
	"os"

	"github.com/aurokin/atlassian-cli/internal/atlbbcmd"
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
	os.Exit(atlbbcmd.Run(version, commit, date))
}
