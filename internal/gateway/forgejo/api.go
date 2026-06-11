package forgejo

import (
	"context"
	"net/url"
	"strconv"
	"strings"
)

// User is a Forgejo user.
type User struct {
	ID       int64  `json:"id"`
	UserName string `json:"login"`
}

// Label is a Forgejo issue label.
type Label struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// PRBranch identifies one side of a pull request.
type PRBranch struct {
	Ref  string      `json:"ref"`
	SHA  string      `json:"sha"`
	Repo *Repository `json:"repo,omitempty"` // head repo; may be nil for base branches
}

// PullRequest is a Forgejo pull request.
type PullRequest struct {
	Index              int64     `json:"number"`
	Title              string    `json:"title"`
	Body               string    `json:"body"`
	State              string    `json:"state"` // "open" or "closed"
	Merged             bool      `json:"merged"`
	HTMLURL            string    `json:"html_url"`
	Base               *PRBranch `json:"base"`
	Head               *PRBranch `json:"head"`
	Labels             []*Label  `json:"labels"`
	Assignees          []*User   `json:"assignees"`
	RequestedReviewers []*User   `json:"requested_reviewers"`
	Comments           int64     `json:"comments"`
}

// Repository is a Forgejo repository.
type Repository struct {
	ID                int64  `json:"id"`
	Owner             *User  `json:"owner"`
	Name              string `json:"name"`
	FullName          string `json:"full_name"`
	DefaultBranch     string `json:"default_branch"`
	DefaultMergeStyle string `json:"default_merge_style"`
}

// Comment is a comment on a Forgejo issue or pull request.
type Comment struct {
	ID   int64  `json:"id"`
	Body string `json:"body"`
	User *User  `json:"user"`
}

// CommitStatus is a single commit status check.
type CommitStatus struct {
	Status string `json:"status"`
}

// CombinedStatus is the aggregate commit status for a ref.
type CombinedStatus struct {
	State      string          `json:"state"` // pending|success|error|failure|warning
	TotalCount int64           `json:"total_count"`
	Statuses   []*CommitStatus `json:"statuses"`
}

// ContentsResponse is a file fetched from a repository.
type ContentsResponse struct {
	Name     string `json:"name"`
	Content  string `json:"content"`  // base64-encoded
	Encoding string `json:"encoding"` // "base64"
}

// CreatePullRequestOptions specifies fields for a new pull request.
type CreatePullRequestOptions struct {
	Title     string   `json:"title"`
	Body      string   `json:"body,omitempty"`
	Base      string   `json:"base"`
	Head      string   `json:"head"`
	Labels    []int64  `json:"labels,omitempty"`
	Assignees []string `json:"assignees,omitempty"`
}

// UpdatePullRequestOptions specifies fields to change on a pull request.
type UpdatePullRequestOptions struct {
	Title     *string  `json:"title,omitempty"`
	Body      *string  `json:"body,omitempty"`
	Base      *string  `json:"base,omitempty"`
	Labels    []int64  `json:"labels,omitempty"`
	Assignees []string `json:"assignees,omitempty"`
}

// ListPullRequestsOptions filters listed pull requests.
type ListPullRequestsOptions struct {
	State string // "open", "closed", "all"
	Limit int
	Page  int
}

func (o *ListPullRequestsOptions) encodeQuery() url.Values {
	values := make(url.Values)
	if o == nil {
		return values
	}
	if o.State != "" {
		values.Set("state", o.State)
	}
	if o.Limit != 0 {
		values.Set("limit", strconv.Itoa(o.Limit))
	}
	if o.Page != 0 {
		values.Set("page", strconv.Itoa(o.Page))
	}
	return values
}

// MergePullRequestOptions controls a pull request merge.
type MergePullRequestOptions struct {
	Do                     string `json:"Do"` // required; merge|rebase|rebase-merge|squash
	HeadCommitID           string `json:"head_commit_id,omitempty"`
	DeleteBranchAfterMerge bool   `json:"delete_branch_after_merge,omitempty"`
}

// ListCommentsOptions filters listed comments.
type ListCommentsOptions struct {
	Page  int // 1-based page number; 0 means unset
	Limit int // page size; 0 means unset
}

func (o *ListCommentsOptions) encodeQuery() url.Values {
	values := make(url.Values)
	if o == nil {
		return values
	}
	if o.Page != 0 {
		values.Set("page", strconv.Itoa(o.Page))
	}
	if o.Limit != 0 {
		values.Set("limit", strconv.Itoa(o.Limit))
	}
	return values
}

