package forgejo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.abhg.dev/gs/internal/forge"
	forgejogw "go.abhg.dev/gs/internal/gateway/forgejo"
)

func TestRepository_SubmitChange_plain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/alice/widget/pulls" {
			var got forgejogw.CreatePullRequestOptions
			require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
			assert.Equal(t, "feat: thing", got.Title)
			assert.Equal(t, "main", got.Base)
			assert.Equal(t, "feature", got.Head)
			writeJSON(t, w, http.StatusCreated, &forgejogw.PullRequest{
				Index:   7,
				HTMLURL: "https://forgejo.example.com/alice/widget/pulls/7",
				State:   "open",
			})
			return
		}
		http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	result, err := r.SubmitChange(t.Context(), forge.SubmitChangeRequest{
		Subject: "feat: thing",
		Base:    "main",
		Head:    "feature",
	})
	require.NoError(t, err)
	assert.Equal(t, &PR{Number: 7}, result.ID)
	assert.Equal(t, "https://forgejo.example.com/alice/widget/pulls/7", result.URL)
}

func TestRepository_SubmitChange_draft(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/alice/widget/pulls" {
			var got forgejogw.CreatePullRequestOptions
			require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
			assert.Equal(t, "WIP: my feature", got.Title)
			writeJSON(t, w, http.StatusCreated, &forgejogw.PullRequest{
				Index:   8,
				HTMLURL: "https://forgejo.example.com/alice/widget/pulls/8",
				Title:   "WIP: my feature",
			})
			return
		}
		http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	result, err := r.SubmitChange(t.Context(), forge.SubmitChangeRequest{
		Subject: "my feature",
		Base:    "main",
		Head:    "feature",
		Draft:   true,
	})
	require.NoError(t, err)
	assert.Equal(t, &PR{Number: 8}, result.ID)
}

