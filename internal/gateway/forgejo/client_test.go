package forgejo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    string
		wantErr string
	}{
		{
			name:    "Empty",
			baseURL: "",
			want:    "https://codeberg.org/api/v1/",
		},
		{
			name:    "HostURL",
			baseURL: "https://forgejo.example.com",
			want:    "https://forgejo.example.com/api/v1/",
		},
		{
			name:    "HostURLTrailingSlash",
			baseURL: "https://forgejo.example.com/",
			want:    "https://forgejo.example.com/api/v1/",
		},
		{
			name:    "APIURL",
			baseURL: "https://forgejo.example.com/api/v1",
			want:    "https://forgejo.example.com/api/v1/",
		},
		{
			name:    "APIURLTrailingSlash",
			baseURL: "https://forgejo.example.com/api/v1/",
			want:    "https://forgejo.example.com/api/v1/",
		},
		{
			name:    "Subpath",
			baseURL: "https://example.com/forgejo",
			want:    "https://example.com/forgejo/api/v1/",
		},
		{
			name:    "QueryAndFragmentDropped",
			baseURL: "https://example.com/forgejo?debug=1#frag",
			want:    "https://example.com/forgejo/api/v1/",
		},
		{
			name:    "MissingSchemeFails",
			baseURL: "forgejo.example.com",
			wantErr: `invalid base URL "forgejo.example.com"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeBaseURL(tt.baseURL)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildAuthHeader(t *testing.T) {
	t.Run("UnsupportedTokenType", func(t *testing.T) {
		header, err := buildAuthHeader(
			StaticTokenSource(Token{Type: TokenType(99)}),
		)
		require.NoError(t, err)

		_, err = header(t.Context())
		require.Error(t, err)
	})
}

func TestClient_authHeader(t *testing.T) {
	tests := []struct {
		name      string
		token     Token
		wantValue string
	}{
		{
			name:      "AccessToken",
			token:     Token{Type: TokenTypeAccessToken, Value: "secret"},
			wantValue: "token secret",
		},
		{
			name:      "Bearer",
			token:     Token{Type: TokenTypeBearer, Value: "secret"},
			wantValue: "Bearer secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tt.wantValue, r.Header.Get("Authorization"))
				assert.Equal(t, "git-spice", r.Header.Get("User-Agent"))
				writeJSON(t, w, http.StatusOK, map[string]any{"id": 1})
			}))
			defer srv.Close()

			client, err := NewClient(
				StaticTokenSource(tt.token),
				&ClientOptions{BaseURL: srv.URL},
			)
			require.NoError(t, err)

			var dst map[string]any
			_, err = client.get(t.Context(), "test", nil, &dst)
			require.NoError(t, err)
		})
	}
}

func TestClient_get_notFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	var dst map[string]any
	_, err := client.get(t.Context(), "something", nil, &dst)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestClient_post(t *testing.T) {
	type requestBody struct {
		Name string `json:"name"`
	}
	type responseBody struct {
		ID int `json:"id"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assertJSONBody(t, r, `{"name":"widget"}`)
		writeJSON(t, w, http.StatusCreated, responseBody{ID: 99})
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	var dst responseBody
	resp, err := client.post(t.Context(), "items", nil, requestBody{Name: "widget"}, &dst)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, 99, dst.ID)
}

func TestClient_patch(t *testing.T) {
	type requestBody struct {
		Title string `json:"title"`
	}
	type responseBody struct {
		Title string `json:"title"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assertJSONBody(t, r, `{"title":"updated"}`)
		writeJSON(t, w, http.StatusOK, responseBody{Title: "updated"})
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	var dst responseBody
	resp, err := client.patch(t.Context(), "items/1", nil, requestBody{Title: "updated"}, &dst)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "updated", dst.Title)
}

func TestClient_delete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	resp, err := client.delete(t.Context(), "items/1", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestClient_errors(t *testing.T) {
	t.Run("APIError", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(t, w, http.StatusBadRequest, map[string]any{
				"message": "bad request",
			})
		}))
		defer srv.Close()

		client := newTestClient(t, srv)
		var dst map[string]any
		_, err := client.get(t.Context(), "items", nil, &dst)
		require.Error(t, err)

		var respErr *APIError
		require.ErrorAs(t, err, &respErr)
		assert.Equal(t, http.StatusBadRequest, respErr.StatusCode)
		assert.Equal(t, "bad request", respErr.Message)
	})
}

func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()

	client, err := NewClient(
		StaticTokenSource(Token{
			Type:  TokenTypeAccessToken,
			Value: "test-token",
		}),
		&ClientOptions{BaseURL: srv.URL},
	)
	require.NoError(t, err)
	return client
}

func writeJSON(t *testing.T, w http.ResponseWriter, code int, v any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	require.NoError(t, json.NewEncoder(w).Encode(v))
}

func assertJSONBody(t *testing.T, r *http.Request, want string) {
	t.Helper()

	var body any
	require.NoError(t, json.NewDecoder(r.Body).Decode(&body))

	got, err := json.Marshal(body)
	require.NoError(t, err)
	assert.JSONEq(t, want, string(got))
}
