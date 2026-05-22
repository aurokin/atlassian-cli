package cli

import (
	"reflect"
	"strings"
	"testing"
)

func TestSplitCommandLine(t *testing.T) {
	cases := []struct {
		in        string
		want      []string
		wantError bool
	}{
		{`pr list`, []string{"pr", "list"}, false},
		{`pr   list  --all`, []string{"pr", "list", "--all"}, false},
		{`issue create --title "a bug"`, []string{"issue", "create", "--title", "a bug"}, false},
		{`issue create --title 'a bug'`, []string{"issue", "create", "--title", "a bug"}, false},
		{`a\ b`, []string{"a b"}, false},
		{`"unterminated`, nil, true},
		{`trailing\`, nil, true},
		{``, nil, false},
	}
	for _, tc := range cases {
		got, err := splitCommandLine(tc.in)
		if tc.wantError {
			if err == nil {
				t.Errorf("splitCommandLine(%q) = %v, want error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("splitCommandLine(%q): %v", tc.in, err)
			continue
		}
		if len(got) == 0 && len(tc.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("splitCommandLine(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestExpandAliasArgs(t *testing.T) {
	aliases := map[string]string{
		"prs":   "pr list",
		"co":    "pr view",
		"loop":  "self", // self-reference
		"self":  "loop", // mutual cycle with loop
		"chain": "prs",  // references another alias
		"blank": "   ",  // empty expansion
	}
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"simple", []string{"prs"}, []string{"pr", "list"}},
		{"with trailing args", []string{"prs", "--all"}, []string{"pr", "list", "--all"}},
		{"non-alias unchanged", []string{"repo", "list"}, []string{"repo", "list"}},
		{"chained alias", []string{"chain"}, []string{"pr", "list"}},
		{"blank expansion unchanged", []string{"blank"}, []string{"blank"}},
		{"empty args", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := expandAliasArgs(tc.in, aliases)
			if err != nil {
				t.Fatalf("expandAliasArgs: %v", err)
			}
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("expandAliasArgs(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestExpandAliasArgsCycleTerminates(t *testing.T) {
	// A mutual cycle must terminate (cycle detection / depth bound) rather than
	// loop forever.
	aliases := map[string]string{"a": "b", "b": "a"}
	got, err := expandAliasArgs([]string{"a", "x"}, aliases)
	if err != nil {
		t.Fatalf("expandAliasArgs: %v", err)
	}
	// The trailing "x" must be preserved through expansion.
	if len(got) == 0 || got[len(got)-1] != "x" {
		t.Fatalf("expansion lost trailing arg: %v", got)
	}
}

// TestAliasSetListDelete exercises the alias command group end to end against a
// temp config. It runs under jiraInfo to confirm aliases are now a shared
// capability available to every atl-* binary, not just atl-bb.
func TestAliasSetListDelete(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	out, err := execRoot(t, jiraInfo(), "alias", "set", "ji", "issue view")
	if err != nil {
		t.Fatalf("alias set: %v\n%s", err, out)
	}
	if !strings.Contains(out, "set alias ji = issue view") {
		t.Fatalf("unexpected set output:\n%s", out)
	}

	out, err = execRoot(t, jiraInfo(), "alias", "list")
	if err != nil {
		t.Fatalf("alias list: %v\n%s", err, out)
	}
	if !strings.Contains(out, "ji") || !strings.Contains(out, "issue view") {
		t.Fatalf("alias list missing entry:\n%s", out)
	}

	// Round-trip through expandAliases (reads the same config).
	args, err := expandAliases([]string{"ji", "ABC-1"})
	if err != nil {
		t.Fatalf("expandAliases: %v", err)
	}
	if !reflect.DeepEqual(args, []string{"issue", "view", "ABC-1"}) {
		t.Fatalf("expandAliases = %v", args)
	}

	out, err = execRoot(t, jiraInfo(), "alias", "delete", "ji")
	if err != nil {
		t.Fatalf("alias delete: %v\n%s", err, out)
	}
	if !strings.Contains(out, "deleted alias ji") {
		t.Fatalf("unexpected delete output:\n%s", out)
	}

	out, err = execRoot(t, jiraInfo(), "alias", "list")
	if err != nil {
		t.Fatalf("alias list (after delete): %v\n%s", err, out)
	}
	if !strings.Contains(out, "No aliases defined.") {
		t.Fatalf("alias list should be empty:\n%s", out)
	}
}

func TestAliasSetRejectsBadExpansion(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := execRoot(t, jiraInfo(), "alias", "set", "bad", `"unterminated`)
	if err == nil || !strings.Contains(err.Error(), "invalid alias expansion") {
		t.Fatalf("expected invalid-expansion error, got %v", err)
	}
}

func TestAliasDeleteMissing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := execRoot(t, jiraInfo(), "alias", "delete", "nope")
	if err == nil || !strings.Contains(err.Error(), `no alias named "nope"`) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}
