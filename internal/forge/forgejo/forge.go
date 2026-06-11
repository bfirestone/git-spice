// Package forgejo provides a wrapper around Forgejo's APIs
// in a manner compliant with the [forge.Forge] interface.
package forgejo

import (
	"cmp"
	"context"
	"fmt"

	"go.abhg.dev/gs/internal/forge"
	forgejogw "go.abhg.dev/gs/internal/gateway/forgejo"
	"go.abhg.dev/gs/internal/silog"
)

// DefaultURL is the default Forgejo instance URL
// (Codeberg, the flagship public instance).
const DefaultURL = "https://codeberg.org"

// Options defines command line options for the Forgejo Forge.
// These are all hidden in the CLI,
// and are expected to be set only via environment variables
// or configuration.
type Options struct {
	// URL is the URL for Forgejo.
	// Override this if you use a self-hosted Forgejo instance.
	URL string `name:"forgejo-url" hidden:"" config:"forge.forgejo.url" env:"FORGEJO_URL" help:"Base URL for Forgejo web requests"`

	// APIURL is the URL for the Forgejo API.
	APIURL string `name:"forgejo-api-url" hidden:"" config:"forge.forgejo.apiURL" env:"FORGEJO_API_URL" help:"Base URL for Forgejo API requests"`

	// Token is a fixed token used to authenticate with Forgejo.
	// This may be used to skip the login flow.
	Token string `name:"forgejo-token" hidden:"" env:"FORGEJO_TOKEN" help:"Forgejo API token"`

	// ClientID is the OAuth client ID for the Forgejo OAuth device flow.
	// There is no central Forgejo instance,
	// so an instance-registered OAuth app client ID is required
	// to use the device flow.
	ClientID string `name:"forgejo-oauth-client-id" hidden:"" env:"FORGEJO_OAUTH_CLIENT_ID" config:"forge.forgejo.oauth.clientID" help:"Forgejo OAuth client ID"`

	// DeleteBranchOnMerge specifies whether a branch should be deleted
	// after its pull request is merged.
	DeleteBranchOnMerge bool `name:"forgejo-delete-branch-on-merge" hidden:"" config:"forge.forgejo.deleteBranchOnMerge" default:"true" help:"Delete the source branch after merging a pull request"`
}

// Forge builds a Forgejo Forge.
type Forge struct {
	Options Options

	// Log specifies the logger to use.
	Log *silog.Logger
}

var _ forge.Forge = (*Forge)(nil)

func (f *Forge) logger() *silog.Logger {
	if f.Log == nil {
		return silog.Nop()
	}
	return f.Log.WithPrefix("forgejo")
}

// URL returns the base URL configured for the Forgejo Forge
// or the default URL if none is set.
func (f *Forge) URL() string {
	return cmp.Or(f.Options.URL, DefaultURL)
}

// BaseURL reports the Forgejo web URL used for host matching and links.
func (f *Forge) BaseURL() string {
	return f.URL()
}

// APIURL returns the base API URL configured for the Forgejo Forge
// or the base URL if none is set.
func (f *Forge) APIURL() string {
	return cmp.Or(f.Options.APIURL, f.URL())
}

// ID reports a unique key for this forge.
func (*Forge) ID() string { return "forgejo" }

// CLIPlugin returns the CLI plugin for the Forgejo Forge.
func (f *Forge) CLIPlugin() any { return &f.Options }

// ParseRepositoryPath parses a Forgejo repository path
// and returns a [forge.RepositoryID] for it.
//
// It returns [forge.ErrUnsupportedURL] if the path is not a valid
// Forgejo path.
func (f *Forge) ParseRepositoryPath(path string) (forge.RepositoryID, error) {
	owner, repo, ok := forge.SplitRepositoryPath(path)
	if !ok {
		return nil, fmt.Errorf("%w: %q", forge.ErrUnsupportedURL, path)
	}
	return &RepositoryID{url: f.URL(), owner: owner, name: repo}, nil
}

// ChangeTemplatePaths reports the allowed paths for possible
// PR templates.
func (f *Forge) ChangeTemplatePaths() []string {
	return []string{
		".forgejo/PULL_REQUEST_TEMPLATE.md",
		".gitea/PULL_REQUEST_TEMPLATE.md",
		".github/PULL_REQUEST_TEMPLATE.md",
		"PULL_REQUEST_TEMPLATE.md",
		"docs/PULL_REQUEST_TEMPLATE.md",
	}
}

// OpenRepository opens the Forgejo repository that the given ID points to.
func (f *Forge) OpenRepository(
	ctx context.Context,
	token forge.AuthenticationToken,
	id forge.RepositoryID,
) (forge.Repository, error) {
	rid, ok := id.(*RepositoryID)
	if !ok {
		return nil, fmt.Errorf("unexpected repository ID type: %T", id)
	}

	ft, ok := token.(*AuthenticationToken)
	if !ok {
		return nil, fmt.Errorf("unexpected token type: %T", token)
	}

	var tokenType forgejogw.TokenType
	switch ft.AuthType {
	case AuthTypeOAuth2:
		tokenType = forgejogw.TokenTypeBearer
	case AuthTypePAT, AuthTypeCLI:
		tokenType = forgejogw.TokenTypeAccessToken
	default:
		return nil, fmt.Errorf("unsupported auth type: %v", ft.AuthType)
	}

	client, err := forgejogw.NewClient(
		forgejogw.StaticTokenSource(forgejogw.Token{
			Type:  tokenType,
			Value: ft.AccessToken,
		}),
		&forgejogw.ClientOptions{BaseURL: f.APIURL()},
	)
	if err != nil {
		return nil, fmt.Errorf("create Forgejo client: %w", err)
	}

	return newRepository(ctx, f, rid.owner, rid.name, f.logger(), client, &repositoryOptions{
		DeleteBranchOnMerge: f.Options.DeleteBranchOnMerge,
	})
}
