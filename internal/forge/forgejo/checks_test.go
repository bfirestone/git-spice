package forgejo

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.abhg.dev/gs/internal/forge"
	forgejogw "go.abhg.dev/gs/internal/gateway/forgejo"
)

func TestRepository_ChangeChecksState_stateMapping(t *testing.T) {
	tests := []struct {
		name       string
		state      string
		totalCount int64
		want       forge.ChecksState
	}{
		{
			name:       "EmptyState",
			state:      "",
			totalCount: 0,
			want:       forge.ChecksPassed,
		},
		{
			name:       "NoStatuses",
			state:      "",
			totalCount: 0,
			want:       forge.ChecksPassed,
		},
		{
			name:       "Pending",
			state:      "pending",
			totalCount: 1,
			want:       forge.ChecksPending,
		},
		{
			name:       "Success",
			state:      "success",
			totalCount: 1,
			want:       forge.ChecksPassed,
		},
		{
			name:       "Warning",
			state:      "warning",
			totalCount: 1,
			want:       forge.ChecksPassed,
		},
		{
			name:       "Error",
			state:      "error",
			totalCount: 1,
			want:       forge.ChecksFailed,
		},
		{
			name:       "Failure",
			state:      "failure",
			totalCount: 1,
			want:       forge.ChecksFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const headSHA = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v1/repos/alice/widget/pulls/7":
					writeJSON(t, w, http.StatusOK, &forgejogw.PullRequest{
						Index: 7,
						Head:  &forgejogw.PRBranch{SHA: headSHA},
					})
				case "/api/v1/repos/alice/widget/commits/" + headSHA + "/status":
					writeJSON(t, w, http.StatusOK, &forgejogw.CombinedStatus{
						State:      tt.state,
						TotalCount: tt.totalCount,
					})
				default:
					http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
				}
			}))
			defer srv.Close()

			r := newTestRepository(t, srv, "alice", "widget")

			state, err := r.ChangeChecksState(t.Context(), &PR{Number: 7})
			require.NoError(t, err)
			assert.Equal(t, tt.want, state, "state=%q totalCount=%d", tt.state, tt.totalCount)
		})
	}
}

func TestRepository_ChangeChecksState_noStatuses(t *testing.T) {
	// When TotalCount is 0, ChecksPassed is returned without
	// the state value mattering.
	const headSHA = "cafebabecafebabecafebabecafebabecafebabe"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/repos/alice/widget/pulls/3":
			writeJSON(t, w, http.StatusOK, &forgejogw.PullRequest{
				Index: 3,
				Head:  &forgejogw.PRBranch{SHA: headSHA},
			})
		case "/api/v1/repos/alice/widget/commits/" + headSHA + "/status":
			writeJSON(t, w, http.StatusOK, &forgejogw.CombinedStatus{
				TotalCount: 0,
				State:      "",
			})
		default:
			http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	r := newTestRepository(t, srv, "alice", "widget")

	state, err := r.ChangeChecksState(t.Context(), &PR{Number: 3})
	require.NoError(t, err)
	assert.Equal(t, forge.ChecksPassed, state)
}