// UserCurrent fetches the authenticated Forgejo user.
//
// Forgejo API:
// https://codeberg.org/api/swagger#/user/userGetCurrent
func (c *Client) UserCurrent(ctx context.Context) (*User, *Response, error) {
	var response User
	resp, err := c.get(ctx, "user", nil, &response)
	if err != nil {
		return nil, resp, err
	}
	return &response, resp, nil
}

// RepoGet fetches a single repository.
//
// Forgejo API:
// https://codeberg.org/api/swagger#/repository/repoGet
func (c *Client) RepoGet(
	ctx context.Context,
	owner, repo string,
) (*Repository, *Response, error) {
	var response Repository
	resp, err := c.get(
		ctx,
		"repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo),
		nil,
		&response,
	)
	if err != nil {
		return nil, resp, err
	}
	return &response, resp, nil
}

// PullRequestCreate creates a new pull request.
//
// Forgejo API:
// https://codeberg.org/api/swagger#/issue/repoCreatePullRequest
func (c *Client) PullRequestCreate(
	ctx context.Context,
	owner, repo string,
	opt *CreatePullRequestOptions,
) (*PullRequest, *Response, error) {
	var response PullRequest
	resp, err := c.post(
		ctx,
		"repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+"/pulls",
		nil,
		opt,
		&response,
	)
	if err != nil {
		return nil, resp, err
	}
	return &response, resp, nil
}

// PullRequestGet fetches a single pull request.
//
// Forgejo API:
// https://codeberg.org/api/swagger#/issue/repoGetPullRequest
func (c *Client) PullRequestGet(
	ctx context.Context,
	owner, repo string,
	index int64,
) (*PullRequest, *Response, error) {
	var response PullRequest
	resp, err := c.get(
		ctx,
		"repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+
			"/pulls/"+strconv.FormatInt(index, 10),
		nil,
		&response,
	)
	if err != nil {
		return nil, resp, err
	}
	return &response, resp, nil
}

// PullRequestList lists pull requests for a repository.
//
// Forgejo API:
// https://codeberg.org/api/swagger#/issue/repoListPullRequests
func (c *Client) PullRequestList(
	ctx context.Context,
	owner, repo string,
	opt *ListPullRequestsOptions,
) ([]*PullRequest, *Response, error) {
	var response []*PullRequest
	resp, err := c.get(
		ctx,
		"repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+"/pulls",
		opt.encodeQuery(),
		&response,
	)
	if err != nil {
		return nil, resp, err
	}
	return response, resp, nil
}

// PullRequestUpdate updates a pull request.
//
// Forgejo API:
// https://codeberg.org/api/swagger#/issue/repoEditPullRequest
func (c *Client) PullRequestUpdate(
	ctx context.Context,
	owner, repo string,
	index int64,
	opt *UpdatePullRequestOptions,
) (*PullRequest, *Response, error) {
	var response PullRequest
	resp, err := c.patch(
		ctx,
		"repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+
			"/pulls/"+strconv.FormatInt(index, 10),
		nil,
		opt,
		&response,
	)
	if err != nil {
		return nil, resp, err
	}
	return &response, resp, nil
}

// PullRequestMerge merges a pull request.
//
// Forgejo API:
// https://codeberg.org/api/swagger#/issue/repoMergePullRequest
func (c *Client) PullRequestMerge(
	ctx context.Context,
	owner, repo string,
	index int64,
	opt *MergePullRequestOptions,
) (*Response, error) {
	return c.post(
		ctx,
		"repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+
			"/pulls/"+strconv.FormatInt(index, 10)+"/merge",
		nil,
		opt,
		nil,
	)
}

// ReviewerRequest requests reviewers on a pull request.
//
// Forgejo API:
// https://codeberg.org/api/swagger#/issue/repoCreatePullReviewRequests
func (c *Client) ReviewerRequest(
	ctx context.Context,
	owner, repo string,
	index int64,
	reviewers []string,
) (*Response, error) {
	return c.post(
		ctx,
		"repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+
			"/pulls/"+strconv.FormatInt(index, 10)+"/requested_reviewers",
		nil,
		struct {
			Reviewers []string `json:"reviewers"`
		}{Reviewers: reviewers},
		nil,
	)
}

// CommentCreate creates a comment on an issue or pull request.
//
// Forgejo API:
// https://codeberg.org/api/swagger#/issue/issueCreateComment
func (c *Client) CommentCreate(
	ctx context.Context,
	owner, repo string,
	index int64,
	body string,
) (*Comment, *Response, error) {
	var response Comment
	resp, err := c.post(
		ctx,
		"repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+
			"/issues/"+strconv.FormatInt(index, 10)+"/comments",
		nil,
		struct {
			Body string `json:"body"`
		}{Body: body},
		&response,
	)
	if err != nil {
		return nil, resp, err
	}
	return &response, resp, nil
}

