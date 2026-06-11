package forgejo

import (
	"context"
	"errors"
	"fmt"

	"go.abhg.dev/gs/internal/forge"
	forgejogw "go.abhg.dev/gs/internal/gateway/forgejo"
)

// SubmitChange creates a new pull request in the Forgejo repository.
func (r *Repository) SubmitChange(
	ctx context.Context,
	req forge.SubmitChangeRequest,
) (forge.SubmitChangeResult, error) {
	head := req.Head
	if req.PushRepository != nil {
		pushOwner := mustRepositoryID(req.PushRepository).owner
		if pushOwner != r.owner {
			head = pushOwner + ":" + req.Head
		}
	}

	title := req.Subject
	if req.Draft {
		title = addDraftPrefix(title)
	}

	opt := &forgejogw.CreatePullRequestOptions{
		Title: title,
		Base:  req.Base,
		Head:  head,
	}
	if req.Body != "" {
		opt.Body = req.Body
	}
	if len(req.Assignees) > 0 {
		opt.Assignees = req.Assignees
	}

	if len(req.Labels) > 0 {
		ids, err := r.ensureLabels(ctx, req.Labels)
		if err != nil {
			return forge.SubmitChangeResult{}, fmt.Errorf("ensure labels: %w", err)
		}
		opt.Labels = ids
	}

	pr, _, err := r.client.PullRequestCreate(ctx, r.owner, r.repo, opt)
	if err != nil {
		if errors.Is(err, forgejogw.ErrNotFound) {
			return forge.SubmitChangeResult{}, fmt.Errorf(
				"create pull request: %w",
				forge.ErrUnsubmittedBase,
			)
		}
		return forge.SubmitChangeResult{}, fmt.Errorf("create pull request: %w", err)
	}
	r.log.Debug("Created pull request", "pr", pr.Index, "url", pr.HTMLURL)

	if len(req.Reviewers) > 0 {
		if _, reviewErr := r.client.ReviewerRequest(
			ctx, r.owner, r.repo, pr.Index, req.Reviewers,
		); reviewErr != nil {
			r.log.Warn("Add reviewers to pull request",
				"pr", pr.Index,
				"error", reviewErr,
			)
		}
	}

	return forge.SubmitChangeResult{
		ID:  &PR{Number: pr.Index},
		URL: pr.HTMLURL,
	}, nil
}

// NewChangeMetadata returns the metadata for a pull request.
// No network call is needed — it wraps the ID in a PRMetadata.
func (r *Repository) NewChangeMetadata(
	_ context.Context,
	id forge.ChangeID,
) (forge.ChangeMetadata, error) {
	return &PRMetadata{PR: mustPR(id)}, nil
}

// mustRepositoryID type-asserts a forge.RepositoryID to *RepositoryID,
// panicking if the assertion fails.
func mustRepositoryID(id forge.RepositoryID) *RepositoryID {
	rid, ok := id.(*RepositoryID)
	if !ok {
		panic(fmt.Sprintf("forgejo: expected *RepositoryID, got %T", id))
	}
	return rid
}
