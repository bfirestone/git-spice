package forgejo

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.abhg.dev/gs/internal/secret"
	"go.abhg.dev/gs/internal/silog"
	"go.abhg.dev/gs/internal/ui"
	"go.abhg.dev/testing/stub"
	"golang.org/x/oauth2"
)

func TestForgeOAuth2Endpoint(t *testing.T) {
	t.Run("DefaultURL", func(t *testing.T) {
		var f Forge

		ep, err := f.oauth2Endpoint()
		require.NoError(t, err)

		assert.Equal(t,
			"https://codeberg.org/login/oauth/authorize",
			ep.AuthURL,
		)
		assert.Equal(t,
			"https://codeberg.org/login/oauth/access_token",
			ep.TokenURL,
		)
		assert.Equal(t,
			"https://codeberg.org/login/oauth/device",
			ep.DeviceAuthURL,
		)
	})

	t.Run("CustomURL", func(t *testing.T) {
		f := Forge{
			Options: Options{
				URL: "https://forgejo.example.com",
			},
		}

		ep, err := f.oauth2Endpoint()
		require.NoError(t, err)

		assert.Equal(t,
			"https://forgejo.example.com/login/oauth/authorize",
			ep.AuthURL,
		)
	})

	t.Run("BadURL", func(t *testing.T) {
		f := Forge{
			Options: Options{
				URL: "://",
			},
		}

		_, err := f.oauth2Endpoint()
		require.Error(t, err)
		assert.ErrorContains(t, err, "bad Forgejo URL")
	})
}

func TestAuthSaveAndLoad(t *testing.T) {
	var logBuffer bytes.Buffer
	f := Forge{
		Log: silog.New(&logBuffer, nil),
	}

	var stash secret.MemoryStash
	t.Run("DoesNotExist", func(t *testing.T) {
		_, err := f.LoadAuthenticationToken(&stash)
		require.Error(t, err)
		assert.ErrorIs(t, err, secret.ErrNotFound)
	})

	t.Run("PAT", func(t *testing.T) {
		require.NoError(t, f.SaveAuthenticationToken(&stash, &AuthenticationToken{
			AccessToken: "pat-token",
			AuthType:    AuthTypePAT,
		}))

		tok, err := f.LoadAuthenticationToken(&stash)
		require.NoError(t, err)
		assert.Equal(t, &AuthenticationToken{
			AccessToken: "pat-token",
			AuthType:    AuthTypePAT,
		}, tok)
	})

	t.Run("OAuth2", func(t *testing.T) {
		require.NoError(t, f.SaveAuthenticationToken(&stash, &AuthenticationToken{
			AccessToken: "oauth-token",
			AuthType:    AuthTypeOAuth2,
		}))

		tok, err := f.LoadAuthenticationToken(&stash)
		require.NoError(t, err)
		assert.Equal(t, &AuthenticationToken{
			AccessToken: "oauth-token",
			AuthType:    AuthTypeOAuth2,
		}, tok)
	})

	t.Run("Clear", func(t *testing.T) {
		require.NoError(t, f.ClearAuthenticationToken(&stash))

		_, err := f.LoadAuthenticationToken(&stash)
		require.Error(t, err)
		assert.ErrorIs(t, err, secret.ErrNotFound)
	})
}

func TestAuth_envToken(t *testing.T) {
	var logBuffer bytes.Buffer
	f := Forge{
		Options: Options{
			Token: "env-token",
		},
		Log: silog.New(&logBuffer, nil),
	}

	t.Run("AuthenticationFlow", func(t *testing.T) {
		view := ui.NewFileView(io.Discard)
		_, err := f.AuthenticationFlow(t.Context(), view)
		require.Error(t, err)
		assert.ErrorContains(t, err, "already authenticated")
		assert.Contains(t, logBuffer.String(), "Already authenticated")
	})

	t.Run("LoadReturnsEnvToken", func(t *testing.T) {
		var stash secret.MemoryStash
		tok, err := f.LoadAuthenticationToken(&stash)
		require.NoError(t, err)

		ft := tok.(*AuthenticationToken)
		assert.Equal(t, "env-token", ft.AccessToken)
		assert.Equal(t, AuthTypePAT, ft.AuthType)
	})

	t.Run("SaveSkipsStash", func(t *testing.T) {
		var stash secret.MemoryStash
		tok := &AuthenticationToken{
			AccessToken: "env-token",
			AuthType:    AuthTypePAT,
		}
		require.NoError(t, f.SaveAuthenticationToken(&stash, tok))

		// Nothing was written to the stash.
		_, err := stash.LoadSecret(f.URL(), "token")
		require.Error(t, err)
		assert.ErrorIs(t, err, secret.ErrNotFound)
	})

	t.Run("ClearDeletesStash", func(t *testing.T) {
		// Even when FORGEJO_TOKEN is set,
		// ClearAuthenticationToken must delete any stashed credential.
		var stash secret.MemoryStash
		nonEnvForge := Forge{Log: silog.Nop()}
		require.NoError(t, nonEnvForge.SaveAuthenticationToken(&stash,
			&AuthenticationToken{
				AccessToken: "stashed-token",
				AuthType:    AuthTypePAT,
			},
		))

		// Clearing with the env-token forge must delete the stash entry.
		require.NoError(t, f.ClearAuthenticationToken(&stash))

		// The stashed token must now be absent.
		_, err := stash.LoadSecret(f.URL(), "token")
		require.Error(t, err)
		assert.ErrorIs(t, err, secret.ErrNotFound)
	})
}

