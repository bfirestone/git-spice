package forgejo

import (
	"cmp"
	"context"
	"fmt"

	"go.abhg.dev/gs/internal/forge"
	forgejogw "go.abhg.dev/gs/internal/gateway/forgejo"
	"go.abhg.dev/gs/internal/silog"
)

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
