package list

import "go.abhg.dev/gs/internal/forge"

// Readiness is the local classification of a branch's merge-readiness.
// It is computed entirely from information already present in the
// branch and change store, and never performs live API calls.
type Readiness int

const (
	// ReadinessUnknown is the zero value, meaning classification has not
	// been performed. Renderers should not draw a badge in this state.
	ReadinessUnknown Readiness = iota

	// ReadinessUnsubmitted marks a branch that is tracked but has no
	// open change, or whose change has been closed without merging.
	ReadinessUnsubmitted

	// ReadinessMerged marks a branch whose change has been merged.
	ReadinessMerged

	// ReadinessDraft marks a branch whose change is a draft.
	ReadinessDraft

	// ReadinessBlocked marks a branch whose change is open but a
	// non-merged submitted downstack ancestor still blocks merge order.
	ReadinessBlocked

	// ReadinessReady marks a branch that is submitted, not a draft, and
	// whose every submitted ancestor is merged (or has none).
	ReadinessReady
)

// ClassifyChange is the subset of change information the classifier
// requires about a branch's own change or about any ancestor change.
type ClassifyChange struct {
	ID    forge.ChangeID
	State forge.ChangeState
	Draft bool
}

// ClassifyAncestor captures what the classifier cares about for a
// single downstack ancestor: whether it has a submitted change, and
// what state that change is in.
type ClassifyAncestor struct {
	// Change is the ancestor's change metadata, or nil if unsubmitted.
	Change *ClassifyChange
}

// ClassifyInput is the minimum information the classifier needs about
// a single branch and its downstack ancestry.
type ClassifyInput struct {
	// Change is the branch's own change, or nil if unsubmitted.
	Change *ClassifyChange

	// Ancestors lists downstack ancestors in order from closest parent
	// to furthest. The trunk itself is not included.
	Ancestors []ClassifyAncestor
}

// Classify returns the Readiness for a single branch and, when
// Readiness == ReadinessBlocked, the ChangeID of the nearest blocking
// ancestor. The returned ChangeID is nil for every other Readiness.
func Classify(in ClassifyInput) (Readiness, forge.ChangeID) {
	if in.Change == nil {
		return ReadinessUnsubmitted, nil
	}
	switch in.Change.State {
	case forge.ChangeMerged:
		return ReadinessMerged, nil
	case forge.ChangeClosed:
		return ReadinessUnsubmitted, nil
	}
	if in.Change.Draft {
		return ReadinessDraft, nil
	}
	for _, a := range in.Ancestors {
		if a.Change == nil {
			continue
		}
		switch a.Change.State {
		case forge.ChangeMerged, forge.ChangeClosed:
			continue
		}
		return ReadinessBlocked, a.Change.ID
	}
	return ReadinessReady, nil
}
