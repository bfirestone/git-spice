package forgejo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.abhg.dev/gs/internal/forge"
	forgejogw "go.abhg.dev/gs/internal/gateway/forgejo"
)

func TestRepository_EditChange_noOp(t *testing.T) {
	// No-op options must produce zero HTTP requests.
	var requestCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		http.Error(w, "unexpected request: "+r.URL.Path, http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	err := r.EditChange(t.Context(), &PR{Number: 1}, forge.EditChangeOptions{})
	require.NoError(t, err)
	assert.Equal(t, int32(0), requestCount.Load())
}

func TestRepository_EditChange_baseOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPatch &&
			r.URL.Path == "/api/v1/repos/alice/widget/pulls/42":
			var body forgejogw.UpdatePullRequestOptions
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "main", *body.Base)
			writeJSON(t, w, http.StatusCreated, &forgejogw.PullRequest{
				Index: 42,
				Title: "my pr",
				Base:  &forgejogw.PRBranch{Ref: "main"},
			})
		default:
			http.Error(w, "unexpected request: "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	err := r.EditChange(t.Context(), &PR{Number: 42}, forge.EditChangeOptions{
		Base: "main",
	})
	require.NoError(t, err)
}

func TestRepository_EditChange_draftTrue(t *testing.T) {
	// Draft=true on a non-draft PR should prepend "WIP: " to the title.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet &&
			r.URL.Path == "/api/v1/repos/alice/widget/pulls/5":
			writeJSON(t, w, http.StatusOK, &forgejogw.PullRequest{
				Index: 5,
				Title: "Add feature",
			})
		case r.Method == http.MethodPatch &&
			r.URL.Path == "/api/v1/repos/alice/widget/pulls/5":
			var body forgejogw.UpdatePullRequestOptions
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "WIP: Add feature", *body.Title)
			writeJSON(t, w, http.StatusCreated, &forgejogw.PullRequest{
				Index: 5,
				Title: "WIP: Add feature",
			})
		default:
			http.Error(w, "unexpected request: "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	draft := true
	r := newTestRepository(t, srv, "alice", "widget")
	err := r.EditChange(t.Context(), &PR{Number: 5}, forge.EditChangeOptions{
		Draft: &draft,
	})
	require.NoError(t, err)
}

func TestRepository_EditChange_draftFalse(t *testing.T) {
	// Draft=false on a draft PR should strip the "WIP:" prefix.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet &&
			r.URL.Path == "/api/v1/repos/alice/widget/pulls/7":
			writeJSON(t, w, http.StatusOK, &forgejogw.PullRequest{
				Index: 7,
				Title: "WIP: Fix bug",
			})
		case r.Method == http.MethodPatch &&
			r.URL.Path == "/api/v1/repos/alice/widget/pulls/7":
			var body forgejogw.UpdatePullRequestOptions
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "Fix bug", *body.Title)
			writeJSON(t, w, http.StatusCreated, &forgejogw.PullRequest{
				Index: 7,
				Title: "Fix bug",
			})
		default:
			http.Error(w, "unexpected request: "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	draft := false
	r := newTestRepository(t, srv, "alice", "widget")
	err := r.EditChange(t.Context(), &PR{Number: 7}, forge.EditChangeOptions{
		Draft: &draft,
	})
	require.NoError(t, err)
}

func TestRepository_EditChange_draftNoChange(t *testing.T) {
	// Draft=true on an already-draft PR should not include Title in PATCH.
	// Draft=false on a non-draft PR should not include Title in PATCH.
	// In both cases, PullRequestGet is still fetched (to know the title),
	// but Title must not appear in the update body.
	t.Run("AlreadyDraft", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet &&
				r.URL.Path == "/api/v1/repos/alice/widget/pulls/10":
				writeJSON(t, w, http.StatusOK, &forgejogw.PullRequest{
					Index: 10,
					Title: "WIP: already draft",
				})
			case r.Method == http.MethodPatch &&
				r.URL.Path == "/api/v1/repos/alice/widget/pulls/10":
				// Only non-nil fields appear in JSON; Title should be absent.
				var body map[string]json.RawMessage
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.NotContains(t, body, "title")
				writeJSON(t, w, http.StatusCreated, &forgejogw.PullRequest{
					Index: 10,
					Title: "WIP: already draft",
				})
			default:
				http.Error(w, "unexpected request: "+r.URL.Path, http.StatusInternalServerError)
			}
		}))
		defer srv.Close()

		draft := true
		r := newTestRepository(t, srv, "alice", "widget")
		// Also provide base so we have something to update.
		err := r.EditChange(t.Context(), &PR{Number: 10}, forge.EditChangeOptions{
			Draft: &draft,
			Base:  "main",
		})
		require.NoError(t, err)
	})

	t.Run("AlreadyNotDraft", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet &&
				r.URL.Path == "/api/v1/repos/alice/widget/pulls/11":
				writeJSON(t, w, http.StatusOK, &forgejogw.PullRequest{
					Index: 11,
					Title: "No wip prefix",
				})
			case r.Method == http.MethodPatch &&
				r.URL.Path == "/api/v1/repos/alice/widget/pulls/11":
				var body map[string]json.RawMessage
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.NotContains(t, body, "title")
				writeJSON(t, w, http.StatusCreated, &forgejogw.PullRequest{
					Index: 11,
					Title: "No wip prefix",
				})
			default:
				http.Error(w, "unexpected request: "+r.URL.Path, http.StatusInternalServerError)
			}
		}))
		defer srv.Close()

		draft := false
		r := newTestRepository(t, srv, "alice", "widget")
		err := r.EditChange(t.Context(), &PR{Number: 11}, forge.EditChangeOptions{
			Draft: &draft,
			Base:  "develop",
		})
		require.NoError(t, err)
	})
}