func TestLoadAuthenticationToken_badJSON(t *testing.T) {
	f := Forge{
		Log: silog.Nop(),
	}

	var stash secret.MemoryStash
	require.NoError(t, stash.SaveSecret(f.URL(), "token", "not valid JSON"))

	_, err := f.LoadAuthenticationToken(&stash)
	require.Error(t, err)
	assert.ErrorContains(t, err, "unmarshal token")
}

func TestAuthType(t *testing.T) {
	for _, typ := range []AuthType{AuthTypePAT, AuthTypeOAuth2, AuthTypeCLI} {
		t.Run(typ.String(), func(t *testing.T) {
			t.Run("JSONRoundTrip", func(t *testing.T) {
				bs, err := json.Marshal(typ)
				require.NoError(t, err)

				var got AuthType
				require.NoError(t, json.Unmarshal(bs, &got))

				assert.Equal(t, typ, got)
			})
		})
	}

	t.Run("JSONError", func(t *testing.T) {
		t.Run("Unknown", func(t *testing.T) {
			_, err := json.Marshal(AuthType(42))
			require.Error(t, err)

			var got AuthType
			require.Error(t, json.Unmarshal([]byte(`"foo"`), &got))
		})
	})

	t.Run("String", func(t *testing.T) {
		tests := []struct {
			name string
			typ  AuthType
			str  string
		}{
			{"PAT", AuthTypePAT, "Personal Access Token"},
			{"OAuth2", AuthTypeOAuth2, "OAuth2"},
			{"CLI", AuthTypeCLI, "tea CLI"},
			{"Unknown", AuthType(42), "AuthType(42)"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				assert.Equal(t, tt.str, tt.typ.String())
			})
		}
	})
}

func TestSelectAuthenticator_methods(t *testing.T) {
	t.Run("PATOnly", func(t *testing.T) {
		// No ClientID, no tea binary -> PAT only.
		t.Cleanup(stub.Func(&_execLookPath, "", assert.AnError))

		methods := buildAuthMethods(authenticatorOptions{
			Endpoint:    oauth2.Endpoint{},
			ClientID:    "",
			InstanceURL: DefaultURL,
		})
		require.Len(t, methods, 1)
		assert.Equal(t, "Personal Access Token", methods[0].Title)
	})

	t.Run("WithOAuth2", func(t *testing.T) {
		// ClientID set -> OAuth2 + PAT.
		t.Cleanup(stub.Func(&_execLookPath, "", assert.AnError))

		methods := buildAuthMethods(authenticatorOptions{
			Endpoint:    oauth2.Endpoint{},
			ClientID:    "client-id",
			InstanceURL: DefaultURL,
		})
		require.Len(t, methods, 2)
		assert.Equal(t, "OAuth2", methods[0].Title)
		assert.Equal(t, "Personal Access Token", methods[1].Title)
	})

	t.Run("WithTeaCLI", func(t *testing.T) {
		// tea binary present -> PAT + tea CLI.
		t.Cleanup(stub.Func(&_execLookPath, "/usr/bin/tea", nil))

		methods := buildAuthMethods(authenticatorOptions{
			Endpoint:    oauth2.Endpoint{},
			ClientID:    "",
			InstanceURL: DefaultURL,
		})
		require.Len(t, methods, 2)
		titles := []string{methods[0].Title, methods[1].Title}
		assert.Contains(t, titles, "Personal Access Token")
		assert.Contains(t, titles, "tea CLI")
	})

	t.Run("WithAll", func(t *testing.T) {
		// ClientID + tea -> OAuth2 + PAT + tea CLI.
		t.Cleanup(stub.Func(&_execLookPath, "/usr/bin/tea", nil))

		methods := buildAuthMethods(authenticatorOptions{
			Endpoint:    oauth2.Endpoint{},
			ClientID:    "client-id",
			InstanceURL: DefaultURL,
		})
		require.Len(t, methods, 3)
		titles := []string{
			methods[0].Title,
			methods[1].Title,
			methods[2].Title,
		}
		assert.Contains(t, titles, "OAuth2")
		assert.Contains(t, titles, "Personal Access Token")
		assert.Contains(t, titles, "tea CLI")
	})
}

