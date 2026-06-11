package forgejo

import (
	"encoding/json"
	"fmt"

	"go.abhg.dev/gs/internal/forge"
)

// PRMetadata is the per-change metadata
// persisted to the state store alongside branch state.
type PRMetadata struct {
	PR *PR `json:"pr,omitempty"`

	NavigationComment *PRComment `json:"navigation_comment,omitempty"`
}

var _ forge.ChangeMetadata = (*PRMetadata)(nil)

// MarshalChangeMetadata serializes a PRMetadata into JSON.
func (*Forge) MarshalChangeMetadata(
	md forge.ChangeMetadata,
) (json.RawMessage, error) {
	return json.Marshal(md)
}

// UnmarshalChangeMetadata deserializes a PRMetadata from JSON.
func (*Forge) UnmarshalChangeMetadata(
	data json.RawMessage,
) (forge.ChangeMetadata, error) {
	var md PRMetadata
	if err := json.Unmarshal(data, &md); err != nil {
		return nil, fmt.Errorf("unmarshal PR metadata: %w", err)
	}
	return &md, nil
}

// MarshalChangeID serializes a PR into JSON.
func (*Forge) MarshalChangeID(cid forge.ChangeID) (json.RawMessage, error) {
	return json.Marshal(mustPR(cid))
}

// UnmarshalChangeID deserializes a PR from JSON.
func (*Forge) UnmarshalChangeID(data json.RawMessage) (forge.ChangeID, error) {
	var pr PR
	if err := json.Unmarshal(data, &pr); err != nil {
		return nil, fmt.Errorf("unmarshal PR: %w", err)
	}
	return &pr, nil
}

// ForgeID reports the forge that owns this metadata.
func (*PRMetadata) ForgeID() string { return "forgejo" }

// ChangeID reports the change this metadata refers to.
func (m *PRMetadata) ChangeID() forge.ChangeID { return m.PR }

// NavigationCommentID reports the navigation comment on the change,
// or nil if there isn't one.
func (m *PRMetadata) NavigationCommentID() forge.ChangeCommentID {
	if m.NavigationComment == nil {
		return nil
	}
	return m.NavigationComment
}

// SetNavigationCommentID sets the navigation comment on the metadata.
// The ID may be nil to clear it.
func (m *PRMetadata) SetNavigationCommentID(id forge.ChangeCommentID) {
	if id == nil {
		m.NavigationComment = nil
		return
	}
	m.NavigationComment = mustPRComment(id)
}
