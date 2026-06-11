package forgejo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsDraftTitle(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  bool
	}{
		{
			name:  "WIPColon",
			title: "WIP: fix the thing",
			want:  true,
		},
		{
			name:  "BracketWIP",
			title: "[WIP] fix the thing",
			want:  true,
		},
		{
			name:  "LowercaseWIPColon",
			title: "wip: fix the thing",
			want:  true,
		},
		{
			name:  "LowercaseBracketWIP",
			title: "[wip] fix the thing",
			want:  true,
		},
		{
			name:  "MixedCase",
			title: "Wip: fix the thing",
			want:  true,
		},
		{
			name:  "WipeNotDraft",
			title: "Wipe x",
			want:  false,
		},
		{
			name:  "BracketWIPNoSpace",
			title: "[WIP]nodraft",
			want:  false,
		},
		{
			name:  "NormalTitle",
			title: "Fix the thing",
			want:  false,
		},
		{
			name:  "Empty",
			title: "",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDraftTitle(tt.title)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAddDraftPrefix(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  string
	}{
		{
			name:  "PlainTitle",
			title: "fix the thing",
			want:  "WIP: fix the thing",
		},
		{
			name:  "AlreadyWIPColon",
			title: "WIP: fix the thing",
			want:  "WIP: fix the thing",
		},
		{
			name:  "AlreadyBracketWIP",
			title: "[WIP] fix the thing",
			want:  "[WIP] fix the thing",
		},
		{
			name:  "AlreadyLowercase",
			title: "wip: fix the thing",
			want:  "wip: fix the thing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := addDraftPrefix(tt.title)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStripDraftPrefix(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  string
	}{
		{
			name:  "WIPColon",
			title: "WIP: fix the thing",
			want:  "fix the thing",
		},
		{
			name:  "BracketWIP",
			title: "[WIP] fix the thing",
			want:  "fix the thing",
		},
		{
			name:  "LowercaseWIPColon",
			title: "wip: fix the thing",
			want:  "fix the thing",
		},
		{
			name:  "WIPColonNoSpace",
			title: "WIP:fix the thing",
			want:  "fix the thing",
		},
		{
			name:  "WIPColonAlone",
			title: "WIP:",
			want:  "",
		},
		{
			name:  "BracketWIPAlone",
			title: "[WIP]",
			want:  "",
		},
		{
			name:  "NotDraft",
			title: "fix the thing",
			want:  "fix the thing",
		},
		{
			name:  "Empty",
			title: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripDraftPrefix(tt.title)
			assert.Equal(t, tt.want, got)
		})
	}
}