// CommentUpdate edits an existing issue comment.
//
// Forgejo API:
// https://codeberg.org/api/swagger#/issue/issueEditComment
func (c *Client) CommentUpdate(
	ctx context.Context,
	owner, repo string,
	commentID int64,
	body string,
) (*Comment, *Response, error) {
	var response Comment
	resp, err := c.patch(
		ctx,
		"repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+
			"/issues/comments/"+strconv.FormatInt(commentID, 10),
		nil,
		struct {
			Body string `json:"body"`
		}{Body: body},
		&response,
	)
	if err != nil {
		return nil, resp, err
	}
	return &response, resp, nil
}

// CommentDelete deletes an issue comment.
//
// Forgejo API:
// https://codeberg.org/api/swagger#/issue/issueDeleteComment
func (c *Client) CommentDelete(
	ctx context.Context,
	owner, repo string,
	commentID int64,
) (*Response, error) {
	return c.delete(
		ctx,
		"repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+
			"/issues/comments/"+strconv.FormatInt(commentID, 10),
		nil,
	)
}

// CommentList lists comments on an issue or pull request.
//
// Forgejo API:
// https://codeberg.org/api/swagger#/issue/issueGetComments
func (c *Client) CommentList(
	ctx context.Context,
	owner, repo string,
	index int64,
	opt *ListCommentsOptions,
) ([]*Comment, *Response, error) {
	var response []*Comment
	resp, err := c.get(
		ctx,
		"repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+
			"/issues/"+strconv.FormatInt(index, 10)+"/comments",
		opt.encodeQuery(),
		&response,
	)
	if err != nil {
		return nil, resp, err
	}
	return response, resp, nil
}

// CommitStatusGet fetches the combined commit status for a ref.
//
// Forgejo API:
// https://codeberg.org/api/swagger#/repository/repoGetCombinedStatusByRef
func (c *Client) CommitStatusGet(
	ctx context.Context,
	owner, repo, ref string,
) (*CombinedStatus, *Response, error) {
	var response CombinedStatus
	resp, err := c.get(
		ctx,
		"repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+
			"/commits/"+escapeFilePath(ref)+"/status",
		nil,
		&response,
	)
	if err != nil {
		return nil, resp, err
	}
	return &response, resp, nil
}

// LabelList lists labels for a repository.
//
// Forgejo API:
// https://codeberg.org/api/swagger#/issue/issueListLabels
func (c *Client) LabelList(
	ctx context.Context,
	owner, repo string,
) ([]*Label, *Response, error) {
	var response []*Label
	resp, err := c.get(
		ctx,
		"repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+"/labels",
		nil,
		&response,
	)
	if err != nil {
		return nil, resp, err
	}
	return response, resp, nil
}

// LabelCreate creates a label in a repository.
// A neutral default color is used as the color field is required by the API.
//
// Forgejo API:
// https://codeberg.org/api/swagger#/issue/issueCreateLabel
func (c *Client) LabelCreate(
	ctx context.Context,
	owner, repo, name string,
) (*Label, *Response, error) {
	var response Label
	resp, err := c.post(
		ctx,
		"repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+"/labels",
		nil,
		struct {
			Name  string `json:"name"`
			Color string `json:"color"`
		}{Name: name, Color: "#cccccc"},
		&response,
	)
	if err != nil {
		return nil, resp, err
	}
	return &response, resp, nil
}

// ContentsGet fetches the contents of a file from a repository.
// Path segments within the file path are individually URL-escaped
// so that slashes remain as path separators.
//
// Forgejo API:
// https://codeberg.org/api/swagger#/repository/repoGetContents
func (c *Client) ContentsGet(
	ctx context.Context,
	owner, repo, path string,
) (*ContentsResponse, *Response, error) {
	var response ContentsResponse
	resp, err := c.get(
		ctx,
		"repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+
			"/contents/"+escapeFilePath(path),
		nil,
		&response,
	)
	if err != nil {
		return nil, resp, err
	}
	return &response, resp, nil
}

// escapeFilePath escapes each segment of a repository file path individually,
// preserving slash separators so the URL path structure remains intact.
func escapeFilePath(path string) string {
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		segments[i] = url.PathEscape(seg)
	}
	return strings.Join(segments, "/")
}
