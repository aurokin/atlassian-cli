package bbcmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/bitbucket"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/output"
)

// newPipelineCommand builds the "pipeline" command group.
func newPipelineCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pipeline",
		Aliases: []string{"pipelines", "pipe"},
		Short:   "List, view, and run Bitbucket pipelines",
	}
	cmd.AddCommand(
		newPipelineListCommand(info, g),
		newPipelineViewCommand(info, g),
		newPipelineRunCommand(info, g),
	)
	return cmd
}

func newPipelineListCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag      string
		workspaceFlag string
		status        string
		limit         int
		all           bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List a repository's pipeline runs (newest first)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			target, err := resolveRepoTarget(nil, repoFlag, workspaceFlag)
			if err != nil {
				return err
			}
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			st := strings.ToUpper(strings.TrimSpace(status))
			list := bc.ListPipelines
			if all {
				list = bc.ListPipelinesAll
			}
			raw, err := list(cmd.Context(), target.Workspace, target.Repo, st, limit)
			if err != nil {
				return err
			}
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, raw)
			}
			page, err := bitbucket.Decode[bitbucket.PipelinePage](raw)
			if err != nil {
				return err
			}
			writePipelineList(cmd.OutOrStdout(), page.Values)
			return nil
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	f := cmd.Flags()
	f.StringVar(&status, "status", "", "filter by pipeline state name (e.g. PENDING, IN_PROGRESS, COMPLETED)")
	cli.AddPaginationFlags(cmd, &limit, &all, "pipeline runs")
	return cmd
}

func newPipelineViewCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag      string
		workspaceFlag string
	)
	cmd := &cobra.Command{
		Use:   "view <number-or-uuid>",
		Short: "View one pipeline run, by build number or UUID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := resolveRepoTarget(nil, repoFlag, workspaceFlag)
			if err != nil {
				return err
			}
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			var raw json.RawMessage
			if n, ok := bitbucket.PipelineRef(args[0]); ok {
				raw, err = bc.GetPipelineByBuildNumber(cmd.Context(), target.Workspace, target.Repo, n)
			} else {
				raw, err = bc.GetPipeline(cmd.Context(), target.Workspace, target.Repo, args[0])
			}
			if err != nil {
				return err
			}
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, raw)
			}
			p, err := bitbucket.Decode[bitbucket.Pipeline](raw)
			if err != nil {
				return err
			}
			writePipeline(cmd.OutOrStdout(), p)
			return nil
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	return cmd
}

func newPipelineRunCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag      string
		workspaceFlag string
		refType       string
		refName       string
	)
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Trigger a new pipeline run for a ref",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(refName) == "" {
				return apperr.InvalidInput("a ref is required; pass --ref (the branch or tag name)")
			}
			// Default an empty ref type to "branch" up front so the success
			// message reports exactly what TriggerPipeline sends.
			if strings.TrimSpace(refType) == "" {
				refType = "branch"
			}
			target, err := resolveRepoTarget(nil, repoFlag, workspaceFlag)
			if err != nil {
				return err
			}
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			raw, err := bc.TriggerPipeline(cmd.Context(), target.Workspace, target.Repo, refType, refName)
			if err != nil {
				return err
			}
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, raw)
			}
			p, err := bitbucket.Decode[bitbucket.Pipeline](raw)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "triggered pipeline #%d on %s %s\n",
				p.BuildNumber, refType, refName)
			return nil
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	f := cmd.Flags()
	f.StringVar(&refName, "ref", "", "branch or tag name to run the pipeline against (required)")
	f.StringVar(&refType, "ref-type", "branch", "ref type: branch or tag")
	return cmd
}

// writePipelineList prints pipeline runs as aligned number/state/target rows.
func writePipelineList(w io.Writer, pipelines []bitbucket.Pipeline) {
	if len(pipelines) == 0 {
		fmt.Fprintln(w, "No pipeline runs found.")
		return
	}
	tw := output.TabWriter(w)
	for _, p := range pipelines {
		fmt.Fprintf(tw, "#%d\t%s\t%s\n", p.BuildNumber, pipelineState(p.State), pipelineTarget(p.Target))
	}
	_ = tw.Flush()
}

// writePipeline prints a single pipeline run as aligned label/value lines.
func writePipeline(w io.Writer, p bitbucket.Pipeline) {
	fmt.Fprintf(w, "%-12s #%d\n", "build:", p.BuildNumber)
	if p.UUID != "" {
		fmt.Fprintf(w, "%-12s %s\n", "uuid:", p.UUID)
	}
	if s := pipelineState(p.State); s != "" {
		fmt.Fprintf(w, "%-12s %s\n", "state:", s)
	}
	if t := pipelineTarget(p.Target); t != "" {
		fmt.Fprintf(w, "%-12s %s\n", "target:", t)
	}
	if creator := accountLabel(p.Creator); creator != "" {
		fmt.Fprintf(w, "%-12s %s\n", "creator:", creator)
	}
	if p.CreatedOn != "" {
		fmt.Fprintf(w, "%-12s %s\n", "created:", p.CreatedOn)
	}
}

// pipelineState renders a state as "NAME (RESULT)", "NAME", or "".
func pipelineState(s *bitbucket.PipelineState) string {
	if s == nil {
		return ""
	}
	if s.Result != nil && s.Result.Name != "" {
		if s.Name != "" {
			return s.Name + " (" + s.Result.Name + ")"
		}
		return s.Result.Name
	}
	return s.Name
}

// pipelineTarget renders a target as "ref_type:ref_name", "ref_name", or "".
func pipelineTarget(t *bitbucket.PipelineTarget) string {
	if t == nil {
		return ""
	}
	switch {
	case t.RefType != "" && t.RefName != "":
		return t.RefType + ":" + t.RefName
	default:
		return t.RefName
	}
}
