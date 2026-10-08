// Package domain holds the instance module's model.
package domain

// Product is the product name every instance reports.
const Product = "Nerve Wiki"

// APIVersion is the version of the HTTP API this build serves. It prefixes
// every API path (v0.1 design 6.1).
const APIVersion = "v0"

// Build identifies the binary an instance runs.
type Build struct {
	Version string // product version, e.g. "0.1.0-dev"
	Commit  string // git revision, "unknown" without a VCS stamp
}

// Info is what an instance tells API clients about itself.
type Info struct {
	Product    string
	Version    string
	Commit     string
	APIVersion string
	// SignupEnabled is auth.signup_enabled: whether anyone may register.
	SignupEnabled bool
	// WorkspaceCreationEnabled is workspace.creation_enabled: whether
	// accounts may create workspaces.
	WorkspaceCreationEnabled bool
	// AssetMaxBytes is asset.max_bytes: the largest attachment an upload
	// may send, which a client checks before it sends one.
	AssetMaxBytes int64
}
