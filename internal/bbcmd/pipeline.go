package bbcmd

import (
	"context"
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
		newPipelineStopCommand(info, g),
		newPipelineStepsCommand(info, g),
		newPipelineLogCommand(info, g),
	)
	return cmd
}

// resolvePipelineUUID turns a "<build-number>|<uuid>" reference into a pipeline
// UUID. A build number is resolved to its run via the API; a UUID is
// brace-normalized in place.
func resolvePipelineUUID(ctx context.Context, bc *bitbucket.Client, target repoTarget, ref string) (string, error) {
	if n, ok := bitbucket.PipelineRef(ref); ok {
		raw, err := bc.GetPipelineByBuildNumber(ctx, target.Workspace, target.Repo, n)
		if err != nil {
			return "", err
		}
		p, err := bitbucket.Decode[bitbucket.Pipeline](raw)
		if err != nil {
			return "", err
		}
		return p.UUID, nil
	}
	return bitbucket.NormalizePipelineUUID(ref), nil
}

func newPipelineStopCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var repoFlag, workspaceFlag string
	cmd := &cobra.Command{
		Use:   "stop <number-or-uuid>",
		Short: "Stop an in-progress pipeline run",
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
			uuid, err := resolvePipelineUUID(cmd.Context(), bc, target, args[0])
			if err != nil {
				return err
			}
			if err := bc.StopPipeline(cmd.Context(), target.Workspace, target.Repo, uuid); err != nil {
				return err
			}
			if g.WantsStructured() {
				return cli.Render(cmd, g, pipelineStopResult{UUID: uuid, Stopped: true})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "stopped pipeline %s\n", uuid)
			return nil
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	return cmd
}

// pipelineStopResult is the synthesized outcome of a stop, whose API call
// returns no body, so --json has a stable object to render.
type pipelineStopResult struct {
	UUID    string `json:"uuid"`
	Stopped bool   `json:"stopped"`
}

func newPipelineStepsCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag, workspaceFlag string
		limit                   int
		all                     bool
	)
	cmd := &cobra.Command{
		Use:   "steps <number-or-uuid>",
		Short: "List the steps of a pipeline run",
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
			uuid, err := resolvePipelineUUID(cmd.Context(), bc, target, args[0])
			if err != nil {
				return err
			}
			list := bc.ListPipelineSteps
			if all {
				list = bc.ListPipelineStepsAll
				limit = allPageSize(limit)
			}
			raw, err := list(cmd.Context(), target.Workspace, target.Repo, uuid, limit)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, bitbucket.Decode[bitbucket.PipelineStepPage],
				func(w io.Writer, page bitbucket.PipelineStepPage) {
					writePipelineStepList(w, page.Values)
				})
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	cli.AddPaginationFlags(cmd, &limit, &all, "steps")
	return cmd
}

func newPipelineLogCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var repoFlag, workspaceFlag string
	cmd := &cobra.Command{
		Use:   "log <number-or-uuid> <step-uuid>",
		Short: "Print a pipeline step's log",
		Long: "Writes the step's log output to stdout verbatim. Find step UUIDs with\n" +
			"`pipeline steps`. Log output is raw text, so --json/--jq do not apply.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := resolveRepoTarget(nil, repoFlag, workspaceFlag)
			if err != nil {
				return err
			}
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			uuid, err := resolvePipelineUUID(cmd.Context(), bc, target, args[0])
			if err != nil {
				return err
			}
			data, err := bc.GetPipelineStepLog(cmd.Context(), target.Workspace, target.Repo, uuid, args[1])
			if err != nil {
				return err
			}
			_, err = cmd.OutOrStdout().Write(data)
			return err
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	return cmd
}

// writePipelineStepList prints steps as aligned state/uuid/name rows.
func writePipelineStepList(w io.Writer, steps []bitbucket.PipelineStep) {
	if len(steps) == 0 {
		fmt.Fprintln(w, "No steps found.")
		return
	}
	tw := output.TabWriter(w)
	for _, s := range steps {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", pipelineState(s.State), s.UUID, s.Name)
	}
	_ = tw.Flush()
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
				limit = allPageSize(limit)
			}
			raw, err := list(cmd.Context(), target.Workspace, target.Repo, st, limit)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, bitbucket.Decode[bitbucket.PipelinePage],
				func(w io.Writer, page bitbucket.PipelinePage) {
					writePipelineList(w, page.Values)
				})
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
			return cli.RenderDecoded(cmd, g, raw, bitbucket.Decode[bitbucket.Pipeline], writePipeline)
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
			return cli.RenderDecoded(cmd, g, raw, bitbucket.Decode[bitbucket.Pipeline],
				func(w io.Writer, p bitbucket.Pipeline) {
					fmt.Fprintf(w, "triggered pipeline #%d on %s %s\n",
						p.BuildNumber, refType, refName)
				})
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
	lw := output.NewLabelWriter(w)
	lw.Addf("build", "#%d", p.BuildNumber)
	lw.AddIf("uuid", p.UUID)
	lw.AddIf("state", pipelineState(p.State))
	lw.AddIf("target", pipelineTarget(p.Target))
	lw.AddIf("creator", accountLabel(p.Creator))
	lw.AddIf("created", p.CreatedOn)
	_ = lw.Flush()
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
