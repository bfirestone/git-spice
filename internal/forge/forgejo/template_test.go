package forgejo

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	forgejogw "go.abhg.dev/gs/internal/gateway/forgejo"
)

func TestRepository_ListChangeTemplates_found(t *testing.T) {
	const body = "## Summary\n\nDescribe your changes.\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(body))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/repos/alice/widget/contents/.forgejo/PULL_REQUEST_TEMPLATE.md":
			writeJSON(t, w, http.StatusOK, forgejogw.ContentsResponse{
				Name:     "PULL_REQUEST_TEMPLATE.md",
				Content:  encoded,
				Encoding: "base64",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	templates, err := r.ListChangeTemplates(t.Context())
	require.NoError(t, err)
	require.Len(t, templates, 1)
	assert.Equal(t, "PULL_REQUEST_TEMPLATE.md", templates[0].Filename)
	assert.Equal(t, body, templates[0].Body)
}

func TestRepository_ListChangeTemplates_none(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	templates, err := r.ListChangeTemplates(t.Context())
	require.NoError(t, err)
	assert.Empty(t, templates)
}

func TestRepository_ListChangeTemplates_newlineWrappedBase64(t *testing.T) {
	// Forgejo wraps base64 content with embedded newlines every 60 chars.
	// The decoder must strip them before decoding.
	const body = "## Summary\n\nDescribe your changes here.\n"
	raw := base64.StdEncoding.EncodeToString([]byte(body))

	// Wrap at 60 chars to simulate Forgejo's line-wrapped base64 output.
	var sb strings.Builder
	for i := 0; i < len(raw); i += 60 {
		end := min(i+60, len(raw))
		sb.WriteString(raw[i:end])
		sb.WriteByte('\n')
	}
	wrapped := sb.String()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/repos/alice/widget/contents/.forgejo/PULL_REQUEST_TEMPLATE.md":
			writeJSON(t, w, http.StatusOK, forgejogw.ContentsResponse{
				Name:     "PULL_REQUEST_TEMPLATE.md",
				Content:  wrapped,
				Encoding: "base64",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	templates, err := r.ListChangeTemplates(t.Context())
	require.NoError(t, err)
	require.Len(t, templates, 1)
	assert.Equal(t, "PULL_REQUEST_TEMPLATE.md", templates[0].Filename)
	assert.Equal(t, body, templates[0].Body)
}

func TestRepository_ListChangeTemplates_fallThrough(t *testing.T) {
	// Earlier candidates (e.g. .forgejo/, .gitea/) return 404.
	// Only .github/PULL_REQUEST_TEMPLATE.md exists.
	const body = "## PR Template\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(body))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/repos/alice/widget/contents/.github/PULL_REQUEST_TEMPLATE.md" {
			writeJSON(t, w, http.StatusOK, forgejogw.ContentsResponse{
				Name:     "PULL_REQUEST_TEMPLATE.md",
				Content:  encoded,
				Encoding: "base64",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	templates, err := r.ListChangeTemplates(t.Context())
	require.NoError(t, err)
	require.Len(t, templates, 1)
	assert.Equal(t, "PULL_REQUEST_TEMPLATE.md", templates[0].Filename)
	assert.Equal(t, body, templates[0].Body)
}
