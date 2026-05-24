package confcmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/conf"
)

// contentEditOps are the client operations a versioned content edit needs.
// Pages and blogposts share an identical v2 field shape, so a single edit flow
// (runContentEdit) drives both; only these closures and the noun differ.
type contentEditOps struct {
	noun   string // "page" or "blogpost", for messages
	get    func(ctx context.Context, id string) (json.RawMessage, error)
	getADF func(ctx context.Context, id string) (string, error)
	update func(ctx context.Context, id, status, title, bodyFormat, body string, version int) (json.RawMessage, error)
}

// validateContentEditFlags checks the title/body/body-format flag combination
// for a page or blogpost edit. It is called before client construction so a
// clean config still surfaces an invalid_input error rather than
// site_not_configured.
func validateContentEditFlags(noun string, titleSet, bodySet bool, bodyFormat string) error {
	if !titleSet && !bodySet {
		return apperr.InvalidInput(
			noun + " edit requires at least one change; pass --title or --body")
	}
	if bodySet && bodyFormat == "" {
		return apperr.InvalidInput("--body requires --body-format")
	}
	if !bodySet && bodyFormat != "" {
		return apperr.InvalidInput("--body-format is only valid together with --body")
	}
	if bodySet {
		if err := validateBodyFormat(bodyFormat); err != nil {
			return err
		}
	}
	return nil
}

// runContentEdit performs a versioned title/body edit of a page or blogpost.
// Confluence v2 treats an update as a full replacement, so it first GETs the
// content to read its current title, body, status, and version, then sends the
// merged state with the version number incremented by one. A title-only edit
// re-sends the existing body: the storage representation when present, falling
// back to atlas_doc_format for content authored in the modern editor. The
// caller is expected to have run validateContentEditFlags before constructing
// the client.
func runContentEdit(cmd *cobra.Command, g *cli.GlobalFlags, ops contentEditOps, id string,
	titleSet bool, title string, bodySet bool, body, bodyFormat string) error {
	raw, err := ops.get(cmd.Context(), id)
	if err != nil {
		return err
	}
	// Pages and blogposts share the v2 page field shape, so both decode into
	// conf.Page for the round-trip.
	cur, err := conf.Decode[conf.Page](raw)
	if err != nil {
		return err
	}
	newTitle := cur.Title
	if titleSet {
		newTitle = title
	}
	newFormat, newBody := bodyFormat, body
	if !bodySet {
		switch {
		case cur.Body.Storage.Value != "":
			newFormat, newBody = "storage", cur.Body.Storage.Value
		default:
			adf, err := ops.getADF(cmd.Context(), id)
			if err != nil {
				return err
			}
			if adf == "" {
				return apperr.InvalidInput(fmt.Sprintf(
					"%s %s has no storage or atlas_doc_format body to preserve; "+
						"pass --body with --body-format to set the body explicitly", ops.noun, id))
			}
			newFormat, newBody = "atlas_doc_format", adf
		}
	}
	updated, err := ops.update(cmd.Context(), id, cur.Status, newTitle, newFormat, newBody, cur.Version.Number+1)
	if err != nil {
		return err
	}
	return cli.RenderDecoded(cmd, g, updated, conf.Decode[conf.Page],
		func(w io.Writer, p conf.Page) {
			fmt.Fprintf(w, "updated %s %s to version %d\n", ops.noun, p.ID, p.Version.Number)
		})
}

// adfBodyFrom re-fetches content in the atlas_doc_format representation and
// returns its body value (empty if the content has none). It backs the
// title-only edit fallback for modern-editor content, whose storage
// representation is empty.
func adfBodyFrom(ctx context.Context, getWithFormat func(context.Context, string, string) (json.RawMessage, error), id string) (string, error) {
	raw, err := getWithFormat(ctx, id, "atlas_doc_format")
	if err != nil {
		return "", err
	}
	p, err := conf.Decode[conf.Page](raw)
	if err != nil {
		return "", err
	}
	return p.Body.AtlasDocFormat.Value, nil
}
