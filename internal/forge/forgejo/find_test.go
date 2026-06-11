package forgejo

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.abhg.dev/gs/internal/forge"
	forgejogw "go.abhg.dev/gs/internal/gateway/forgejo"
	"go.abhg.dev/gs/internal/git"
)

func TestRepository_FindChangesByBranch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/alice/widget/pulls" {
			assert.Equal(t, "all", r.URL.Query().Get("state"))
			writeJSON(t, w, http.StatusOK, []*forgejogw.PullRequest{
				{
					Index:   1,
					Title:   "feat: first",
					State:   "open",
					HTMLURL: "https://x/alice/widget/pulls/1",
					Head:    &forgejogw.PRBranch{Ref: "feature", SHA: "abc123"},
					Base:    &forgejogw.PRBranch{Ref: "main"},
				},
				{
					Index:   2,
					Title:   "feat: other branch",
					State:   "open",
					HTMLURL: "https://x/alice/widget/pulls/2",
					Head:    &forgejogw.PRBranch{Ref: "other", SHA: "def456"},
					Base:    &forgejogw.PRBranch{Ref: "main"},
				},
			})
			return
		}
		http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	changes, err := r.FindChangesByBranch(t.Context(), "feature", forge.FindChangesOptions{})
	require.NoError(t, err)
	require.Len(t, changes, 1)
	assert.Equal(t, &PR{Number: 1}, changes[0].ID)
	assert.Equal(t, "feat: first", changes[0].Subject)
	assert.Equal(t, git.Hash("abc123"), changes[0].HeadHash)
}

func TestRepository_FindChangesByBranch_limit(t *testing.T) {
	// The limit should restrict the number of results returned.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/alice/widget/pulls" {
			writeJSON(t, w, http.StatusOK, []*forgejogw.PullRequest{
				{
					Index: 1, Title: "first",
					State: "open",
					Head:  &forgejogw.PRBranch{Ref: "feature"},
					Base:  &forgejogw.PRBranch{Ref: "main"},
				},
				{
					Index: 2, Title: "second",
					State: "open",
					Head:  &forgejogw.PRBranch{Ref: "feature"},
					Base:  &forgejogw.PRBranch{Ref: "main"},
				},
				{
					Index: 3, Title: "third",
					State: "open",
					Head:  &forgejogw.PRBranch{Ref: "feature"},
					Base:  &forgejogw.PRBranch{Ref: "main"},
				},
			})
			return
		}
		http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	changes, err := r.FindChangesByBranch(
		t.Context(),
		"feature",
		forge.FindChangesOptions{Limit: 2},
	)
	require.NoError(t, err)
	assert.Len(t, changes, 2)
}

func TestRepository_FindChangesByBranch_stateFilter(t *testing.T) {
	// The state option should be translated to Forgejo's state query param.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/alice/widget/pulls" {
			assert.Equal(t, "open", r.URL.Query().Get("state"))
			writeJSON(t, w, http.StatusOK, []*forgejogw.PullRequest{
				{
					Index: 5, Title: "open pr",
					State: "open",
					Head:  &forgejogw.PRBranch{Ref: "feature"},
					Base:  &forgejogw.PRBranch{Ref: "main"},
				},
			})
			return
		}
		http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	changes, err := r.FindChangesByBranch(
		t.Context(),
		"feature",
		forge.FindChangesOptions{State: forge.ChangeOpen},
	)
	require.NoError(t, err)
	assert.Len(t, changes, 1)
}

