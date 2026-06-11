package forgejo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"go.abhg.dev/gs/internal/forge"
	"go.abhg.dev/gs/internal/secret"
	"go.abhg.dev/gs/internal/text"
	"go.abhg.dev/gs/internal/ui"
	"go.abhg.dev/gs/internal/xec"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v3"
)

// AuthType identifies how a Forgejo token was obtained
// and which HTTP authentication scheme it requires.
type AuthType int

const (
	// AuthTypePAT is a personal access token
	// entered directly by the user.
	AuthTypePAT AuthType = iota

	// AuthTypeOAuth2 is a token obtained via the OAuth2 device flow.
	AuthTypeOAuth2

	// AuthTypeCLI is a token sourced from the tea CLI configuration.
	AuthTypeCLI
)

// MarshalText implements encoding.TextMarshaler.
func (t AuthType) MarshalText() ([]byte, error) {
	switch t {
	case AuthTypePAT:
		return []byte("pat"), nil
	case AuthTypeOAuth2:
		return []byte("oauth2"), nil
	case AuthTypeCLI:
		return []byte("cli"), nil
	default:
		return nil, fmt.Errorf("unknown auth type: %d", int(t))
	}
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (t *AuthType) UnmarshalText(b []byte) error {
	switch string(b) {
	case "pat":
		*t = AuthTypePAT
	case "oauth2":
		*t = AuthTypeOAuth2
	case "cli":
		*t = AuthTypeCLI
	default:
		return fmt.Errorf("unknown auth type: %q", b)
	}
	return nil
}

// String returns the string representation of the AuthType.
func (t AuthType) String() string {
	switch t {
	case AuthTypePAT:
		return "Personal Access Token"
	case AuthTypeOAuth2:
		return "OAuth2"
	case AuthTypeCLI:
		return "tea CLI"
	default:
		return fmt.Sprintf("AuthType(%d)", int(t))
	}
}

// AuthenticationToken is a Forgejo access token
// with the auth scheme it requires.
type AuthenticationToken struct {
	forge.AuthenticationToken

	// AuthType records how the token was obtained.
	AuthType AuthType `json:"auth_type,omitempty"` // required

	// AccessToken is the secret token value.
	AccessToken string `json:"access_token,omitempty"`
}

var _ forge.AuthenticationToken = (*AuthenticationToken)(nil)

func (f *Forge) oauth2Endpoint() (oauth2.Endpoint, error) {
	u, err := url.Parse(f.URL())
	if err != nil {
		return oauth2.Endpoint{}, fmt.Errorf("bad Forgejo URL: %w", err)
	}

	return oauth2.Endpoint{
		AuthURL:       u.JoinPath("/login/oauth/authorize").String(),
		TokenURL:      u.JoinPath("/login/oauth/access_token").String(),
		DeviceAuthURL: u.JoinPath("/login/oauth/device").String(),
	}, nil
}

// AuthenticationFlow prompts the user to authenticate with Forgejo.
// This rejects the request if the user is already authenticated
// with a FORGEJO_TOKEN environment variable.
func (f *Forge) AuthenticationFlow(
	ctx context.Context,
	view ui.View,
) (forge.AuthenticationToken, error) {
	log := f.logger()
	// Already authenticated with FORGEJO_TOKEN.
	if f.Options.Token != "" {
		log.Error("Already authenticated with FORGEJO_TOKEN.")
		log.Error("Unset FORGEJO_TOKEN to login with a different method.")
		return nil, errors.New("already authenticated")
	}

	oauthEndpoint, err := f.oauth2Endpoint()
	if err != nil {
		return nil, fmt.Errorf("get OAuth endpoint: %w", err)
	}

	auth, err := selectAuthenticator(view, authenticatorOptions{
		Endpoint:    oauthEndpoint,
		ClientID:    f.Options.ClientID,
		InstanceURL: f.URL(),
	})
	if err != nil {
		return nil, fmt.Errorf("select authenticator: %w", err)
	}

	return auth.Authenticate(ctx, view)
}

// SaveAuthenticationToken saves the given authentication token to the stash.
func (f *Forge) SaveAuthenticationToken(
	stash secret.Stash,
	t forge.AuthenticationToken,
) error {
	ft := t.(*AuthenticationToken)
	if f.Options.Token != "" && f.Options.Token == ft.AccessToken {
		// Token came from the environment; don't save it.
		return nil
	}

	bs, err := json.Marshal(ft)
	if err != nil {
		return fmt.Errorf("marshal token: %w", err)
	}

	f.logger().Debug("Saving authentication token to local secret storage")
	return stash.SaveSecret(f.URL(), "token", string(bs))
}

// LoadAuthenticationToken loads the authentication token from the stash.
// If the user has set FORGEJO_TOKEN, it will be used instead.
func (f *Forge) LoadAuthenticationToken(
	stash secret.Stash,
) (forge.AuthenticationToken, error) {
	if f.Options.Token != "" {
		return &AuthenticationToken{
			AccessToken: f.Options.Token,
			AuthType:    AuthTypePAT,
		}, nil
	}

	tokstr, err := stash.LoadSecret(f.URL(), "token")
	if err != nil {
		return nil, fmt.Errorf("load token: %w", err)
	}

	var tok AuthenticationToken
	if err := json.Unmarshal([]byte(tokstr), &tok); err != nil {
		return nil, fmt.Errorf("unmarshal token: %w", err)
	}

	return &tok, nil
}

// ClearAuthenticationToken removes the authentication token from the stash.
func (f *Forge) ClearAuthenticationToken(stash secret.Stash) error {
	f.logger().Debug("Clearing authentication token from local secret storage")
	return stash.DeleteSecret(f.URL(), "token")
}

type authenticator interface {
	Authenticate(context.Context, ui.View) (*AuthenticationToken, error)
}

var _execLookPath = xec.LookPath

// authenticatorOptions presents the user with multiple authentication methods,
// prompts them to choose one, and executes the chosen method.
type authenticatorOptions struct {
	Endpoint    oauth2.Endpoint // required
	ClientID    string
	InstanceURL string // required
}

type authMethod struct {
	Title       string
	Description func(ui.Theme, bool) string
	Value       authenticator
}

// buildAuthMethods returns the list of available authentication methods
// based on the given options.
func buildAuthMethods(a authenticatorOptions) []authMethod {
	var methods []authMethod

	// OAuth2 device flow: only if ClientID is configured.
	if a.ClientID != "" {
		methods = append(methods, authMethod{
			Title:       "OAuth2",
			Description: oauth2Desc,
			Value: &DeviceFlowAuthenticator{
				ClientID: a.ClientID,
				Endpoint: a.Endpoint,
				Scopes:   []string{"write:repository", "write:issue", "read:user"},
			},
		})
	}

	// PAT: always available.
	methods = append(methods, authMethod{
		Title:       "Personal Access Token",
		Description: patDesc,
		Value:       &PATAuthenticator{},
	})

	// tea CLI: only if the binary is found on PATH.
	if _, err := _execLookPath("tea"); err == nil {
		methods = append(methods, authMethod{
			Title:       "tea CLI",
			Description: teaDesc,
			Value: &teaCLIAuthenticator{
				InstanceURL: a.InstanceURL,
				ConfigPath:  teaDefaultConfigPath(),
			},
		})
	}

	return methods
}

func selectAuthenticator(
	view ui.View,
	a authenticatorOptions,
) (authenticator, error) {
	var items []ui.ListItem[authenticator]
	for _, m := range buildAuthMethods(a) {
		items = append(items, ui.ListItem[authenticator]{
			Title:       m.Title,
			Description: m.Description,
			Value:       m.Value,
		})
	}

	var method authenticator
	field := ui.NewList[authenticator]().
		WithTitle("Select an authentication method").
		WithItems(items...).
		WithValue(&method)
	err := ui.Run(view, field)
	return method, err
}

func oauth2Desc(_ ui.Theme, _ bool) string {
	return text.Dedent(`
	Authorize git-spice to act on your behalf from this device only.
	git-spice will get access to your repositories: public and private.
	`)
}

var (
	_urlStyle = ui.NewStyle()

	_urlStyleFocused = ui.NewStyle().
				Bold(true).
				Foreground(ui.Magenta).
				Underline(true)

	_scopeStyle = ui.NewStyle()

	_scopeStyleFocused = ui.NewStyle().
				Bold(true)
)

func patDesc(theme ui.Theme, focused bool) string {
	scopeStyle := _scopeStyle
	if focused {
		scopeStyle = _scopeStyleFocused
	}

	return text.Dedentf(`
	Enter a Personal Access Token generated from your Forgejo instance
	settings page.
	The token needs the following scopes: %[1]s.
	`,
		scopeStyle.Render(theme, "write:repository, write:issue, read:user"),
	)
}

func teaDesc(theme ui.Theme, focused bool) string {
	urlStyle := _urlStyle
	if focused {
		urlStyle = _urlStyleFocused
	}

	return text.Dedentf(`
	Re-use an existing tea CLI (%[1]s) session.
	You must be logged in with 'tea login add' for this to work.
	`, urlStyle.Render(theme, "https://gitea.com/gitea/tea"))
}

var (
	_deviceFlowURLStyle = ui.NewStyle().
				Foreground(ui.Cyan).
				Bold(true).
				Underline(true)

	_deviceFlowCodeStyle = ui.NewStyle().
				Foreground(ui.Cyan).
				Bold(true)

	_deviceFlowFaintStyle = ui.NewStyle().
				Faint(true)
)

// PATAuthenticator implements PAT authentication for Forgejo.
type PATAuthenticator struct{}

// Authenticate prompts the user for a Personal Access Token
// and returns the token if successful.
func (a *PATAuthenticator) Authenticate(
	_ context.Context,
	view ui.View,
) (*AuthenticationToken, error) {
	var token string
	err := ui.Run(view, ui.NewInput().
		WithTitle("Enter Personal Access Token").
		WithValidate(func(input string) error {
			if strings.TrimSpace(input) == "" {
				return errors.New("token is required")
			}
			return nil
		}).WithValue(&token),
	)

	return &AuthenticationToken{
		AccessToken: token,
		AuthType:    AuthTypePAT,
	}, err
}

// DeviceFlowAuthenticator implements the OAuth2 device flow for Forgejo.
type DeviceFlowAuthenticator struct {
	// Endpoint is the OAuth2 endpoint to use.
	Endpoint oauth2.Endpoint

	// ClientID for the OAuth2 application.
	ClientID string

	// Scopes specifies the OAuth2 scopes to request.
	Scopes []string
}

// Authenticate executes the OAuth2 device flow authentication.
func (a *DeviceFlowAuthenticator) Authenticate(
	ctx context.Context,
	view ui.View,
) (*AuthenticationToken, error) {
	cfg := oauth2.Config{
		ClientID:    a.ClientID,
		Endpoint:    a.Endpoint,
		Scopes:      a.Scopes,
		RedirectURL: "http://127.0.0.1/callback",
	}

	resp, err := cfg.DeviceAuth(ctx)
	if err != nil {
		return nil, fmt.Errorf("device auth: %w", err)
	}

	theme := view.Theme()
	urlStyle := _deviceFlowURLStyle.Resolve(theme)
	codeStyle := _deviceFlowCodeStyle.Resolve(theme)
	bullet := lipgloss.NewStyle().PaddingLeft(2).Foreground(theme.Gray)
	faint := _deviceFlowFaintStyle.Resolve(theme)

	lipgloss.Fprintf(view, "%s Visit %s\n",
		bullet.Render("1."), urlStyle.Render(resp.VerificationURI))
	lipgloss.Fprintf(view, "%s Enter code: %s\n",
		bullet.Render("2."), codeStyle.Render(resp.UserCode))
	lipgloss.Fprintln(view, faint.Render("The code expires in a few minutes."))
	lipgloss.Fprintln(view,
		faint.Render("It will take a few seconds to verify after you enter it."))

	token, err := cfg.DeviceAccessToken(ctx, resp,
		oauth2.SetAuthURLParam(
			"grant_type",
			"urn:ietf:params:oauth:grant-type:device_code",
		))
	if err != nil {
		return nil, fmt.Errorf("device access token: %w", err)
	}

	return &AuthenticationToken{
		AccessToken: token.AccessToken,
		AuthType:    AuthTypeOAuth2,
	}, nil
}

// teaCLIAuthenticator reuses an existing tea CLI session.
type teaCLIAuthenticator struct {
	// InstanceURL is the Forgejo instance URL to match.
	InstanceURL string // required

	// ConfigPath is the path to the tea config file.
	// Defaults to teaDefaultConfigPath() if empty.
	ConfigPath string
}

// teaConfig is the minimal shape of the tea CLI config file.
type teaConfig struct {
	Logins []teaLogin `yaml:"logins"`
}

// teaLogin represents one login entry in the tea config.
type teaLogin struct {
	URL   string `yaml:"url"`
	Token string `yaml:"token"`
}

// Authenticate reads the token for the matching login from the tea config.
func (a *teaCLIAuthenticator) Authenticate(
	_ context.Context,
	_ ui.View,
) (*AuthenticationToken, error) {
	configPath := a.ConfigPath
	if configPath == "" {
		configPath = teaDefaultConfigPath()
	}

	bs, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read tea config: %w", err)
	}

	var cfg teaConfig
	if err := yaml.Unmarshal(bs, &cfg); err != nil {
		return nil, fmt.Errorf("parse tea config: %w", err)
	}

	for _, login := range cfg.Logins {
		if login.URL == a.InstanceURL {
			return &AuthenticationToken{
				AccessToken: login.Token,
				AuthType:    AuthTypeCLI,
			}, nil
		}
	}

	return nil, fmt.Errorf("no tea login found for %s", a.InstanceURL)
}

// teaDefaultConfigPath returns the default path to the tea CLI config file.
func teaDefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home + "/.config/tea/config.yml"
}
