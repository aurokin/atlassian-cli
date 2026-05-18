// Command atl-conf is a true-to-API command-line interface for Atlassian Confluence.
package main

import (
	"os"

	"github.com/aurokin/atlassian-cli/internal/atlconfcmd"
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
	os.Exit(atlconfcmd.Run(version, commit, date))
}
