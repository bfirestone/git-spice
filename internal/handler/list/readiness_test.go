package list

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.abhg.dev/gs/internal/forge"
	"go.abhg.dev/gs/internal/forge/shamhub"
)

func TestClassify(t *testing.T) {
	id := func(n int) forge.ChangeID { return shamhub.ChangeID(n) }

	tests := []struct {
		name   string
		input  ClassifyInput
		want   Readiness
		wantBy forge.ChangeID
	}{
		{
			name:  "NoChange_Unsubmitted",
			input: ClassifyInput{Change: nil},
			want:  ReadinessUnsubmitted,
		},
		{
			name: "ClosedNotMerged_Unsubmitted",
			input: ClassifyInput{
				Change: &ClassifyChange{ID: id(1), State: forge.ChangeClosed},
			},
			want: ReadinessUnsubmitted,
		},
		{
			name: "Merged",
			input: ClassifyInput{
				Change: &ClassifyChange{ID: id(1), State: forge.ChangeMerged},
			},
			want: ReadinessMerged,
		},
		{
			name: "Draft_EvenIfParentMerged",
			input: ClassifyInput{
				Change: &ClassifyChange{ID: id(2), State: forge.ChangeOpen, Draft: true},
				Ancestors: []ClassifyAncestor{{
					Change: &ClassifyChange{ID: id(1), State: forge.ChangeMerged},
				}},
			},
			want: ReadinessDraft,
		},
		{
			name: "BlockedBySubmittedParent",
			input: ClassifyInput{
				Change: &ClassifyChange{ID: id(2), State: forge.ChangeOpen},
				Ancestors: []ClassifyAncestor{{
					Change: &ClassifyChange{ID: id(1), State: forge.ChangeOpen},
				}},
			},
			want:   ReadinessBlocked,
			wantBy: id(1),
		},
		{
			name: "ReadyWhenParentMerged",
			input: ClassifyInput{
				Change: &ClassifyChange{ID: id(2), State: forge.ChangeOpen},
				Ancestors: []ClassifyAncestor{{
					Change: &ClassifyChange{ID: id(1), State: forge.ChangeMerged},
				}},
			},
			want: ReadinessReady,
		},
		{
			name: "ReadyOnTrunk",
			input: ClassifyInput{
				Change: &ClassifyChange{ID: id(1), State: forge.ChangeOpen},
			},
			want: ReadinessReady,
		},
		{
			name: "UnsubmittedAncestorIgnored_FurtherOpenAncestorBlocks",
			input: ClassifyInput{
				Change: &ClassifyChange{ID: id(3), State: forge.ChangeOpen},
				Ancestors: []ClassifyAncestor{
					{Change: nil}, // unsubmitted parent
					{Change: &ClassifyChange{ID: id(1), State: forge.ChangeOpen}},
				},
			},
			want:   ReadinessBlocked,
			wantBy: id(1),
		},
		{
			name: "MergedAncestorSkipped_FurtherOpenAncestorBlocks",
			input: ClassifyInput{
				Change: &ClassifyChange{ID: id(3), State: forge.ChangeOpen},
				Ancestors: []ClassifyAncestor{
					{Change: &ClassifyChange{ID: id(2), State: forge.ChangeMerged}},
					{Change: &ClassifyChange{ID: id(1), State: forge.ChangeOpen}},
				},
			},
			want:   ReadinessBlocked,
			wantBy: id(1),
		},
		{
			name: "ClosedAncestorSkipped_ReadyWhenNoOthers",
			input: ClassifyInput{
				Change: &ClassifyChange{ID: id(2), State: forge.ChangeOpen},
				Ancestors: []ClassifyAncestor{{
					Change: &ClassifyChange{ID: id(1), State: forge.ChangeClosed},
				}},
			},
			want: ReadinessReady,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, by := Classify(tt.input)
			assert.Equal(t, tt.want, got)
			if tt.wantBy != nil {
				assert.Equal(t, tt.wantBy, by)
			} else {
				assert.Nil(t, by)
			}
		})
	}
}
