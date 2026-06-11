package forgejo

import (
	"context"
	"fmt"

	"go.abhg.dev/gs/internal/forge"
)

// ChangeChecksState reports the aggregate CI/checks state
// for the given pull request.
func (r *Repository) ChangeChecksState(
	ctx context.Context, fid forge.ChangeID,
) (forge.ChecksState, error) {
	id := mustPR(fid)

	pr, _, err := r.client.PullRequestGet(ctx, r.owner, r.repo, id.Number)
	if err != nil {
		return 0, fmt.Errorf("get pull request: %w", err)
	}

	if pr.Head == nil {
		return forge.ChecksPassed, nil
	}

	combined, _, err := r.client.CommitStatusGet(
		ctx, r.owner, r.repo, pr.Head.SHA,
	)
	if err != nil {
		return 0, fmt.Errorf("get commit status: %w", err)
	}

	return checksStateFrom(combined.TotalCount, combined.State), nil
}

// checksStateFrom maps a Forgejo CombinedStatus to a forge.ChecksState.
//
// Mapping:
//   - TotalCount == 0 or empty state -> ChecksPassed (no checks configured)
//   - "error" or "failure"           -> ChecksFailed
//   - "pending"                      -> ChecksPending
//   - "success", "warning", unknown  -> ChecksPassed
func checksStateFrom(totalCount int64, state string) forge.ChecksState {
	if totalCount == 0 || state == "" {
		return forge.ChecksPassed
	}

	switch state {
	case "error", "failure":
		return forge.ChecksFailed
	case "pending":
		return forge.ChecksPending
	default:
		return forge.ChecksPassed
	}
}