func TestRepository_SubmitChange_forkHead(t *testing.T) {
	// When PushRepository has a different owner, head should be "owner:branch".
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/alice/widget/pulls" {
			var got forgejogw.CreatePullRequestOptions
			require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
			assert.Equal(t, "bob:feature", got.Head)
			writeJSON(t, w, http.StatusCreated, &forgejogw.PullRequest{
				Index:   9,
				HTMLURL: "https://forgejo.example.com/alice/widget/pulls/9",
			})
			return
		}
		http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	result, err := r.SubmitChange(t.Context(), forge.SubmitChangeRequest{
		Subject: "feat: fork",
		Base:    "main",
		Head:    "feature",
		PushRepository: &RepositoryID{
			owner: "bob",
			name:  "widget",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, &PR{Number: 9}, result.ID)
}

func TestRepository_SubmitChange_sameOwnerNoPrefix(t *testing.T) {
	// When PushRepository has the same owner, head should not be prefixed.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/alice/widget/pulls" {
			var got forgejogw.CreatePullRequestOptions
			require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
			assert.Equal(t, "feature", got.Head)
			writeJSON(t, w, http.StatusCreated, &forgejogw.PullRequest{
				Index:   10,
				HTMLURL: "https://forgejo.example.com/alice/widget/pulls/10",
			})
			return
		}
		http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	result, err := r.SubmitChange(t.Context(), forge.SubmitChangeRequest{
		Subject: "feat: same owner",
		Base:    "main",
		Head:    "feature",
		PushRepository: &RepositoryID{
			owner: "alice",
			name:  "widget",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, &PR{Number: 10}, result.ID)
}

func TestRepository_SubmitChange_withLabels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/alice/widget/labels":
			writeJSON(t, w, http.StatusOK, []*forgejogw.Label{
				{ID: 1, Name: "bug"},
				{ID: 2, Name: "enhancement"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/alice/widget/pulls":
			var got forgejogw.CreatePullRequestOptions
			require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
			assert.Equal(t, []int64{1, 2}, got.Labels)
			writeJSON(t, w, http.StatusCreated, &forgejogw.PullRequest{
				Index:   11,
				HTMLURL: "https://forgejo.example.com/alice/widget/pulls/11",
			})
		default:
			http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	result, err := r.SubmitChange(t.Context(), forge.SubmitChangeRequest{
		Subject: "feat: labels",
		Base:    "main",
		Head:    "feature",
		Labels:  []string{"bug", "enhancement"},
	})
	require.NoError(t, err)
	assert.Equal(t, &PR{Number: 11}, result.ID)
}

func TestRepository_SubmitChange_withAssignees(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/alice/widget/pulls" {
			var got forgejogw.CreatePullRequestOptions
			require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
			assert.Equal(t, []string{"dave"}, got.Assignees)
			writeJSON(t, w, http.StatusCreated, &forgejogw.PullRequest{
				Index:   14,
				HTMLURL: "https://forgejo.example.com/alice/widget/pulls/14",
			})
			return
		}
		http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	result, err := r.SubmitChange(t.Context(), forge.SubmitChangeRequest{
		Subject:   "feat: assignees",
		Base:      "main",
		Head:      "feature",
		Assignees: []string{"dave"},
	})
	require.NoError(t, err)
	assert.Equal(t, &PR{Number: 14}, result.ID)
}

func TestRepository_SubmitChange_withReviewers(t *testing.T) {
	// Verify that reviewer request is issued after PR creation.
	var reviewerCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost &&
			r.URL.Path == "/api/v1/repos/alice/widget/pulls":
			writeJSON(t, w, http.StatusCreated, &forgejogw.PullRequest{
				Index:   12,
				HTMLURL: "https://forgejo.example.com/alice/widget/pulls/12",
			})
		case r.Method == http.MethodPost &&
			r.URL.Path == "/api/v1/repos/alice/widget/pulls/12/requested_reviewers":
			reviewerCalled = true
			var body struct {
				Reviewers []string `json:"reviewers"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, []string{"bob", "carol"}, body.Reviewers)
			w.WriteHeader(http.StatusCreated)
		default:
			http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	result, err := r.SubmitChange(t.Context(), forge.SubmitChangeRequest{
		Subject:   "feat: reviewers",
		Base:      "main",
		Head:      "feature",
		Reviewers: []string{"bob", "carol"},
	})
	require.NoError(t, err)
	assert.Equal(t, &PR{Number: 12}, result.ID)
	assert.True(t, reviewerCalled, "reviewer request should have been called")
}

func TestRepository_SubmitChange_reviewerFailureDoesNotFail(t *testing.T) {
	// A reviewer request failure should log a warning but not fail the submit.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost &&
			r.URL.Path == "/api/v1/repos/alice/widget/pulls":
			writeJSON(t, w, http.StatusCreated, &forgejogw.PullRequest{
				Index:   13,
				HTMLURL: "https://forgejo.example.com/alice/widget/pulls/13",
			})
		case r.Method == http.MethodPost &&
			r.URL.Path == "/api/v1/repos/alice/widget/pulls/13/requested_reviewers":
			// Simulate reviewer API failure.
			writeJSON(t, w, http.StatusUnprocessableEntity, map[string]any{
				"message": "reviewer not found",
			})
		default:
			http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	result, err := r.SubmitChange(t.Context(), forge.SubmitChangeRequest{
		Subject:   "feat: reviewer fail",
		Base:      "main",
		Head:      "feature",
		Reviewers: []string{"nonexistent"},
	})
	// Reviewer failure must not fail the submit.
	require.NoError(t, err)
	assert.Equal(t, &PR{Number: 13}, result.ID)
}

func TestRepository_SubmitChange_unsubmittedBase(t *testing.T) {
	// When Forgejo returns 404 for PR create, the base branch doesn't exist;
	// SubmitChange should return forge.ErrUnsubmittedBase.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/alice/widget/pulls" {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	_, err := r.SubmitChange(t.Context(), forge.SubmitChangeRequest{
		Subject: "feat: missing base",
		Base:    "does-not-exist",
		Head:    "feature",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, forge.ErrUnsubmittedBase)
}

func TestRepository_NewChangeMetadata(t *testing.T) {
	r := newTestRepository(t, nil, "alice", "widget")
	md, err := r.NewChangeMetadata(t.Context(), &PR{Number: 42})
	require.NoError(t, err)
	assert.Equal(t, &PRMetadata{PR: &PR{Number: 42}}, md)
}
