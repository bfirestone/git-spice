package forgejo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.abhg.dev/gs/internal/forge"
	forgejogw "go.abhg.dev/gs/internal/gateway/forgejo"
)

func TestChangeStateFrom(t *testing.T) {
	tests := []struct {
		name string
		pr   *forgejogw.PullRequest
		want forge.ChangeState
	}{
		{
			name: "Merged",
			pr:   &forgejogw.PullRequest{Merged: true, State: "closed"},
			want: forge.ChangeMerged,
		},
		{
			name: "ClosedNotMerged",
			pr:   &forgejogw.PullRequest{Merged: false, State: "closed"},
			want: forge.ChangeClosed,
		},
		{
			name: "Open",
			pr:   &forgejogw.PullRequest{Merged: false, State: "open"},
			want: forge.ChangeOpen,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := changeStateFrom(tt.pr)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStateFilter(t *testing.T) {
	tests := []struct {
		name  string
		state forge.ChangeState
		want  string
	}{
		{
			name:  "Open",
			state: forge.ChangeOpen,
			want:  "open",
		},
		{
			name:  "Closed",
			state: forge.ChangeClosed,
			want:  "closed",
		},
		{
			name:  "Merged",
			state: forge.ChangeMerged,
			want:  "closed",
		},
		{
			name:  "All",
			state: 0,
			want:  "all",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stateFilter(tt.state)
			assert.Equal(t, tt.want, got)
		})
	}
}