func TestRepository_FindChangesByBranch_draftRoundtrip(t *testing.T) {
	// A draft PR (WIP: prefix) should have Draft=true and Subject without prefix.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/alice/widget/pulls" {
			writeJSON(t, w, http.StatusOK, []*forgejogw.PullRequest{
				{
					Index:   7,
					Title:   "WIP: my draft",
					State:   "open",
					HTMLURL: "https://x/alice/widget/pulls/7",
					Head:    &forgejogw.PRBranch{Ref: "feature", SHA: "abc"},
					Base:    &forgejogw.PRBranch{Ref: "main"},
				},
			})
			return
		}
		http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	changes, err := r.FindChangesByBranch(t.Context(), "feature", forge.FindChangesOptions{})
	require.NoError(t, err)
	require.Len(t, changes, 1)
	assert.True(t, changes[0].Draft)
	assert.Equal(t, "my draft", changes[0].Subject)
}

func TestRepository_FindChangeByID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/alice/widget/pulls/7" {
			writeJSON(t, w, http.StatusOK, &forgejogw.PullRequest{
				Index:   7,
				Title:   "feat: thing",
				State:   "open",
				HTMLURL: "https://x/alice/widget/pulls/7",
				Head:    &forgejogw.PRBranch{Ref: "feature", SHA: "abc123"},
				Base:    &forgejogw.PRBranch{Ref: "main"},
			})
			return
		}
		http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	item, err := r.FindChangeByID(t.Context(), &PR{Number: 7})
	require.NoError(t, err)
	assert.Equal(t, &PR{Number: 7}, item.ID)
	assert.Equal(t, "feat: thing", item.Subject)
	assert.Equal(t, forge.ChangeOpen, item.State)
	assert.Equal(t, git.Hash("abc123"), item.HeadHash)
	assert.Equal(t, "main", item.BaseName)
}

func TestRepository_FindChangeByID_notFound(t *testing.T) {
	// A 404 from Forgejo should map to forge.ErrNotFound.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	_, err := r.FindChangeByID(t.Context(), &PR{Number: 999})
	require.Error(t, err)
	assert.ErrorIs(t, err, forge.ErrNotFound)
}

func TestRepository_ChangeStatuses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/repos/alice/widget/pulls/1":
			writeJSON(t, w, http.StatusOK, &forgejogw.PullRequest{
				Index:  1,
				State:  "open",
				Merged: false,
				Head:   &forgejogw.PRBranch{SHA: "abc123"},
			})
		case "/api/v1/repos/alice/widget/pulls/2":
			writeJSON(t, w, http.StatusOK, &forgejogw.PullRequest{
				Index:  2,
				State:  "closed",
				Merged: true,
				Head:   &forgejogw.PRBranch{SHA: "def456"},
			})
		default:
			http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	statuses, err := r.ChangeStatuses(t.Context(), []forge.ChangeID{
		&PR{Number: 1},
		&PR{Number: 2},
	})
	require.NoError(t, err)
	require.Len(t, statuses, 2)
	assert.Equal(t, forge.ChangeOpen, statuses[0].State)
	assert.Equal(t, git.Hash("abc123"), statuses[0].HeadHash)
	assert.Equal(t, forge.ChangeMerged, statuses[1].State)
	assert.Equal(t, git.Hash("def456"), statuses[1].HeadHash)
}

func TestRepository_ChangeStatuses_nilHead(t *testing.T) {
	// A PR with nil Head must not panic; HeadHash must be empty.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/repos/alice/widget/pulls/5":
			writeJSON(t, w, http.StatusOK, &forgejogw.PullRequest{
				Index:  5,
				State:  "open",
				Merged: false,
				Head:   nil,
			})
		default:
			http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	statuses, err := r.ChangeStatuses(t.Context(), []forge.ChangeID{&PR{Number: 5}})
	require.NoError(t, err)
	require.Len(t, statuses, 1)
	assert.Equal(t, forge.ChangeOpen, statuses[0].State)
	assert.Empty(t, statuses[0].HeadHash)
}

