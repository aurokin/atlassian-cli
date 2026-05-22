// Command gen-docs generates the Markdown command reference for the Atlassian
// CLIs. It is product-agnostic: each CLI's root command is built by its
// atl*cmd package and walked by cobra/doc, so one tool documents atl-jira,
// atl-conf, and atl-bb identically.
//
// Usage:
//
//	go run ./cmd/gen-docs --product all --out docs/cli
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/aurokin/atlassian-cli/internal/atlbbcmd"
	"github.com/aurokin/atlassian-cli/internal/atlconfcmd"
	"github.com/aurokin/atlassian-cli/internal/atljiracmd"
)

// rootBuilder builds a product's root command for documentation. Build metadata
// is irrelevant to docs, so empty version/commit/date are passed.
type rootBuilder func() *cobra.Command

// builders maps an appinfo product name to its root-command builder. Adding a
// product is a one-line change here.
var builders = map[string]rootBuilder{
	"jira":       func() *cobra.Command { r, _ := atljiracmd.NewRoot("", "", ""); return r },
	"confluence": func() *cobra.Command { r, _ := atlconfcmd.NewRoot("", "", ""); return r },
	"bitbucket":  func() *cobra.Command { r, _ := atlbbcmd.NewRoot("", "", ""); return r },
}

func main() {
	out := flag.String("out", "docs/cli", "output directory for the generated Markdown tree")
	product := flag.String("product", "all", "product to document: jira, confluence, bitbucket, or all")
	flag.Parse()

	if err := run(*out, *product); err != nil {
		fmt.Fprintln(os.Stderr, "gen-docs:", err)
		os.Exit(1)
	}
}

// run generates the Markdown command tree for the selected product(s) under
// outDir, one subdirectory per product.
func run(outDir, product string) error {
	selected, err := selectBuilders(product)
	if err != nil {
		return err
	}
	for _, name := range sortedKeys(selected) {
		root := selected[name]()
		// A stable footer keeps the generated docs reproducible in version
		// control (no timestamp churn on every regeneration).
		root.DisableAutoGenTag = true
		dir := filepath.Join(outDir, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create output directory %q: %w", dir, err)
		}
		if err := doc.GenMarkdownTree(root, dir); err != nil {
			return fmt.Errorf("generate %s docs: %w", name, err)
		}
		fmt.Printf("generated %s docs in %s\n", name, dir)
	}
	return nil
}

// selectBuilders returns the builder map for product, or every builder when
// product is "all" (or empty).
func selectBuilders(product string) (map[string]rootBuilder, error) {
	if product == "all" || product == "" {
		return builders, nil
	}
	b, ok := builders[product]
	if !ok {
		return nil, fmt.Errorf("unknown product %q (want jira, confluence, bitbucket, or all)", product)
	}
	return map[string]rootBuilder{product: b}, nil
}

// sortedKeys returns a map's keys in sorted order for deterministic output.
func sortedKeys(m map[string]rootBuilder) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
