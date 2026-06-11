package forgejo

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"go.abhg.dev/gs/internal/cmputil"
	"go.abhg.dev/gs/internal/forge"
	forgejogw "go.abhg.dev/gs/internal/gateway/forgejo"
)

// EditChange edits an existing pull request in the repository.
func (r *Repository) EditChange(
	ctx context.Context,
	id forge.ChangeID,
	opts forge.EditChangeOptions,
) error {
	if cmputil.Zero(opts.Base) &&
		cmputil.Zero(opts.Draft) &&
		len(opts.AddLabels) == 0 &&
		len(opts.AddReviewers) == 0 &&
		len(opts.AddAssignees) == 0 {
		return nil // nothing to do
	}

	prNumber := mustPR(id).Number

	var (
		updateOptions forgejogw.UpdatePullRequestOptions
		hasUpdate     bool

		pr *forgejogw.PullRequest
	)

	// getPR fetches the current pull request state lazily,
	// caching the result for subsequent calls.
	getPR := func() (*forgejogw.PullRequest, error) {
		if pr != nil {
			return pr, nil
		}
		var err error
		pr, _, err = r.client.PullRequestGet(ctx, r.owner, r.repo, prNumber)
		if err != nil {
			return nil, fmt.Errorf("get pull request for update: %w", err)
		}
		return pr, nil
	}

	if opts.Base != "" {
		updateOptions.Base = &opts.Base
		hasUpdate = true
	}

	if opts.Draft != nil {
		current, err := getPR()
		if err != nil {
			return err
		}

		currentlyDraft := isDraftTitle(current.Title)
		if *opts.Draft && !currentlyDraft {
			newTitle := addDraftPrefix(current.Title)
			updateOptions.Title = &newTitle
			hasUpdate = true
		} else if !*opts.Draft && currentlyDraft {
			newTitle := stripDraftPrefix(current.Title)
			updateOptions.Title = &newTitle
			hasUpdate = true
		}
	}

	if len(opts.AddLabels) > 0 {
		newIDs, err := r.ensureLabels(ctx, opts.AddLabels)
		if err != nil {
			return fmt.Errorf("ensure labels: %w", err)
		}

		current, err := getPR()
		if err != nil {
			return err
		}

		updateOptions.Labels = mergeEditLabelIDs(current.Labels, newIDs)
		hasUpdate = true
	}

	if len(opts.AddAssignees) > 0 {
		current, err := getPR()
		if err != nil {
			return err
		}

		updateOptions.Assignees = mergeEditAssignees(current.Assignees, opts.AddAssignees)
		hasUpdate = true
	}

	if hasUpdate {
		_, _, err := r.client.PullRequestUpdate(
			ctx, r.owner, r.repo, prNumber, &updateOptions,
		)
		if err != nil {
			return fmt.Errorf("update pull request: %w", err)
		}
		r.log.Debug("Updated pull request", "pr", prNumber)
	}

	if len(opts.AddReviewers) > 0 {
		_, err := r.client.ReviewerRequest(
			ctx, r.owner, r.repo, prNumber, opts.AddReviewers,
		)
		if err != nil {
			return fmt.Errorf("request reviewers: %w", err)
		}
	}

	return nil
}

// mergeEditLabelIDs unions existing label IDs with the new IDs.
// Forgejo's PATCH replaces the full label set,
// so the union is required to preserve existing labels.
func mergeEditLabelIDs(existing []*forgejogw.Label, newIDs []int64) []int64 {
	seen := make(map[int64]struct{}, len(existing)+len(newIDs))
	for _, lbl := range existing {
		seen[lbl.ID] = struct{}{}
	}
	for _, id := range newIDs {
		seen[id] = struct{}{}
	}
	return slices.Sorted(maps.Keys(seen))
}

// mergeEditAssignees unions existing assignee usernames with the new ones.
// Forgejo's PATCH replaces the full assignee set,
// so the union is required to preserve existing assignees.
func mergeEditAssignees(existing []*forgejogw.User, newNames []string) []string {
	seen := make(map[string]struct{}, len(existing)+len(newNames))
	for _, u := range existing {
		seen[u.UserName] = struct{}{}
	}
	for _, name := range newNames {
		seen[name] = struct{}{}
	}
	return slices.Sorted(maps.Keys(seen))
}
