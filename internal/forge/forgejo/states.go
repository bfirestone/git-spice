package forgejo

import (
	"context"
	"fmt"

	"go.abhg.dev/gs/internal/forge"
	forgejogw "go.abhg.dev/gs/internal/gateway/forgejo"
	"go.abhg.dev/gs/internal/git"
)

// ChangeStatuses retrieves compact status summaries for the given changes.
// Each change is fetched individually.
func (r *Repository) ChangeStatuses(
	ctx context.Context,
	ids []forge.ChangeID,
) ([]forge.ChangeStatus, error) {
	statuses := make([]forge.ChangeStatus, len(ids))
	for i, id := range ids {
		pr, _, err := r.client.PullRequestGet(ctx, r.owner, r.repo, mustPR(id).Number)
		if err != nil {
			return nil, fmt.Errorf("get pull request %v: %w", id, err)
		}
		var headHash git.Hash
		if pr.Head != nil {
			headHash = git.Hash(pr.Head.SHA)
		}
		statuses[i] = forge.ChangeStatus{
			State:    changeStateFrom(pr),
			HeadHash: headHash,
		}
	}
	return statuses, nil
}

// changeStateFrom maps a Forgejo pull request to a forge.ChangeState.
// A merged PR maps to ChangeMerged,
// a closed but unmerged PR maps to ChangeClosed,
// and everything else maps to ChangeOpen.
func changeStateFrom(pr *forgejogw.PullRequest) forge.ChangeState {
	if pr.Merged {
		return forge.ChangeMerged
	}
	if pr.State == "closed" {
		return forge.ChangeClosed
	}
	return forge.ChangeOpen
}

// stateFilter converts a forge.ChangeState to the query parameter value
// accepted by Forgejo's list-pull-requests endpoint.
// ChangeOpen maps to "open", ChangeClosed and ChangeMerged map to "closed",
// and the zero value maps to "all".
func stateFilter(s forge.ChangeState) string {
	switch s {
	case forge.ChangeOpen:
		return "open"
	case forge.ChangeClosed, forge.ChangeMerged:
		return "closed"
	default:
		return "all"
	}
}