func TestDeviceFlowAuthenticator(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /login/oauth/device",
		func(w http.ResponseWriter, r *http.Request) {
			clientID := r.FormValue("client_id")
			if !assert.Equal(t, "client-id", clientID) {
				http.Error(w, "bad client_id", http.StatusBadRequest)
				return
			}

			scope := r.FormValue("scope")
			if !assert.Equal(t, "scope", scope) {
				http.Error(w, "bad scope", http.StatusBadRequest)
				return
			}

			_, _ = w.Write([]byte(`{
				"device_code": "device-code",
				"verification_uri": "https://example.com/verify",
				"expires_in": 900,
				"interval": 1
			}`))
		},
	)

	mux.HandleFunc("POST /login/oauth/access_token",
		func(w http.ResponseWriter, r *http.Request) {
			clientID := r.FormValue("client_id")
			if !assert.Equal(t, "client-id", clientID) {
				http.Error(w, "bad client_id", http.StatusBadRequest)
				return
			}

			deviceCode := r.FormValue("device_code")
			if !assert.Equal(t, "device-code", deviceCode) {
				http.Error(w, "bad device_code", http.StatusBadRequest)
				return
			}

			result := map[string]string{
				"access_token": "my-token",
				"token_type":   "bearer",
				"scope":        "scope",
			}

			switch r.Header.Get("Accept") {
			case "application/json":
				_ = json.NewEncoder(w).Encode(result)
			default:
				q := make(url.Values)
				for k, v := range result {
					q.Set(k, v)
				}
				_, _ = io.WriteString(w, q.Encode())
			}
		},
	)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	tok, err := (&DeviceFlowAuthenticator{
		ClientID: "client-id",
		Scopes:   []string{"scope"},
		Endpoint: oauth2.Endpoint{
			DeviceAuthURL: srv.URL + "/login/oauth/device",
			TokenURL:      srv.URL + "/login/oauth/access_token",
		},
	}).Authenticate(t.Context(), ui.NewFileView(io.Discard))
	require.NoError(t, err)

	assert.Equal(t, &AuthenticationToken{
		AccessToken: "my-token",
		AuthType:    AuthTypeOAuth2,
	}, tok)
}

func TestTeaCLIAuthenticator(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		dir := t.TempDir()
		configFile := filepath.Join(dir, "config.yml")
		require.NoError(t, os.WriteFile(configFile, []byte(`logins:
  - url: https://codeberg.org
    token: tea-token
`), 0o600))

		auth := &teaCLIAuthenticator{
			InstanceURL: "https://codeberg.org",
			ConfigPath:  configFile,
		}

		tok, err := auth.Authenticate(t.Context(), ui.NewFileView(io.Discard))
		require.NoError(t, err)
		assert.Equal(t, &AuthenticationToken{
			AccessToken: "tea-token",
			AuthType:    AuthTypeCLI,
		}, tok)
	})

	t.Run("NoMatchingLogin", func(t *testing.T) {
		dir := t.TempDir()
		configFile := filepath.Join(dir, "config.yml")
		require.NoError(t, os.WriteFile(configFile, []byte(`logins:
  - url: https://other.example.com
    token: other-token
`), 0o600))

		auth := &teaCLIAuthenticator{
			InstanceURL: "https://codeberg.org",
			ConfigPath:  configFile,
		}

		_, err := auth.Authenticate(t.Context(), ui.NewFileView(io.Discard))
		require.Error(t, err)
		assert.ErrorContains(t, err, "no tea login found")
	})

	t.Run("MissingConfigFile", func(t *testing.T) {
		auth := &teaCLIAuthenticator{
			InstanceURL: "https://codeberg.org",
			ConfigPath:  "/nonexistent/path/config.yml",
		}

		_, err := auth.Authenticate(t.Context(), ui.NewFileView(io.Discard))
		require.Error(t, err)
		assert.ErrorContains(t, err, "read tea config")
	})
}
