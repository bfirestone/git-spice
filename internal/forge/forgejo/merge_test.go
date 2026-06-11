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
	"go.abhg.dev/gs/internal/git"
	"go.abhg.dev/gs/internal/silog"
)

func TestRepository_MergeChange_method(t *testing.T) {
	tests := []struct {
		name   string
		method forge.MergeMethod
		wantDo string
	}{
		{
			name:   "Merge",
			method: forge.MergeMethodMerge,
			wantDo: "merge",
		},
		{
			name:   "Squash",
			method: forge.MergeMethodSquash,
			wantDo: "squash",
		},
		{
			name:   "Rebase",
			method: forge.MergeMethodRebase,
			wantDo: "rebase",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBody forgejogw.MergePullRequestOptions
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodPost, r.Method)
				require.Equal(t, "/api/v1/repos/alice/widget/pulls/42/merge", r.URL.Path)
				require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
				w.WriteHeader(http.StatusNoContent)
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

			r := &Repository{
				client:            client,
				owner:             "alice",
				repo:              "widget",
				log:               silog.Nop(),
				defaultMergeStyle: "merge",
			}

			err = r.MergeChange(t.Context(), &PR{Number: 42}, forge.MergeChangeOptions{
				Method: tt.method,
			})
			require.NoError(t, err)
			assert.Equal(t, tt.wantDo, gotBody.Do)
		})
	}
}

func TestRepository_MergeChange_default(t *testing.T) {
	// MergeMethodDefault should use the repository's defaultMergeStyle.
	var gotBody forgejogw.MergePullRequestOptions
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.WriteHeader(http.StatusNoContent)
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

	r := &Repository{
		client:            client,
		owner:             "alice",
		repo:              "widget",
		log:               silog.Nop(),
		defaultMergeStyle: "squash",
	}

	err = r.MergeChange(t.Context(), &PR{Number: 1}, forge.MergeChangeOptions{
		Method: forge.MergeMethodDefault,
	})
	require.NoError(t, err)
	assert.Equal(t, "squash", gotBody.Do)
}

func TestRepository_MergeChange_headHash(t *testing.T) {
	// A non-empty HeadHash should be sent as HeadCommitID.
	var gotBody forgejogw.MergePullRequestOptions
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.WriteHeader(http.StatusNoContent)
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

	r := &Repository{
		client:            client,
		owner:             "alice",
		repo:              "widget",
		log:               silog.Nop(),
		defaultMergeStyle: "merge",
	}

	const headSHA = "abc123def456abc123def456abc123def456abc1"
	err = r.MergeChange(t.Context(), &PR{Number: 5}, forge.MergeChangeOptions{
		Method:   forge.MergeMethodMerge,
		HeadHash: git.Hash(headSHA),
	})
	require.NoError(t, err)
	assert.Equal(t, headSHA, gotBody.HeadCommitID)
}

func TestRepository_MergeChange_headHash_empty(t *testing.T) {
	// An empty HeadHash should NOT set HeadCommitID.
	var gotBody forgejogw.MergePullRequestOptions
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.WriteHeader(http.StatusNoContent)
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

	r := &Repository{
		client:            client,
		owner:             "alice",
		repo:              "widget",
		log:               silog.Nop(),
		defaultMergeStyle: "merge",
	}

	err = r.MergeChange(t.Context(), &PR{Number: 5}, forge.MergeChangeOptions{
		Method:   forge.MergeMethodMerge,
		HeadHash: "",
	})
	require.NoError(t, err)
	assert.Empty(t, gotBody.HeadCommitID)
}

func TestRepository_MergeChange_deleteBranch(t *testing.T) {
	// deleteBranchOnMerge=true should set DeleteBranchAfterMerge in the request.
	var gotBody forgejogw.MergePullRequestOptions
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.WriteHeader(http.StatusNoContent)
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

	r := &Repository{
		client:              client,
		owner:               "alice",
		repo:                "widget",
		log:                 silog.Nop(),
		defaultMergeStyle:   "merge",
		deleteBranchOnMerge: true,
	}

	err = r.MergeChange(t.Context(), &PR{Number: 3}, forge.MergeChangeOptions{
		Method: forge.MergeMethodMerge,
	})
	require.NoError(t, err)
	assert.True(t, gotBody.DeleteBranchAfterMerge)
}
