package forgejo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	forgejogw "go.abhg.dev/gs/internal/gateway/forgejo"
	"go.abhg.dev/gs/internal/silog"
)

func TestRepository_ensureLabels_allExist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/alice/widget/labels" {
			writeJSON(t, w, http.StatusOK, []*forgejogw.Label{
				{ID: 1, Name: "bug"},
				{ID: 2, Name: "enhancement"},
			})
			return
		}
		http.Error(w, "unexpected request: "+r.URL.Path, http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	ids, err := r.ensureLabels(t.Context(), []string{"bug", "enhancement"})
	require.NoError(t, err)
	assert.Equal(t, []int64{1, 2}, ids)
}

func TestRepository_ensureLabels_createMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/alice/widget/labels":
			writeJSON(t, w, http.StatusOK, []*forgejogw.Label{
				{ID: 1, Name: "bug"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/alice/widget/labels":
			var body struct {
				Name string `json:"name"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "enhancement", body.Name)
			writeJSON(t, w, http.StatusCreated, &forgejogw.Label{ID: 2, Name: "enhancement"})
		default:
			http.Error(w, "unexpected request: "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	ids, err := r.ensureLabels(t.Context(), []string{"bug", "enhancement"})
	require.NoError(t, err)
	assert.Equal(t, []int64{1, 2}, ids)
}

func TestRepository_ensureLabels_createConflictRelist(t *testing.T) {
	// Simulate a concurrent create: the POST returns 409 conflict,
	// then the re-list finds the newly created label.
	var listCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/alice/widget/labels":
			n := listCount.Add(1)
			if n == 1 {
				// First list: only "bug" exists.
				writeJSON(t, w, http.StatusOK, []*forgejogw.Label{
					{ID: 1, Name: "bug"},
				})
			} else {
				// Re-list after conflict: "enhancement" now exists.
				writeJSON(t, w, http.StatusOK, []*forgejogw.Label{
					{ID: 1, Name: "bug"},
					{ID: 2, Name: "enhancement"},
				})
			}
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/alice/widget/labels":
			// Conflict: another request already created this label.
			writeJSON(t, w, http.StatusConflict, map[string]any{
				"message": "label already exists",
			})
		default:
			http.Error(w, "unexpected request: "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	ids, err := r.ensureLabels(t.Context(), []string{"bug", "enhancement"})
	require.NoError(t, err)
	assert.Equal(t, []int64{1, 2}, ids)
}

func TestRepository_ensureLabels_empty(t *testing.T) {
	r := newTestRepository(t, nil, "alice", "widget")
	ids, err := r.ensureLabels(t.Context(), nil)
	require.NoError(t, err)
	assert.Empty(t, ids)
}

// newTestRepository builds a minimal Repository for unit tests.
// If srv is nil, the repository's client will have no server
// (only safe when no network calls are expected).
func newTestRepository(t *testing.T, srv *httptest.Server, owner, repo string) *Repository {
	t.Helper()

	var baseURL string
	if srv != nil {
		baseURL = srv.URL
	} else {
		baseURL = "http://localhost:0"
	}

	client, err := forgejogw.NewClient(
		forgejogw.StaticTokenSource(forgejogw.Token{
			Type:  forgejogw.TokenTypeAccessToken,
			Value: "test-token",
		}),
		&forgejogw.ClientOptions{BaseURL: baseURL},
	)
	require.NoError(t, err)

	return &Repository{
		client: client,
		owner:  owner,
		repo:   repo,
		log:    silog.Nop(),
		forge:  &Forge{},
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, code int, v any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	require.NoError(t, json.NewEncoder(w).Encode(v))
}
