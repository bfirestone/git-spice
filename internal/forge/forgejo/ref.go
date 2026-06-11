package forgejo

import (
	"fmt"
	"strconv"

	"go.abhg.dev/gs/internal/forge"
)

// RepositoryID identifies a Forgejo repository.
type RepositoryID struct {
	url   string // instance web URL, e.g. "https://codeberg.org"
	owner string // required
	name  string // required
}

var _ forge.RepositoryID = (*RepositoryID)(nil)

// String reports a human-readable name for the repository.
func (rid *RepositoryID) String() string {
	return rid.owner + "/" + rid.name
}

// ChangeURL returns the web URL for the given change ID
// hosted on the forge in this repository.
func (rid *RepositoryID) ChangeURL(id forge.ChangeID) string {
	return fmt.Sprintf("%s/%s/%s/pulls/%d",
		rid.url, rid.owner, rid.name, mustPR(id).Number)
}

// PR uniquely identifies a pull request in a Forgejo repository.
type PR struct {
	// Number is the pull request index, e.g. 123 for "#123".
	Number int64 `json:"number"` // required
}

var _ forge.ChangeID = (*PR)(nil)

func (id *PR) String() string {
	return "#" + strconv.FormatInt(id.Number, 10)
}

func mustPR(id forge.ChangeID) *PR {
	pr, ok := id.(*PR)
	if !ok {
		panic(fmt.Sprintf("forgejo: expected *PR, got %T", id))
	}
	return pr
}

// PRComment identifies a comment on a Forgejo pull request.
type PRComment struct {
	ID int64 `json:"id"` // required
}

var _ forge.ChangeCommentID = (*PRComment)(nil)

func (c *PRComment) String() string {
	return strconv.FormatInt(c.ID, 10)
}

func mustPRComment(id forge.ChangeCommentID) *PRComment {
	if id == nil {
		return nil
	}
	prc, ok := id.(*PRComment)
	if !ok {
		panic(fmt.Sprintf("forgejo: expected *PRComment, got %T", id))
	}
	return prc
}