func TestRepository_EditChange_addLabels(t *testing.T) {
	// AddLabels must union with the existing PR labels
	// because Forgejo's PATCH replaces the label set.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet &&
			r.URL.Path == "/api/v1/repos/alice/widget/pulls/3":
			writeJSON(t, w, http.StatusOK, &forgejogw.PullRequest{
				Index: 3,
				Title: "PR with labels",
				Labels: []*forgejogw.Label{
					{ID: 10, Name: "existing"},
				},
			})
		case r.Method == http.MethodGet &&
			r.URL.Path == "/api/v1/repos/alice/widget/labels":
			writeJSON(t, w, http.StatusOK, []*forgejogw.Label{
				{ID: 10, Name: "existing"},
				{ID: 20, Name: "new-label"},
			})
		case r.Method == http.MethodPatch &&
			r.URL.Path == "/api/v1/repos/alice/widget/pulls/3":
			var body forgejogw.UpdatePullRequestOptions
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			// Must contain both existing (10) and new (20) label IDs.
			assert.ElementsMatch(t, []int64{10, 20}, body.Labels)
			writeJSON(t, w, http.StatusCreated, &forgejogw.PullRequest{
				Index: 3,
				Title: "PR with labels",
			})
		default:
			http.Error(w, "unexpected request: "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	err := r.EditChange(t.Context(), &PR{Number: 3}, forge.EditChangeOptions{
		AddLabels: []string{"new-label"},
	})
	require.NoError(t, err)
}

func TestRepository_EditChange_addAssignees(t *testing.T) {
	// AddAssignees must union with the existing PR assignees.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet &&
			r.URL.Path == "/api/v1/repos/alice/widget/pulls/8":
			writeJSON(t, w, http.StatusOK, &forgejogw.PullRequest{
				Index: 8,
				Title: "PR with assignees",
				Assignees: []*forgejogw.User{
					{ID: 1, UserName: "alice"},
				},
			})
		case r.Method == http.MethodPatch &&
			r.URL.Path == "/api/v1/repos/alice/widget/pulls/8":
			var body forgejogw.UpdatePullRequestOptions
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			// Must contain both existing ("alice") and new ("bob") assignees.
			assert.ElementsMatch(t, []string{"alice", "bob"}, body.Assignees)
			writeJSON(t, w, http.StatusCreated, &forgejogw.PullRequest{
				Index: 8,
				Title: "PR with assignees",
			})
		default:
			http.Error(w, "unexpected request: "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	err := r.EditChange(t.Context(), &PR{Number: 8}, forge.EditChangeOptions{
		AddAssignees: []string{"bob"},
	})
	require.NoError(t, err)
}

func TestRepository_EditChange_addReviewers(t *testing.T) {
	// AddReviewers uses the separate requested_reviewers endpoint (additive).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost &&
			r.URL.Path == "/api/v1/repos/alice/widget/pulls/9/requested_reviewers":
			var body struct {
				Reviewers []string `json:"reviewers"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.ElementsMatch(t, []string{"carol", "dave"}, body.Reviewers)
			w.WriteHeader(http.StatusCreated)
		default:
			http.Error(w, "unexpected request: "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	err := r.EditChange(t.Context(), &PR{Number: 9}, forge.EditChangeOptions{
		AddReviewers: []string{"carol", "dave"},
	})
	require.NoError(t, err)
}

func TestRepository_EditChange_reviewersAndBase(t *testing.T) {
	// When both AddReviewers and Base are set,
	// a PATCH and a reviewer request should both be issued.
	var patchCalled, reviewerCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPatch &&
			r.URL.Path == "/api/v1/repos/alice/widget/pulls/15":
			patchCalled = true
			var body forgejogw.UpdatePullRequestOptions
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "develop", *body.Base)
			writeJSON(t, w, http.StatusCreated, &forgejogw.PullRequest{
				Index: 15,
				Title: "combined",
			})
		case r.Method == http.MethodPost &&
			r.URL.Path == "/api/v1/repos/alice/widget/pulls/15/requested_reviewers":
			reviewerCalled = true
			w.WriteHeader(http.StatusCreated)
		default:
			http.Error(w, "unexpected request: "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	err := r.EditChange(t.Context(), &PR{Number: 15}, forge.EditChangeOptions{
		Base:         "develop",
		AddReviewers: []string{"eve"},
	})
	require.NoError(t, err)
	assert.True(t, patchCalled, "PATCH should have been called")
	assert.True(t, reviewerCalled, "reviewer request should have been called")
}

func TestRepository_EditChange_reviewersOnly_noPatch(t *testing.T) {
	// When only AddReviewers is set (no other updatable fields),
	// PullRequestUpdate should NOT be called.
	var patchCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost &&
			r.URL.Path == "/api/v1/repos/alice/widget/pulls/20/requested_reviewers":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPatch:
			patchCalled = true
			http.Error(w, "unexpected PATCH", http.StatusInternalServerError)
		default:
			http.Error(w, "unexpected request: "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	err := r.EditChange(t.Context(), &PR{Number: 20}, forge.EditChangeOptions{
		AddReviewers: []string{"frank"},
	})
	require.NoError(t, err)
	assert.False(t, patchCalled, "PATCH must not be called when only reviewers change")
}
