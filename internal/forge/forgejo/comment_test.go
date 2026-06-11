package forgejo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.abhg.dev/gs/internal/forge"
	forgejogw "go.abhg.dev/gs/internal/gateway/forgejo"
)

func TestRepository_PostChangeComment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost &&
			r.URL.Path == "/api/v1/repos/alice/widget/issues/1/comments" {
			var body struct {
				Body string `json:"body"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "hello world", body.Body)
			writeJSON(t, w, http.StatusCreated, &forgejogw.Comment{
				ID:   42,
				Body: body.Body,
			})
			return
		}
		http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	cid, err := r.PostChangeComment(t.Context(), &PR{Number: 1}, "hello world")
	require.NoError(t, err)

	prc := mustPRComment(cid)
	assert.Equal(t, int64(42), prc.ID)
}

func TestRepository_UpdateChangeComment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch &&
			r.URL.Path == "/api/v1/repos/alice/widget/issues/comments/42" {
			var body struct {
				Body string `json:"body"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "updated body", body.Body)
			writeJSON(t, w, http.StatusOK, &forgejogw.Comment{
				ID:   42,
				Body: body.Body,
			})
			return
		}
		http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	err := r.UpdateChangeComment(
		t.Context(),
		&PRComment{ID: 42},
		"updated body",
	)
	require.NoError(t, err)
}

func TestRepository_UpdateChangeComment_notFound(t *testing.T) {
	// Updating a deleted comment must surface forge.ErrNotFound.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch &&
			r.URL.Path == "/api/v1/repos/alice/widget/issues/comments/42" {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	err := r.UpdateChangeComment(
		t.Context(),
		&PRComment{ID: 42},
		"updated body",
	)
	assert.ErrorIs(t, err, forge.ErrNotFound)
}

func TestRepository_DeleteChangeComment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete &&
			r.URL.Path == "/api/v1/repos/alice/widget/issues/comments/99" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	err := r.DeleteChangeComment(t.Context(), &PRComment{ID: 99})
	require.NoError(t, err)
}

func TestRepository_ListChangeComments_noFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet &&
			r.URL.Path == "/api/v1/repos/alice/widget/issues/5/comments" {
			writeJSON(t, w, http.StatusOK, []*forgejogw.Comment{
				{ID: 1, Body: "first comment", User: &forgejogw.User{ID: 10}},
				{ID: 2, Body: "second comment", User: &forgejogw.User{ID: 20}},
			})
			return
		}
		http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	var items []*forge.ListChangeCommentItem
	for item, err := range r.ListChangeComments(t.Context(), &PR{Number: 5}, nil) {
		require.NoError(t, err)
		items = append(items, item)
	}

	require.Len(t, items, 2)
	assert.Equal(t, "first comment", items[0].Body)
	assert.Equal(t, "second comment", items[1].Body)
}

func TestRepository_ListChangeComments_bodyRegexFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet &&
			r.URL.Path == "/api/v1/repos/alice/widget/issues/5/comments" {
			writeJSON(t, w, http.StatusOK, []*forgejogw.Comment{
				{ID: 1, Body: "gs-nav: keep me", User: &forgejogw.User{ID: 10}},
				{ID: 2, Body: "plain comment", User: &forgejogw.User{ID: 10}},
			})
			return
		}
		http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	opts := &forge.ListChangeCommentsOptions{
		BodyMatchesAll: []*regexp.Regexp{regexp.MustCompile(`gs-nav:`)},
	}

	var items []*forge.ListChangeCommentItem
	for item, err := range r.ListChangeComments(t.Context(), &PR{Number: 5}, opts) {
		require.NoError(t, err)
		items = append(items, item)
	}

	require.Len(t, items, 1)
	assert.Equal(t, "gs-nav: keep me", items[0].Body)
	assert.Equal(t, int64(1), mustPRComment(items[0].ID).ID)
}

func TestRepository_ListChangeComments_canUpdateFilter(t *testing.T) {
	// userID 10 is the current user; comments from user 20 must be excluded.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet &&
			r.URL.Path == "/api/v1/repos/alice/widget/issues/5/comments" {
			writeJSON(t, w, http.StatusOK, []*forgejogw.Comment{
				{ID: 1, Body: "mine", User: &forgejogw.User{ID: 10}},
				{ID: 2, Body: "theirs", User: &forgejogw.User{ID: 20}},
			})
			return
		}
		http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	r.userID = 10

	opts := &forge.ListChangeCommentsOptions{CanUpdate: true}
	var items []*forge.ListChangeCommentItem
	for item, err := range r.ListChangeComments(t.Context(), &PR{Number: 5}, opts) {
		require.NoError(t, err)
		items = append(items, item)
	}

	require.Len(t, items, 1)
	assert.Equal(t, "mine", items[0].Body)
}

