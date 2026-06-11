package forgejo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.abhg.dev/gs/internal/forge"
)

func TestForge_ParseRepositoryPath(t *testing.T) {
	f := &Forge{}
	t.Run("Valid", func(t *testing.T) {
		rid, err := f.ParseRepositoryPath("/alice/widget.git")
		require.NoError(t, err)
		assert.Equal(t, "alice/widget", rid.String())
	})
	t.Run("NoOwner", func(t *testing.T) {
		_, err := f.ParseRepositoryPath("/widget")
		assert.ErrorIs(t, err, forge.ErrUnsupportedURL)
	})
}

func TestForge_ChangeID_roundTrip(t *testing.T) {
	f := &Forge{}
	raw, err := f.MarshalChangeID(&PR{Number: 42})
	require.NoError(t, err)
	id, err := f.UnmarshalChangeID(raw)
	require.NoError(t, err)
	assert.Equal(t, "#42", id.String())
}

func TestForge_ChangeMetadata_roundTrip(t *testing.T) {
	f := &Forge{}
	md := &PRMetadata{PR: &PR{Number: 42}, NavigationComment: &PRComment{ID: 7}}
	raw, err := f.MarshalChangeMetadata(md)
	require.NoError(t, err)
	got, err := f.UnmarshalChangeMetadata(raw)
	require.NoError(t, err)
	assert.Equal(t, md, got)
}

func TestForge_ChangeTemplatePaths(t *testing.T) {
	assert.ElementsMatch(t, []string{
		".forgejo/PULL_REQUEST_TEMPLATE.md",
		".gitea/PULL_REQUEST_TEMPLATE.md",
		".github/PULL_REQUEST_TEMPLATE.md",
		"PULL_REQUEST_TEMPLATE.md",
		"docs/PULL_REQUEST_TEMPLATE.md",
	}, (&Forge{}).ChangeTemplatePaths())
}
