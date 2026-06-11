// Package forgejo provides a narrow Forgejo REST client
// for the endpoints git-spice uses.
package forgejo

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	_userAgent      = "git-spice"
	_apiVersionPath = "api/v1/"
	_defaultURL     = "https://codeberg.org"
)

// ErrNotFound reports a Forgejo 404 response.
//
// Callers use this sentinel to translate Forgejo's
// "not found" response into forge-level not-found handling
// without inspecting raw HTTP state.
var ErrNotFound = errors.New("404 Not Found")

// TokenType identifies how a token is attached to a Forgejo request.
type TokenType int

const (
	// TokenTypeAccessToken sends the token
	// as an `Authorization: token` header
	// (personal access tokens).
	TokenTypeAccessToken TokenType = iota

	// TokenTypeBearer sends the token
	// as an `Authorization: Bearer` header (OAuth2 tokens).
	TokenTypeBearer
)

// Token describes a Forgejo credential.
type Token struct {
	Type  TokenType
	Value string
}

// TokenSource provides tokens for Forgejo API requests.
type TokenSource interface {
	Token(context.Context) (Token, error)
}

// StaticTokenSource returns the same token on every request.
type StaticTokenSource Token

// Token implements [TokenSource].
func (s StaticTokenSource) Token(context.Context) (Token, error) {
	return Token(s), nil
}

// Client is a Forgejo REST client
// specialized to the endpoints that git-spice actually uses.
//
// It is intentionally not a general-purpose Forgejo client.
type Client struct {
	httpClient *http.Client

	// baseURL is the normalized API root, always ending in `/api/v1/`.
	baseURL string

	// authHeader supplies the per-request authentication header.
	authHeader authHeaderFunc
}

// ClientOptions configures a Forgejo REST client.
type ClientOptions struct {
	// BaseURL is the Forgejo host URL or API root URL.
	//
	// If empty, the client uses Codeberg at `https://codeberg.org`.
	// The value may be either a host URL or an explicit API URL.
	// It is normalized to an API root ending in `/api/v1/`.
	BaseURL string

	// HTTPClient is the HTTP client used to send requests.
	//
	// If nil, the client uses [http.DefaultClient].
	HTTPClient *http.Client
}

// NewClient builds a Forgejo REST client.
func NewClient(
	tokenSource TokenSource,
	opts *ClientOptions,
) (*Client, error) {
	if tokenSource == nil {
		return nil, errors.New("nil token source")
	}

	opts = cmp.Or(opts, &ClientOptions{})

	authHeader, err := buildAuthHeader(tokenSource)
	if err != nil {
		return nil, err
	}

	normalizedBaseURL, err := normalizeBaseURL(opts.BaseURL)
	if err != nil {
		return nil, err
	}

	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &Client{
		httpClient: httpClient,
		baseURL:    normalizedBaseURL,
		authHeader: authHeader,
	}, nil
}

// authHeaderFunc returns the authentication header for a request.
//
// Forgejo supports different authentication schemes
// depending on how the token was obtained.
// Resolving that once up front
// keeps request execution simple
// and lets Client
// treat authentication as a single injected capability.
type authHeaderFunc func(context.Context) (http.Header, error)

// buildAuthHeader selects the Forgejo authentication scheme
// that matches the configured token source.
//
// The returned closure hides the differences
// between Forgejo personal access tokens and OAuth bearer tokens,
// so the request helpers only need to ask
// for "the auth header" at send time.
func buildAuthHeader(
	tokenSource TokenSource,
) (authHeaderFunc, error) {
	return func(ctx context.Context) (http.Header, error) {
		token, err := tokenSource.Token(ctx)
		if err != nil {
			return nil, err
		}

		header := make(http.Header, 1)
		switch token.Type {
		case TokenTypeAccessToken:
			header.Set("Authorization", "token "+token.Value)
		case TokenTypeBearer:
			header.Set("Authorization", "Bearer "+token.Value)
		default:
			return nil, fmt.Errorf(
				"no source for authentication type: %v",
				token.Type,
			)
		}

		return header, nil
	}, nil
}

// normalizeBaseURL converts user configuration
// into a canonical Forgejo API root.
//
// Inputs may be either a Forgejo host URL
// like `https://forgejo.example.com`
// or an explicit API URL
// like `https://forgejo.example.com/api/v1`.
// The returned URL is always stripped of query and fragment components
// and always ends with `/api/v1/`,
// which lets request helpers append endpoint paths directly
// without needing to reason about slashes
// or whether the caller supplied the host URL
// or the API URL.
func normalizeBaseURL(baseURL string) (string, error) {
	if baseURL == "" {
		baseURL = _defaultURL
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse base URL: %w", err)
	}

	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid base URL %q", baseURL)
	}

	path := strings.TrimPrefix(u.Path, "/")
	switch {
	case path == "":
		u.Path = "/" + _apiVersionPath
	case strings.HasSuffix(path, _apiVersionPath):
		u.Path = "/" + path
	case strings.HasSuffix(path, strings.TrimSuffix(_apiVersionPath, "/")):
		u.Path = "/" + path + "/"
	default:
		u.Path = "/" + strings.TrimSuffix(path, "/") + "/" + _apiVersionPath
	}

	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

// get sends a Forgejo GET request.
//
// `resourcePath` is the endpoint path relative to the normalized API root,
// for example `repos/owner/name/pulls`.
// `query` contains already-encoded query parameters for the request.
// `dst` is the response value to JSON-decode into.
func (c *Client) get(
	ctx context.Context,
	resourcePath string,
	query url.Values,
	dst any,
) (*Response, error) {
	return c.do(ctx, http.MethodGet, resourcePath, query, nil, dst)
}

