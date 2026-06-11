package forgejo

import (
	"context"
	"fmt"

	"go.abhg.dev/gs/internal/forge"
)

// CommentCountsByChange retrieves comment counts for multiple pull requests.
//
// Forgejo's API does not expose thread-resolution state,
// so Resolved and Unresolved are always zero.
// Only Total is populated from the PR's comment count.
func (r *Repository) CommentCountsByChange(
	ctx context.Context,
	ids []forge.ChangeID,
) ([]*forge.CommentCounts, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	results := make([]*forge.CommentCounts, len(ids))
	for i, id := range ids {
		pr, _, err := r.client.PullRequestGet(
			ctx, r.owner, r.repo, mustPR(id).Number,
		)
		if err != nil {
			return nil, fmt.Errorf("get counts for %v: %w", id, err)
		}
		results[i] = &forge.CommentCounts{
			Total: int(pr.Comments),
			// Resolved and Unresolved remain zero:
			// the Forgejo API does not expose thread resolution state.
		}
	}

	return results, nil
}
