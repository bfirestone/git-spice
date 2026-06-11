package forgejo

import (
	"context"
	"errors"
	"fmt"

	"go.abhg.dev/gs/internal/forge"
	forgejogw "go.abhg.dev/gs/internal/gateway/forgejo"
	"go.abhg.dev/gs/internal/git"
)

// _findPageSize is the number of pull requests to request per page
// when paginating FindChangesByBranch results.
const _findPageSize = 30

// _findMaxPages is the maximum number of pages to fetch
// before stopping pagination.
const _findMaxPages = 10

// FindChangesByBranch searches for pull requests with the given head branch.
// Results are filtered client-side to match the branch name and,
// when opts.PushRepository is set, the head repository owner.
// Pages are fetched until opts.Limit matches are found,
// fewer results than the page size are returned (last page),
// or the page cap is reached.
func (r *Repository) FindChangesByBranch(
	ctx context.Context,
	branch string,
	opts forge.FindChangesOptions,
) ([]*forge.FindChangeItem, error) {
	if opts.Limit == 0 {
		opts.Limit = 10
	}

	var pushOwner string
	if opts.PushRepository != nil {
		pushOwner = mustRepositoryID(opts.PushRepository).owner
	}

	var changes []*forge.FindChangeItem
	for page := 1; page <= _findMaxPages; page++ {
		prs, _, err := r.client.PullRequestList(ctx, r.owner, r.repo,
			&forgejogw.ListPullRequestsOptions{
				State: stateFilter(opts.State),
				Limit: _findPageSize,
				Page:  page,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("list pull requests: %w", err)
		}

		for _, pr := range prs {
			if pr.Head == nil || pr.Head.Ref != branch {
				continue
			}
			if pushOwner != "" {
				if pr.Head.Repo == nil ||
					pr.Head.Repo.Owner == nil ||
					pr.Head.Repo.Owner.UserName != pushOwner {
					continue
				}
			}
			changes = append(changes, r.findChangeItem(pr))
			if len(changes) >= opts.Limit {
				return changes, nil
			}
		}

		// Stop if we received fewer results than requested (last page).
		if len(prs) < _findPageSize {
			break
		}
	}

	return changes, nil
}

// FindChangeByID fetches a single pull request by its numeric ID.
// Returns forge.ErrNotFound if the pull request does not exist.
func (r *Repository) FindChangeByID(
	ctx context.Context,
	id forge.ChangeID,
) (*forge.FindChangeItem, error) {
	pr, _, err := r.client.PullRequestGet(ctx, r.owner, r.repo, mustPR(id).Number)
	if err != nil {
		if errors.Is(err, forgejogw.ErrNotFound) {
			return nil, fmt.Errorf("find change by ID: %w", forge.ErrNotFound)
		}
		return nil, fmt.Errorf("find change by ID: %w", err)
	}

	return r.findChangeItem(pr), nil
}

// findChangeItem converts a Forgejo PullRequest to a forge.FindChangeItem.
// Draft status is detected from the WIP title prefix;
// the prefix is stripped from the returned Subject.
func (r *Repository) findChangeItem(pr *forgejogw.PullRequest) *forge.FindChangeItem {
	draft := isDraftTitle(pr.Title)
	subject := pr.Title
	if draft {
		subject = stripDraftPrefix(pr.Title)
	}

	var labels []string
	if len(pr.Labels) > 0 {
		labels = make([]string, len(pr.Labels))
		for i, lbl := range pr.Labels {
			labels[i] = lbl.Name
		}
	}

	var reviewers []string
	if len(pr.RequestedReviewers) > 0 {
		reviewers = make([]string, len(pr.RequestedReviewers))
		for i, u := range pr.RequestedReviewers {
			reviewers[i] = u.UserName
		}
	}

	var assignees []string
	if len(pr.Assignees) > 0 {
		assignees = make([]string, len(pr.Assignees))
		for i, u := range pr.Assignees {
			assignees[i] = u.UserName
		}
	}

	var baseName string
	if pr.Base != nil {
		baseName = pr.Base.Ref
	}

	var headHash git.Hash
	if pr.Head != nil {
		headHash = git.Hash(pr.Head.SHA)
	}

	return &forge.FindChangeItem{
		ID:        &PR{Number: pr.Index},
		URL:       pr.HTMLURL,
		State:     changeStateFrom(pr),
		Subject:   subject,
		HeadHash:  headHash,
		BaseName:  baseName,
		Draft:     draft,
		Labels:    labels,
		Reviewers: reviewers,
		Assignees: assignees,
	}
}
