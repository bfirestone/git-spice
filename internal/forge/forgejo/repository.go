package forgejo

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"iter"

	"go.abhg.dev/gs/internal/forge"
	forgejogw "go.abhg.dev/gs/internal/gateway/forgejo"
	"go.abhg.dev/gs/internal/silog"
)

// ErrNotImplemented indicates that a feature is not yet implemented.
var ErrNotImplemented = errors.New("not implemented")

// Repository is a Forgejo repository hosted on a Forgejo instance.
type Repository struct {
	client *forgejogw.Client

	owner, repo string
	log         *silog.Logger
	forge       *Forge

	// ID of the authenticated user.
	userID int64

	// defaultMergeStyle is the repository's configured
	// default merge style, e.g. "merge" or "squash".
	defaultMergeStyle string

	deleteBranchOnMerge bool
}

var _ forge.Repository = (*Repository)(nil)

// repositoryOptions provides optional configuration for newRepository.
type repositoryOptions struct {
	// DeleteBranchOnMerge controls whether the source branch
	// is deleted after its pull request merges.
	DeleteBranchOnMerge bool
}

// newRepository opens a Forgejo repository,
// fetching repo settings and the current user.
func newRepository(
	ctx context.Context,
	f *Forge,
	owner, repo string,
	log *silog.Logger,
	client *forgejogw.Client,
	opts *repositoryOptions,
) (*Repository, error) {
	opts = cmp.Or(opts, &repositoryOptions{})

	repoInfo, _, err := client.RepoGet(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	user, _, err := client.UserCurrent(ctx)
	if err != nil {
		return nil, fmt.Errorf("get current user: %w", err)
	}

	mergeStyle := cmp.Or(repoInfo.DefaultMergeStyle, "merge")

	return &Repository{
		client:              client,
		owner:               owner,
		repo:                repo,
		log:                 log,
		forge:               f,
		userID:              user.ID,
		defaultMergeStyle:   mergeStyle,
		deleteBranchOnMerge: opts.DeleteBranchOnMerge,
	}, nil
}

// Forge returns the forge this repository belongs to.
func (r *Repository) Forge() forge.Forge { return r.forge }

// SubmitChange creates a new pull request in the repository.
//
// This is a stub that will be implemented in a future PR.
func (r *Repository) SubmitChange(
	_ context.Context,
	_ forge.SubmitChangeRequest,
) (forge.SubmitChangeResult, error) {
	return forge.SubmitChangeResult{}, ErrNotImplemented
}

// EditChange edits an existing pull request.
//
// This is a stub that will be implemented in a future PR.
func (r *Repository) EditChange(
	_ context.Context,
	_ forge.ChangeID,
	_ forge.EditChangeOptions,
) error {
	return ErrNotImplemented
}

// MergeChange merges an open pull request.
//
// This is a stub that will be implemented in a future PR.
func (r *Repository) MergeChange(
	_ context.Context,
	_ forge.ChangeID,
	_ forge.MergeChangeOptions,
) error {
	return ErrNotImplemented
}

// FindChangesByBranch finds pull requests by branch name.
//
// This is a stub that will be implemented in a future PR.
func (r *Repository) FindChangesByBranch(
	_ context.Context,
	_ string,
	_ forge.FindChangesOptions,
) ([]*forge.FindChangeItem, error) {
	return nil, ErrNotImplemented
}

// FindChangeByID finds a pull request by its ID.
//
// This is a stub that will be implemented in a future PR.
func (r *Repository) FindChangeByID(
	_ context.Context,
	_ forge.ChangeID,
) (*forge.FindChangeItem, error) {
	return nil, ErrNotImplemented
}

// ChangeStatuses retrieves the statuses of multiple pull requests.
//
// This is a stub that will be implemented in a future PR.
func (r *Repository) ChangeStatuses(
	_ context.Context,
	_ []forge.ChangeID,
) ([]forge.ChangeStatus, error) {
	return nil, ErrNotImplemented
}

// ChangeChecksState reports the aggregate CI state for a pull request.
//
// This is a stub that will be implemented in a future PR.
func (r *Repository) ChangeChecksState(
	_ context.Context,
	_ forge.ChangeID,
) (forge.ChecksState, error) {
	return 0, ErrNotImplemented
}

// CommentCountsByChange reports comment counts for pull requests.
//
// This is a stub that will be implemented in a future PR.
func (r *Repository) CommentCountsByChange(
	_ context.Context,
	_ []forge.ChangeID,
) ([]*forge.CommentCounts, error) {
	return nil, ErrNotImplemented
}

// PostChangeComment posts a comment on a pull request.
//
// This is a stub that will be implemented in a future PR.
func (r *Repository) PostChangeComment(
	_ context.Context,
	_ forge.ChangeID,
	_ string,
) (forge.ChangeCommentID, error) {
	return nil, ErrNotImplemented
}

// UpdateChangeComment updates an existing comment.
//
// This is a stub that will be implemented in a future PR.
func (r *Repository) UpdateChangeComment(
	_ context.Context,
	_ forge.ChangeCommentID,
	_ string,
) error {
	return ErrNotImplemented
}

// DeleteChangeComment deletes an existing comment.
//
// This is a stub that will be implemented in a future PR.
func (r *Repository) DeleteChangeComment(
	_ context.Context,
	_ forge.ChangeCommentID,
) error {
	return ErrNotImplemented
}

// ListChangeComments lists comments on a pull request.
//
// This is a stub that will be implemented in a future PR.
func (r *Repository) ListChangeComments(
	_ context.Context,
	_ forge.ChangeID,
	_ *forge.ListChangeCommentsOptions,
) iter.Seq2[*forge.ListChangeCommentItem, error] {
	return func(yield func(*forge.ListChangeCommentItem, error) bool) {
		yield(nil, ErrNotImplemented)
	}
}

// NewChangeMetadata returns the metadata for a pull request.
//
// This is a stub that will be implemented in a future PR.
func (r *Repository) NewChangeMetadata(
	_ context.Context,
	_ forge.ChangeID,
) (forge.ChangeMetadata, error) {
	return nil, ErrNotImplemented
}

// ListChangeTemplates lists pull request templates in the repository.
//
// This is a stub that will be implemented in a future PR.
func (r *Repository) ListChangeTemplates(
	_ context.Context,
) ([]*forge.ChangeTemplate, error) {
	return nil, ErrNotImplemented
}
