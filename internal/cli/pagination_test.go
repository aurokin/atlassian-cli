package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestAddPaginationFlags(t *testing.T) {
	var (
		limit int
		all   bool
	)
	cmd := &cobra.Command{Use: "list"}
	AddPaginationFlags(cmd, &limit, &all, "issues")

	limitFlag := cmd.Flags().Lookup("limit")
	if limitFlag == nil {
		t.Fatal("--limit not registered")
	}
	if got, want := limitFlag.Usage, "maximum number of issues per page"; got != want {
		t.Fatalf("--limit usage = %q, want %q", got, want)
	}

	allFlag := cmd.Flags().Lookup("all")
	if allFlag == nil {
		t.Fatal("--all not registered")
	}
	if allFlag.Value.Type() != "bool" {
		t.Fatalf("--all type = %q, want bool", allFlag.Value.Type())
	}

	// The pointers are bound: parsing sets the caller's variables.
	if err := cmd.Flags().Parse([]string{"--limit", "25", "--all"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if limit != 25 || !all {
		t.Fatalf("limit=%d all=%v, want 25/true", limit, all)
	}
}
