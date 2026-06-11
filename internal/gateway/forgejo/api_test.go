package forgejo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_UserCurrent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v1/user", r.URL.Path)
		assert.Equal(t, "git-spice", r.Header.Get("User-Agent"))
		writeJSON(t, w, http.StatusOK, User{
			ID:       7,
			UserName: "spock",
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	user, _, err := client.UserCurrent(t.Context())
	require.NoError(t, err)
	assert.Equal(t, int64(7), user.ID)
	assert.Equal(t, "spock", user.UserName)
}

func TestClient_RepoGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v1/repos/alice/widget", r.URL.Path)
		writeJSON(t, w, http.StatusOK, Repository{
			ID:            42,
			Name:          "widget",
			DefaultBranch: "main",
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	repo, _, err := client.RepoGet(t.Context(), "alice", "widget")
	require.NoError(t, err)
	assert.Equal(t, int64(42), repo.ID)
	assert.Equal(t, "main", repo.DefaultBranch)
}

func TestClient_PullRequestCreate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/repos/alice/widget/pulls", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "git-spice", r.Header.Get("User-Agent"))
		var got CreatePullRequestOptions
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		assert.Equal(t, "feat: thing", got.Title)
		assert.Equal(t, "main", got.Base)
		assert.Equal(t, "feature", got.Head)
		writeJSON(t, w, http.StatusCreated, PullRequest{
			Index:   7,
			HTMLURL: "https://x/alice/widget/pulls/7",
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	pr, _, err := client.PullRequestCreate(t.Context(), "alice", "widget",
		&CreatePullRequestOptions{
			Title: "feat: thing",
			Base:  "main",
			Head:  "feature",
		})
	require.NoError(t, err)
	assert.Equal(t, int64(7), pr.Index)
	assert.Equal(t, "https://x/alice/widget/pulls/7", pr.HTMLURL)
}

func TestClient_PullRequestGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v1/repos/alice/widget/pulls/7", r.URL.Path)
		writeJSON(t, w, http.StatusOK, PullRequest{
			Index: 7,
			Title: "feat: thing",
			State: "open",
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	pr, _, err := client.PullRequestGet(t.Context(), "alice", "widget", 7)
	require.NoError(t, err)
	assert.Equal(t, int64(7), pr.Index)
	assert.Equal(t, "feat: thing", pr.Title)
	assert.Equal(t, "open", pr.State)
}

func TestClient_PullRequestList(t *testing.T) {
	t.Run("OpenState", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/api/v1/repos/alice/widget/pulls", r.URL.Path)
			assert.Equal(t, "open", r.URL.Query().Get("state"))
			assert.Equal(t, "20", r.URL.Query().Get("limit"))
			assert.Equal(t, "1", r.URL.Query().Get("page"))
			writeJSON(t, w, http.StatusOK, []*PullRequest{
				{Index: 1, Title: "One"},
				{Index: 2, Title: "Two"},
			})
		}))
		defer srv.Close()

		client := newTestClient(t, srv)
		prs, _, err := client.PullRequestList(t.Context(), "alice", "widget",
			&ListPullRequestsOptions{State: "open", Limit: 20, Page: 1})
		require.NoError(t, err)
		require.Len(t, prs, 2)
		assert.Equal(t, int64(2), prs[1].Index)
	})

	t.Run("ClosedState", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "closed", r.URL.Query().Get("state"))
			writeJSON(t, w, http.StatusOK, []*PullRequest{
				{Index: 5, Title: "Closed"},
			})
		}))
		defer srv.Close()

		client := newTestClient(t, srv)
		prs, _, err := client.PullRequestList(t.Context(), "alice", "widget",
			&ListPullRequestsOptions{State: "closed"})
		require.NoError(t, err)
		require.Len(t, prs, 1)
		assert.Equal(t, int64(5), prs[0].Index)
	})

	t.Run("NilOptions", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/api/v1/repos/alice/widget/pulls", r.URL.Path)
			assert.Empty(t, r.URL.RawQuery)
			writeJSON(t, w, http.StatusOK, []*PullRequest{})
		}))
		defer srv.Close()

		client := newTestClient(t, srv)
		prs, _, err := client.PullRequestList(t.Context(), "alice", "widget", nil)
		require.NoError(t, err)
		assert.Empty(t, prs)
	})
}

