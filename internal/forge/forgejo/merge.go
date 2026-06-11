package forgejo

import (
	"context"
	"fmt"

	"go.abhg.dev/gs/internal/forge"
	forgejogw "go.abhg.dev/gs/internal/gateway/forgejo"
)

// MergeChange merges an open pull request into its base branch.
func (r *Repository) MergeChange(
	ctx context.Context, fid forge.ChangeID,
	opts forge.MergeChangeOptions,
) error {
	id := mustPR(fid)

	var method string
	switch opts.Method {
	case forge.MergeMethodDefault:
		method = r.defaultMergeStyle
	case forge.MergeMethodMerge:
		method = "merge"
	case forge.MergeMethodSquash:
		method = "squash"
	case forge.MergeMethodRebase:
		method = "rebase"
	default:
		r.log.Warn(
			"Unsupported merge method; using forge default",
			"method", opts.Method,
		)
		method = r.defaultMergeStyle
	}

	mergeOpts := &forgejogw.MergePullRequestOptions{
		Do:                     method,
		DeleteBranchAfterMerge: r.deleteBranchOnMerge,
	}
	if opts.HeadHash != "" {
		mergeOpts.HeadCommitID = opts.HeadHash.String()
	}

	if _, err := r.client.PullRequestMerge(
		ctx, r.owner, r.repo, id.Number, mergeOpts,
	); err != nil {
		return fmt.Errorf("merge pull request: %w", err)
	}

	r.log.Debug("Merged pull request", "pr", id.Number)
	return nil
}
