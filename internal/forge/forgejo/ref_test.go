package forgejo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.abhg.dev/gs/internal/forge"
)

// --- Contract assertions ---
// These verify the design spec. Do NOT modify
// without updating the approved plan.

var (
	_ forge.RepositoryID    = (*RepositoryID)(nil)
	_ forge.ChangeID        = (*PR)(nil)
	_ forge.ChangeCommentID = (*PRComment)(nil)
	_ forge.ChangeMetadata  = (*PRMetadata)(nil)
)

func TestPR_String(t *testing.T) {
	assert.Equal(t, "#123", (&PR{Number: 123}).String())
}

func TestRepositoryID_String(t *testing.T) {
	rid := &RepositoryID{url: "https://codeberg.org", owner: "alice", name: "widget"}
	assert.Equal(t, "alice/widget", rid.String())
}

func TestRepositoryID_ChangeURL(t *testing.T) {
	rid := &RepositoryID{url: "https://codeberg.org", owner: "alice", name: "widget"}
	assert.Equal(t,
		"https://codeberg.org/alice/widget/pulls/42",
		rid.ChangeURL(&PR{Number: 42}))
}

func TestPRMetadata_navigationComment(t *testing.T) {
	var m PRMetadata
	assert.Nil(t, m.NavigationCommentID())
	m.SetNavigationCommentID(&PRComment{ID: 7})
	assert.Equal(t, "7", m.NavigationCommentID().String())
	m.SetNavigationCommentID(nil)
	assert.Nil(t, m.NavigationCommentID())
}
