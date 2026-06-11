package forgejo

import (
	"context"
	"errors"
	"fmt"
	"iter"

	"go.abhg.dev/gs/internal/forge"
	forgejogw "go.abhg.dev/gs/internal/gateway/forgejo"
)

// PostChangeComment posts a new comment on a pull request.
func (r *Repository) PostChangeComment(
	ctx context.Context,
	id forge.ChangeID,
	body string,
) (forge.ChangeCommentID, error) {
	prNumber := mustPR(id).Number
	c, _, err := r.client.CommentCreate(ctx, r.owner, r.repo, prNumber, body)
	if err != nil {
		return nil, fmt.Errorf("post comment: %w", err)
	}

	r.log.Debug("Posted comment", "id", c.ID, "pr", prNumber)
	return &PRComment{ID: c.ID}, nil
}

// UpdateChangeComment updates the contents of an existing comment on a PR.
func (r *Repository) UpdateChangeComment(
	ctx context.Context,
	id forge.ChangeCommentID,
	body string,
) error {
	prc := mustPRComment(id)
	_, _, err := r.client.CommentUpdate(ctx, r.owner, r.repo, prc.ID, body)
	if err != nil {
		if errors.Is(err, forgejogw.ErrNotFound) {
			return fmt.Errorf("update comment: %w", forge.ErrNotFound)
		}
		return fmt.Errorf("update comment: %w", err)
	}

	r.log.Debug("Updated comment", "id", prc.ID)
	return nil
}

// DeleteChangeComment deletes an existing comment on a pull request.
func (r *Repository) DeleteChangeComment(
	ctx context.Context,
	id forge.ChangeCommentID,
) error {
	prc := mustPRComment(id)
	_, err := r.client.CommentDelete(ctx, r.owner, r.repo, prc.ID)
	if err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}

	r.log.Debug("Deleted comment", "id", prc.ID)
	return nil
}

// _commentPageSize is the number of comments requested per page
// when paginating ListChangeComments results.
const _commentPageSize = 30

// ListChangeComments lists comments on a pull request,
// optionally applying the given filtering options.
// Pages of up to _commentPageSize comments are fetched
// until the server returns fewer than a full page.
func (r *Repository) ListChangeComments(
	ctx context.Context,
	id forge.ChangeID,
	opts *forge.ListChangeCommentsOptions,
) iter.Seq2[*forge.ListChangeCommentItem, error] {
	prNumber := mustPR(id).Number

	return func(yield func(*forge.ListChangeCommentItem, error) bool) {
		for page := 1; ; page++ {
			comments, _, err := r.client.CommentList(
				ctx, r.owner, r.repo, prNumber,
				&forgejogw.ListCommentsOptions{
					Page:  page,
					Limit: _commentPageSize,
				},
			)
			if err != nil {
				yield(nil, fmt.Errorf("list comments: %w", err))
				return
			}

			for _, c := range comments {
				if opts != nil {
					match := true
					for _, re := range opts.BodyMatchesAll {
						if !re.MatchString(c.Body) {
							match = false
							break
						}
					}
					if !match {
						continue
					}

					if opts.CanUpdate &&
						(c.User == nil || c.User.ID != r.userID) {
						continue
					}
				}

				item := &forge.ListChangeCommentItem{
					ID:   &PRComment{ID: c.ID},
					Body: c.Body,
				}
				if !yield(item, nil) {
					return
				}
			}

			// Stop if fewer results than requested (last page).
			if len(comments) < _commentPageSize {
				break
			}
		}
	}
}
