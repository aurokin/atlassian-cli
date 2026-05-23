package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestCompleteSiteNamesReturnsConfiguredSites(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	// Two profiles, registered out of order; completion should sort them.
	for _, site := range []string{"work", "play"} {
		if _, err := execRoot(t, jiraInfo(), "auth", "login", "--site", site,
			"--url", "https://example.atlassian.net", "--token-style", "data-center-pat",
			"--token-env", "FOO"); err != nil {
			t.Fatalf("login %s: %v", site, err)
		}
	}

	got, directive := completeSiteNames(nil, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("directive = %v, want NoFileComp", directive)
	}
	want := []string{"play", "work"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v (sorted)", got, want)
		}
	}
}

func TestCompleteSiteNamesEmptyWhenNoConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	got, directive := completeSiteNames(nil, nil, "")
	if len(got) != 0 {
		t.Fatalf("got %v, want no suggestions", got)
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("directive = %v, want NoFileComp", directive)
	}
}

func TestFixedCompletionReturnsValues(t *testing.T) {
	fn := FixedCompletion("a", "b", "c")
	got, directive := fn(nil, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("directive = %v, want NoFileComp", directive)
	}
	if len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Fatalf("got %v, want [a b c]", got)
	}
}

func TestNewRootRegistersGroupsAndTagsCommands(t *testing.T) {
	root, _ := NewRoot(jiraInfo(), "short")

	wantGroups := map[string]bool{GroupProduct: false, GroupConfig: false, GroupAdvanced: false}
	for _, grp := range root.Groups() {
		if _, ok := wantGroups[grp.ID]; ok {
			wantGroups[grp.ID] = true
		}
	}
	for id, found := range wantGroups {
		if !found {
			t.Errorf("group %q was not registered on the root", id)
		}
	}

	// Shared commands are filed under their expected groups.
	wantCommandGroup := map[string]string{
		"auth":      GroupConfig,
		"version":   GroupConfig,
		"api":       GroupAdvanced,
		"resolve":   GroupAdvanced,
		"browse":    GroupAdvanced,
		"alias":     GroupAdvanced,
		"extension": GroupAdvanced,
	}
	for _, c := range root.Commands() {
		want, ok := wantCommandGroup[c.Name()]
		if !ok {
			continue
		}
		if c.GroupID != want {
			t.Errorf("command %q GroupID = %q, want %q", c.Name(), c.GroupID, want)
		}
	}
}

func TestAddProductCommandsTagsWithProductGroup(t *testing.T) {
	root, _ := NewRoot(jiraInfo(), "short")
	sub := &cobra.Command{Use: "widget"}
	AddProductCommands(root, sub)
	if sub.GroupID != GroupProduct {
		t.Fatalf("GroupID = %q, want %q", sub.GroupID, GroupProduct)
	}
}
