// Package auth models the Atlassian authentication styles the CLI supports
// and signs outbound HTTP requests for them. It never persists raw tokens;
// callers supply the token value at request time.
package auth

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

// TokenStyle identifies how a credential authenticates against Atlassian.
type TokenStyle string

const (
	// StyleCloudClassic is a Cloud general API token: Basic auth against the
	// product site URL.
	StyleCloudClassic TokenStyle = "cloud-classic"
	// StyleCloudScoped is a Cloud scoped API token: Basic auth against the
	// api.atlassian.com gateway, addressed by cloud ID.
	StyleCloudScoped TokenStyle = "cloud-scoped"
	// StyleDataCenterPAT is a Data Center / Server personal access token:
	// Bearer auth against the instance URL.
	StyleDataCenterPAT TokenStyle = "data-center-pat"
	// StyleOAuth3LO is an Atlassian OAuth 2.0 (3LO) credential: Bearer auth
	// against the api.atlassian.com gateway, addressed by cloud ID. The Token
	// field carries the current access token; refresh and storage of the wider
	// token bundle live outside this package.
	StyleOAuth3LO TokenStyle = "oauth-3lo"
)

// AllStyles lists every supported token style, in documentation order.
var AllStyles = []TokenStyle{StyleCloudClassic, StyleCloudScoped, StyleDataCenterPAT, StyleOAuth3LO}

// Valid reports whether s is a supported token style.
func (s TokenStyle) Valid() bool {
	switch s {
	case StyleCloudClassic, StyleCloudScoped, StyleDataCenterPAT, StyleOAuth3LO:
		return true
	default:
		return false
	}
}

// AuthType returns the auth_type recorded in config for this style.
func (s TokenStyle) AuthType() string {
	switch s {
	case StyleCloudClassic, StyleCloudScoped:
		return "api-token-basic"
	case StyleDataCenterPAT:
		return "pat-bearer"
	case StyleOAuth3LO:
		return "oauth-bearer"
	default:
		return ""
	}
}

// ParseTokenStyle converts a string to a TokenStyle, returning a structured
// error for unknown values.
func ParseTokenStyle(s string) (TokenStyle, error) {
	ts := TokenStyle(s)
	if !ts.Valid() {
		return "", apperr.InvalidInput(fmt.Sprintf("unknown token style %q; expected one of %s", s, styleList()))
	}
	return ts, nil
}

// Credential is the set of values needed to sign a request. The Token is held
// only in memory for the duration of a command and is never written to disk
// by this package.
type Credential struct {
	Style    TokenStyle
	Username string // account email; required for Basic-auth styles
	Token    string // raw token value
	CloudID  string // required for StyleCloudScoped and StyleOAuth3LO
}

// Validate checks that the credential has every field its style requires.
// It returns a structured *apperr.Error on the first missing field.
func (c Credential) Validate() error {
	if !c.Style.Valid() {
		return apperr.InvalidInput(fmt.Sprintf("unknown token style %q; expected one of %s", c.Style, styleList()))
	}
	if c.Token == "" {
		return apperr.InvalidInput(fmt.Sprintf("token style %s requires a token", c.Style))
	}
	switch c.Style {
	case StyleCloudClassic, StyleCloudScoped:
		if c.Username == "" {
			return apperr.InvalidInput(fmt.Sprintf("token style %s requires a username", c.Style))
		}
	}
	switch c.Style {
	case StyleCloudScoped, StyleOAuth3LO:
		if c.CloudID == "" {
			return apperr.InvalidInput(fmt.Sprintf("token style %s requires a cloud_id", c.Style))
		}
	}
	return nil
}

// Sign validates the credential and applies the appropriate Authorization
// header to req.
func (c Credential) Sign(req *http.Request) error {
	if err := c.Validate(); err != nil {
		return err
	}
	switch c.Style {
	case StyleCloudClassic, StyleCloudScoped:
		req.SetBasicAuth(c.Username, c.Token)
	case StyleDataCenterPAT, StyleOAuth3LO:
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	return nil
}

// styleList renders AllStyles as a comma-separated string for error messages.
func styleList() string {
	parts := make([]string, len(AllStyles))
	for i, s := range AllStyles {
		parts[i] = string(s)
	}
	return strings.Join(parts, ", ")
}