func TestClient_PullRequestUpdate(t *testing.T) {
	title := "updated title"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/api/v1/repos/alice/widget/pulls/7", r.URL.Path)
		assertJSONBody(t, r, `{"title":"updated title"}`)
		writeJSON(t, w, http.StatusOK, PullRequest{
			Index: 7,
			Title: "updated title",
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	pr, _, err := client.PullRequestUpdate(t.Context(), "alice", "widget", 7,
		&UpdatePullRequestOptions{Title: &title})
	require.NoError(t, err)
	assert.Equal(t, "updated title", pr.Title)
}

func TestClient_PullRequestMerge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/repos/alice/widget/pulls/7/merge", r.URL.Path)
		assertJSONBody(t, r, `{"Do":"merge","delete_branch_after_merge":true}`)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	resp, err := client.PullRequestMerge(t.Context(), "alice", "widget", 7,
		&MergePullRequestOptions{
			Do:                     "merge",
			DeleteBranchAfterMerge: true,
		})
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestClient_ReviewerRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/repos/alice/widget/pulls/7/requested_reviewers", r.URL.Path)
		assertJSONBody(t, r, `{"reviewers":["spock","bones"]}`)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	resp, err := client.ReviewerRequest(t.Context(), "alice", "widget", 7,
		[]string{"spock", "bones"})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestClient_CommentCreate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/repos/alice/widget/issues/7/comments", r.URL.Path)
		assertJSONBody(t, r, `{"body":"great work!"}`)
		writeJSON(t, w, http.StatusCreated, Comment{
			ID:   99,
			Body: "great work!",
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	comment, _, err := client.CommentCreate(t.Context(), "alice", "widget", 7, "great work!")
	require.NoError(t, err)
	assert.Equal(t, int64(99), comment.ID)
	assert.Equal(t, "great work!", comment.Body)
}

func TestClient_CommentUpdate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/api/v1/repos/alice/widget/issues/comments/99", r.URL.Path)
		assertJSONBody(t, r, `{"body":"updated comment"}`)
		writeJSON(t, w, http.StatusOK, Comment{
			ID:   99,
			Body: "updated comment",
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	comment, _, err := client.CommentUpdate(t.Context(), "alice", "widget", 99, "updated comment")
	require.NoError(t, err)
	assert.Equal(t, "updated comment", comment.Body)
}

func TestClient_CommentDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/api/v1/repos/alice/widget/issues/comments/99", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	resp, err := client.CommentDelete(t.Context(), "alice", "widget", 99)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestClient_CommentList(t *testing.T) {
	t.Run("NoOptions", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/api/v1/repos/alice/widget/issues/7/comments", r.URL.Path)
			assert.Empty(t, r.URL.Query().Get("page"))
			assert.Empty(t, r.URL.Query().Get("limit"))
			writeJSON(t, w, http.StatusOK, []*Comment{
				{ID: 88, Body: "alpha"},
				{ID: 89, Body: "beta"},
			})
		}))
		defer srv.Close()

		client := newTestClient(t, srv)
		comments, _, err := client.CommentList(t.Context(), "alice", "widget", 7,
			&ListCommentsOptions{})
		require.NoError(t, err)
		require.Len(t, comments, 2)
		assert.Equal(t, "beta", comments[1].Body)
	})

	t.Run("WithPagination", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/api/v1/repos/alice/widget/issues/7/comments", r.URL.Path)
			assert.Equal(t, "2", r.URL.Query().Get("page"))
			assert.Equal(t, "30", r.URL.Query().Get("limit"))
			writeJSON(t, w, http.StatusOK, []*Comment{
				{ID: 91, Body: "page2"},
			})
		}))
		defer srv.Close()

		client := newTestClient(t, srv)
		comments, _, err := client.CommentList(t.Context(), "alice", "widget", 7,
			&ListCommentsOptions{Page: 2, Limit: 30})
		require.NoError(t, err)
		require.Len(t, comments, 1)
		assert.Equal(t, "page2", comments[0].Body)
	})

	t.Run("NilOptions", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Empty(t, r.URL.RawQuery)
			writeJSON(t, w, http.StatusOK, []*Comment{})
		}))
		defer srv.Close()

		client := newTestClient(t, srv)
		comments, _, err := client.CommentList(t.Context(), "alice", "widget", 7, nil)
		require.NoError(t, err)
		assert.Empty(t, comments)
	})
}

