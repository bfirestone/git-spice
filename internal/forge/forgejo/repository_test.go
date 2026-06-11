package forgejo

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	forgejogw "go.abhg.dev/gs/internal/gateway/forgejo"
	"go.abhg.dev/gs/internal/silog"
)

func TestNewRepository(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/repos/alice/widget":
			writeJSON(t, w, http.StatusOK, &forgejogw.Repository{
				ID:                1,
				Name:              "widget",
				FullName:          "alice/widget",
				DefaultMergeStyle: "squash",
			})
		case "/api/v1/user":
			writeJSON(t, w, http.StatusOK, &forgejogw.User{
				ID:       42,
				UserName: "alice",
			})
		default:
			http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	client, err := forgejogw.NewClient(
		forgejogw.StaticTokenSource(forgejogw.Token{
			Type:  forgejogw.TokenTypeAccessToken,
			Value: "test-token",
		}),
		&forgejogw.ClientOptions{BaseURL: srv.URL},
	)
	require.NoError(t, err)

	repo, err := newRepository(
		t.Context(),
		&Forge{},
		"alice", "widget",
		silog.Nop(),
		client,
		nil,
	)
	require.NoError(t, err)

	assert.Equal(t, "alice", repo.owner)
	assert.Equal(t, "widget", repo.repo)
	assert.Equal(t, "squash", repo.defaultMergeStyle)
	assert.Equal(t, int64(42), repo.userID)
}

func TestNewRepository_emptyMergeStyle(t *testing.T) {
	// When the API returns an empty default_merge_style,
	// newRepository should fall back to "merge".
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/repos/bob/lib":
			writeJSON(t, w, http.StatusOK, &forgejogw.Repository{
				ID:                2,
				Name:              "lib",
				FullName:          "bob/lib",
				DefaultMergeStyle: "", // empty
			})
		case "/api/v1/user":
			writeJSON(t, w, http.StatusOK, &forgejogw.User{
				ID:       7,
				UserName: "bob",
			})
		default:
			http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	client, err := forgejogw.NewClient(
		forgejogw.StaticTokenSource(forgejogw.Token{
			Type:  forgejogw.TokenTypeAccessToken,
			Value: "test-token",
		}),
		&forgejogw.ClientOptions{BaseURL: srv.URL},
	)
	require.NoError(t, err)

	repo, err := newRepository(
		t.Context(),
		&Forge{},
		"bob", "lib",
		silog.Nop(),
		client,
		nil,
	)
	require.NoError(t, err)

	assert.Equal(t, "merge", repo.defaultMergeStyle)
}

func TestNewRepository_deleteBranchOnMerge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/repos/alice/widget":
			writeJSON(t, w, http.StatusOK, &forgejogw.Repository{
				ID:       1,
				FullName: "alice/widget",
			})
		case "/api/v1/user":
			writeJSON(t, w, http.StatusOK, &forgejogw.User{ID: 1})
		default:
			http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	client, err := forgejogw.NewClient(
		forgejogw.StaticTokenSource(forgejogw.Token{
			Type:  forgejogw.TokenTypeAccessToken,
			Value: "test-token",
		}),
		&forgejogw.ClientOptions{BaseURL: srv.URL},
	)
	require.NoError(t, err)

	repo, err := newRepository(
		t.Context(),
		&Forge{},
		"alice", "widget",
		silog.Nop(),
		client,
		&repositoryOptions{DeleteBranchOnMerge: true},
	)
	require.NoError(t, err)

	assert.True(t, repo.deleteBranchOnMerge)
}

func TestRepository_Forge(t *testing.T) {
	f := &Forge{}
	r := &Repository{forge: f}
	assert.Equal(t, f, r.Forge())
}