// post sends a Forgejo POST request.
//
// `resourcePath` is the endpoint path relative to the normalized API root.
// `query` contains already-encoded query parameters for the request.
// `body` is the request payload, which will be JSON-encoded.
// `dst` is the response value to JSON-decode into.
func (c *Client) post(
	ctx context.Context,
	resourcePath string,
	query url.Values,
	body any,
	dst any,
) (*Response, error) {
	return c.do(ctx, http.MethodPost, resourcePath, query, body, dst)
}

// patch sends a Forgejo PATCH request.
//
// `resourcePath` is the endpoint path relative to the normalized API root.
// `query` contains already-encoded query parameters for the request.
// `body` is the request payload, which will be JSON-encoded.
// `dst` is the response value to JSON-decode into.
func (c *Client) patch(
	ctx context.Context,
	resourcePath string,
	query url.Values,
	body any,
	dst any,
) (*Response, error) {
	return c.do(ctx, http.MethodPatch, resourcePath, query, body, dst)
}

// delete sends a Forgejo DELETE request.
//
// `resourcePath` is the endpoint path relative to the normalized API root.
// `query` contains already-encoded query parameters for the request.
func (c *Client) delete(
	ctx context.Context,
	resourcePath string,
	query url.Values,
) (*Response, error) {
	return c.do(ctx, http.MethodDelete, resourcePath, query, nil, nil)
}

// do sends a Forgejo API request and decodes the response.
//
// `method` is the HTTP verb to use.
// `resourcePath` is the endpoint path relative to the normalized API root.
// `query` contains already-encoded query parameters for the request.
// `body` is the request payload, if any, which will be JSON-encoded.
// `dst` is the response value to decode into. If `dst` is nil, the response
// body is ignored after error handling.
func (c *Client) do(
	ctx context.Context,
	method string,
	resourcePath string,
	query url.Values,
	body any,
	dst any,
) (*Response, error) {
	reqURL := c.baseURL + resourcePath
	if len(query) > 0 {
		reqURL += "?" + query.Encode()
	}

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", _userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.authHeader != nil {
		header, err := c.authHeader(ctx)
		if err != nil {
			return nil, err
		}
		for key, values := range header {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}

	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, httpResp.Body)
		_ = httpResp.Body.Close()
	}()

	resp := newResponse(httpResp)
	if err := checkResponse(httpResp); err != nil {
		return resp, err
	}

	if dst == nil || httpResp.StatusCode == http.StatusNoContent {
		return resp, nil
	}

	if err := json.NewDecoder(httpResp.Body).Decode(dst); err != nil {
		return resp, fmt.Errorf("decode response: %w", err)
	}
	return resp, nil
}

// Response wraps the raw HTTP response metadata
// for a Forgejo API call.
type Response struct {
	Header     http.Header
	StatusCode int
}

func newResponse(resp *http.Response) *Response {
	return &Response{
		Header:     resp.Header.Clone(),
		StatusCode: resp.StatusCode,
	}
}

// APIError captures a non-success Forgejo response.
//
// `Body` preserves the raw response for debugging.
// `StatusCode` preserves the HTTP status code.
// `Message` stores the best-effort flattened error text
// extracted from Forgejo's JSON error payload.
type APIError struct {
	StatusCode int
	Message    string
	Body       []byte

	method string
	url    string
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf(
			"%s %s: %d",
			e.method,
			e.url,
			e.StatusCode,
		)
	}

	return fmt.Sprintf(
		"%s %s: %d %s",
		e.method,
		e.url,
		e.StatusCode,
		e.Message,
	)
}

// checkResponse converts Forgejo HTTP failures into package errors.
//
// `404 Not Found` is returned as the sentinel `ErrNotFound`
// so callers can treat missing resources specially.
// Other failures are converted into `APIError`,
// including a parsed best-effort message
// when Forgejo returns a structured JSON error body.
func checkResponse(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusOK,
		http.StatusCreated,
		http.StatusAccepted,
		http.StatusNoContent,
		http.StatusNotModified:
		return nil
	case http.StatusNotFound:
		return ErrNotFound
	}

	path := resp.Request.URL.RawPath
	if path == "" {
		path = resp.Request.URL.EscapedPath()
	}

	errResp := &APIError{
		StatusCode: resp.StatusCode,
		method:     resp.Request.Method,
		url: fmt.Sprintf(
			"%s://%s%s",
			resp.Request.URL.Scheme,
			resp.Request.URL.Host,
			path,
		),
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return errResp
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return errResp
	}

	errResp.Body = body

	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		errResp.Message = fmt.Sprintf(
			"parse unknown error format: %s",
			body,
		)
		return errResp
	}

	errResp.Message = parseError(raw)
	return errResp
}

// parseError flattens the range of JSON error payloads
// Forgejo returns into a single readable message.
func parseError(v any) string {
	switch v := v.(type) {
	case map[string]any:
		if msg, ok := v["message"]; ok {
			return parseError(msg)
		}

		var parts []string
		for key, value := range v {
			message := parseError(value)
			if message == "" {
				continue
			}
			parts = append(parts, key+": "+message)
		}
		return strings.Join(parts, ", ")

	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if msg := parseError(item); msg != "" {
				parts = append(parts, msg)
			}
		}
		return strings.Join(parts, ", ")

	case string:
		return v

	default:
		return fmt.Sprint(v)
	}
}
