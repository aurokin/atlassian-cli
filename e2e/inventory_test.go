package e2e

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/atlbbcmd"
	"github.com/aurokin/atlassian-cli/internal/atlconfcmd"
	"github.com/aurokin/atlassian-cli/internal/atljiracmd"
	"github.com/spf13/cobra"
)

type commandCoverage struct {
	Command  string `json:"command"`
	Evidence []struct {
		Kind string `json:"kind"`
		File string `json:"file"`
		Test string `json:"test"`
	} `json:"evidence,omitempty"`
	Unverified string `json:"unverified,omitempty"`
}

// The manifest records evidence pointers, not a claim that every flag, auth mode
// or optional capability passed. Every new runnable command needs an explicit
// coverage decision, and stale test references fail this guard.
func TestCommandCoverageInventory(t *testing.T) {
	jira, _ := atljiracmd.NewRoot("test", "", "")
	conf, _ := atlconfcmd.NewRoot("test", "", "")
	bb, _ := atlbbcmd.NewRoot("test", "", "")
	commands := map[string]bool{}
	var visit func(*cobra.Command)
	visit = func(cmd *cobra.Command) {
		if cmd.Runnable() {
			commands[cmd.CommandPath()] = true
		}
		for _, child := range cmd.Commands() {
			visit(child)
		}
	}
	for _, root := range []*cobra.Command{jira, conf, bb} {
		root.InitDefaultHelpCmd()
		root.InitDefaultCompletionCmd()
		visit(root)
	}
	data, err := os.ReadFile("coverage.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Description string            `json:"description"`
		Commands    []commandCoverage `json:"commands"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	tests := map[string]map[string]bool{}
	for _, row := range manifest.Commands {
		if seen[row.Command] {
			t.Errorf("duplicate command %s", row.Command)
		}
		seen[row.Command] = true
		if !commands[row.Command] {
			t.Errorf("manifest command no longer runnable: %s", row.Command)
		}
		if len(row.Evidence) == 0 && strings.TrimSpace(row.Unverified) == "" {
			t.Errorf("%s needs evidence or an explicit unverified reason", row.Command)
		}
		for _, evidence := range row.Evidence {
			prefix := map[string]string{"process": "e2e/", "live": "integration/", "package": "internal/"}[evidence.Kind]
			if prefix == "" || !strings.HasPrefix(evidence.File, prefix) || strings.Contains(evidence.File, "..") || !strings.HasSuffix(evidence.File, "_test.go") {
				t.Errorf("%s invalid evidence classification: %+v", row.Command, evidence)
				continue
			}
			if tests[evidence.File] == nil {
				file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", evidence.File), nil, 0)
				if err != nil {
					t.Errorf("%s evidence file: %v", row.Command, err)
					continue
				}
				tests[evidence.File] = map[string]bool{}
				for _, decl := range file.Decls {
					if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && strings.HasPrefix(fn.Name.Name, "Test") {
						tests[evidence.File][fn.Name.Name] = true
					}
				}
			}
			if !tests[evidence.File][evidence.Test] {
				t.Errorf("%s claims nonexistent test %s:%s", row.Command, evidence.File, evidence.Test)
			}
		}
	}
	missing := []string{}
	for name := range commands {
		if !seen[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	for _, name := range missing {
		t.Errorf("missing coverage entry: %s", name)
	}
	t.Logf("accounted for %d runnable commands; evidence is not whole-command or full-auth-mode coverage", len(commands))
}