func TestRepository_FindChangesByBranch_pushRepositoryFilter(t *testing.T) {
	// Only PRs whose head repo owner matches PushRepository's owner are returned.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/alice/widget/pulls" {
			writeJSON(t, w, http.StatusOK, []*forgejogw.PullRequest{
				{
					Index: 10,
					Title: "matching fork",
					State: "open",
					Head: &forgejogw.PRBranch{
						Ref:  "feature",
						SHA:  "abc",
						Repo: &forgejogw.Repository{Owner: &forgejogw.User{UserName: "bob"}},
					},
					Base: &forgejogw.PRBranch{Ref: "main"},
				},
				{
					Index: 11,
					Title: "wrong owner",
					State: "open",
					Head: &forgejogw.PRBranch{
						Ref:  "feature",
						SHA:  "def",
						Repo: &forgejogw.Repository{Owner: &forgejogw.User{UserName: "carol"}},
					},
					Base: &forgejogw.PRBranch{Ref: "main"},
				},
			})
			return
		}
		http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	changes, err := r.FindChangesByBranch(
		t.Context(),
		"feature",
		forge.FindChangesOptions{
			PushRepository: &RepositoryID{owner: "bob", name: "widget"},
		},
	)
	require.NoError(t, err)
	require.Len(t, changes, 1)
	assert.Equal(t, &PR{Number: 10}, changes[0].ID)
}

func TestRepository_FindChangesByBranch_pagination(t *testing.T) {
	// Matches are collected across multiple pages when the first page
	// returns non-matching PRs and the match is on the second page.
	const pageSize = 30
	var requestCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/alice/widget/pulls" {
			requestCount++
			page := r.URL.Query().Get("page")
			switch page {
			case "", "1":
				// First page: full page of non-matching PRs.
				prs := make([]*forgejogw.PullRequest, pageSize)
				for i := range prs {
					prs[i] = &forgejogw.PullRequest{
						Index: int64(100 + i),
						Title: "unrelated",
						State: "open",
						Head:  &forgejogw.PRBranch{Ref: "other"},
						Base:  &forgejogw.PRBranch{Ref: "main"},
					}
				}
				writeJSON(t, w, http.StatusOK, prs)
			case "2":
				// Second page: contains the matching PR.
				writeJSON(t, w, http.StatusOK, []*forgejogw.PullRequest{
					{
						Index: 42,
						Title: "feat: found on page 2",
						State: "open",
						Head:  &forgejogw.PRBranch{Ref: "feature", SHA: "cafebabe"},
						Base:  &forgejogw.PRBranch{Ref: "main"},
					},
				})
			default:
				writeJSON(t, w, http.StatusOK, []*forgejogw.PullRequest{})
			}
			return
		}
		http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	changes, err := r.FindChangesByBranch(t.Context(), "feature", forge.FindChangesOptions{})
	require.NoError(t, err)
	require.Len(t, changes, 1)
	assert.Equal(t, &PR{Number: 42}, changes[0].ID)
	assert.Greater(t, requestCount, 1, "must have fetched more than one page")
}

func TestRepository_FindChangesByBranch_withReviewersAndAssignees(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/alice/widget/pulls" {
			writeJSON(t, w, http.StatusOK, []*forgejogw.PullRequest{
				{
					Index: 3,
					Title: "feat: with people",
					State: "open",
					Head:  &forgejogw.PRBranch{Ref: "feature"},
					Base:  &forgejogw.PRBranch{Ref: "main"},
					RequestedReviewers: []*forgejogw.User{
						{UserName: "bob"},
					},
					Assignees: []*forgejogw.User{
						{UserName: "carol"},
					},
					Labels: []*forgejogw.Label{
						{Name: "bug"},
					},
				},
			})
			return
		}
		http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	changes, err := r.FindChangesByBranch(t.Context(), "feature", forge.FindChangesOptions{})
	require.NoError(t, err)
	require.Len(t, changes, 1)
	assert.Equal(t, []string{"bob"}, changes[0].Reviewers)
	assert.Equal(t, []string{"carol"}, changes[0].Assignees)
	assert.Equal(t, []string{"bug"}, changes[0].Labels)
}
