// Package appinfo centralizes binary name, product, version, and build
// metadata shared by the atl-jira and atl-conf binaries.
package appinfo

// Product identifies the Atlassian product a binary targets.
type Product string

const (
	// ProductJira is the product value for the atl-jira binary.
	ProductJira Product = "jira"
	// ProductConfluence is the product value for the atl-conf binary.
	ProductConfluence Product = "confluence"
)

// Info describes a built CLI binary. It is safe to render directly as JSON.
type Info struct {
	Binary  string  `json:"binary"`
	Product Product `json:"product"`
	Version string  `json:"version"`
	Commit  string  `json:"commit,omitempty"`
	Date    string  `json:"date,omitempty"`
}

// New builds an Info, defaulting an empty version to "dev" so unversioned
// local builds still report a usable value.
func New(binary string, product Product, version, commit, date string) Info {
	if version == "" {
		version = "dev"
	}
	return Info{
		Binary:  binary,
		Product: product,
		Version: version,
		Commit:  commit,
		Date:    date,
	}
}