func TestRepository_ListChangeComments_multipleRegexAllMustMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet &&
			r.URL.Path == "/api/v1/repos/alice/widget/issues/5/comments" {
			writeJSON(t, w, http.StatusOK, []*forgejogw.Comment{
				{ID: 1, Body: "gs-nav: foo bar", User: &forgejogw.User{ID: 10}},
				{ID: 2, Body: "gs-nav: foo only", User: &forgejogw.User{ID: 10}},
				{ID: 3, Body: "neither", User: &forgejogw.User{ID: 10}},
			})
			return
		}
		http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	opts := &forge.ListChangeCommentsOptions{
		BodyMatchesAll: []*regexp.Regexp{
			regexp.MustCompile(`gs-nav:`),
			regexp.MustCompile(`bar`),
		},
	}

	var ids []int64
	for item, err := range r.ListChangeComments(t.Context(), &PR{Number: 5}, opts) {
		require.NoError(t, err)
		ids = append(ids, mustPRComment(item.ID).ID)
	}

	assert.True(t, slices.Equal([]int64{1}, ids))
}

func TestRepository_CommentCountsByChange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/repos/alice/widget/pulls/1":
			writeJSON(t, w, http.StatusOK, &forgejogw.PullRequest{
				Index:    1,
				Comments: 5,
			})
		case "/api/v1/repos/alice/widget/pulls/2":
			writeJSON(t, w, http.StatusOK, &forgejogw.PullRequest{
				Index:    2,
				Comments: 0,
			})
		default:
			http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	ids := []forge.ChangeID{&PR{Number: 1}, &PR{Number: 2}}
	counts, err := r.CommentCountsByChange(t.Context(), ids)
	require.NoError(t, err)

	require.Len(t, counts, 2)
	assert.Equal(t, 5, counts[0].Total)
	assert.Equal(t, 0, counts[0].Resolved)
	assert.Equal(t, 0, counts[0].Unresolved)
	assert.Equal(t, 0, counts[1].Total)
}

func TestRepository_CommentCountsByChange_empty(t *testing.T) {
	r := newTestRepository(t, nil, "alice", "widget")
	counts, err := r.CommentCountsByChange(t.Context(), nil)
	require.NoError(t, err)
	assert.Nil(t, counts)
}

func TestRepository_ListChangeComments_pagination(t *testing.T) {
	// First page is full (30 items); second page has 1 item.
	// All items must be yielded across both pages.
	const pageSize = 30
	var requestCount int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet &&
			r.URL.Path == "/api/v1/repos/alice/widget/issues/5/comments" {
			requestCount++
			page := r.URL.Query().Get("page")
			switch page {
			case "1", "":
				// Full first page.
				comments := make([]*forgejogw.Comment, pageSize)
				for i := range comments {
					comments[i] = &forgejogw.Comment{
						ID:   int64(i + 1),
						Body: "page1",
						User: &forgejogw.User{ID: 1},
					}
				}
				writeJSON(t, w, http.StatusOK, comments)
			case "2":
				writeJSON(t, w, http.StatusOK, []*forgejogw.Comment{
					{ID: 31, Body: "page2", User: &forgejogw.User{ID: 1}},
				})
			default:
				writeJSON(t, w, http.StatusOK, []*forgejogw.Comment{})
			}
			return
		}
		http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	var items []*forge.ListChangeCommentItem
	for item, err := range r.ListChangeComments(t.Context(), &PR{Number: 5}, nil) {
		require.NoError(t, err)
		items = append(items, item)
	}

	assert.Len(t, items, pageSize+1)
	assert.Greater(t, requestCount, 1, "must have fetched more than one page")
}

func TestRepository_ListChangeComments_commentIDsAreCorrect(t *testing.T) {
	// Verify that each returned item's ID round-trips through mustPRComment.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet &&
			r.URL.Path == "/api/v1/repos/alice/widget/issues/7/comments" {
			writeJSON(t, w, http.StatusOK, []*forgejogw.Comment{
				{ID: 100, Body: "a", User: &forgejogw.User{ID: 1}},
				{ID: 200, Body: "b", User: &forgejogw.User{ID: 2}},
			})
			return
		}
		http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")
	var ids []int64
	for item, err := range r.ListChangeComments(t.Context(), &PR{Number: 7}, nil) {
		require.NoError(t, err)
		ids = append(ids, mustPRComment(item.ID).ID)
	}

	assert.True(t, slices.Equal([]int64{100, 200}, ids))
}