func TestClient_CommitStatusGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v1/repos/alice/widget/commits/abc123/status", r.URL.Path)
		writeJSON(t, w, http.StatusOK, CombinedStatus{
			State:      "success",
			TotalCount: 2,
			Statuses: []*CommitStatus{
				{Status: "success"},
				{Status: "success"},
			},
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	status, _, err := client.CommitStatusGet(t.Context(), "alice", "widget", "abc123")
	require.NoError(t, err)
	assert.Equal(t, "success", status.State)
	assert.Equal(t, int64(2), status.TotalCount)
}

func TestClient_LabelList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v1/repos/alice/widget/labels", r.URL.Path)
		writeJSON(t, w, http.StatusOK, []*Label{
			{ID: 1, Name: "bug"},
			{ID: 2, Name: "enhancement"},
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	labels, _, err := client.LabelList(t.Context(), "alice", "widget")
	require.NoError(t, err)
	require.Len(t, labels, 2)
	assert.Equal(t, "bug", labels[0].Name)
}

func TestClient_LabelCreate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/repos/alice/widget/labels", r.URL.Path)
		assertJSONBody(t, r, `{"name":"bug","color":"#cccccc"}`)
		writeJSON(t, w, http.StatusCreated, Label{
			ID:   1,
			Name: "bug",
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	label, _, err := client.LabelCreate(t.Context(), "alice", "widget", "bug")
	require.NoError(t, err)
	assert.Equal(t, int64(1), label.ID)
	assert.Equal(t, "bug", label.Name)
}

func TestClient_ContentsGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v1/repos/alice/widget/contents/src/main.go", r.URL.Path)
		writeJSON(t, w, http.StatusOK, ContentsResponse{
			Name:     "main.go",
			Content:  "aGVsbG8=",
			Encoding: "base64",
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	contents, _, err := client.ContentsGet(t.Context(), "alice", "widget", "src/main.go")
	require.NoError(t, err)
	assert.Equal(t, "main.go", contents.Name)
	assert.Equal(t, "aGVsbG8=", contents.Content)
}

func TestClient_ContentsGet_specialChars(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		// Segments are individually escaped; slashes are preserved as
		// path separators.
		assert.Equal(
			t,
			"/api/v1/repos/alice/widget/contents/dir/with%20spaces/file.go",
			r.URL.EscapedPath(),
		)
		writeJSON(t, w, http.StatusOK, ContentsResponse{
			Name: "file.go",
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	_, _, err := client.ContentsGet(
		t.Context(),
		"alice",
		"widget",
		"dir/with spaces/file.go",
	)
	require.NoError(t, err)
}

func TestClient_PullRequestCreate_withSpecialChars(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(
			t,
			"/api/v1/repos/alice%2Fteam/widget%20repo/pulls",
			r.URL.EscapedPath(),
		)
		writeJSON(t, w, http.StatusCreated, PullRequest{Index: 1})
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	_, _, err := client.PullRequestCreate(t.Context(), "alice/team", "widget repo",
		&CreatePullRequestOptions{Title: "test", Base: "main", Head: "feat"})
	require.NoError(t, err)
}

// TestClient_PullRequestMerge_withHeadCommit verifies that
// head_commit_id is included in the request body when set.
func TestClient_PullRequestMerge_withHeadCommit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/repos/alice/widget/pulls/7/merge", r.URL.Path)
		assertJSONBody(t, r, `{"Do":"squash","head_commit_id":"abc123"}`)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	_, err := client.PullRequestMerge(t.Context(), "alice", "widget", 7,
		&MergePullRequestOptions{
			Do:           "squash",
			HeadCommitID: "abc123",
		})
	require.NoError(t, err)
}
