# git-spice Stacked-PR Ergonomics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver three independent PRs that make stacked-PR day-to-day operation on GitHub easier: (1) richer nav-comment with explicit `Depends on:` / `Next:` lines, (2) readiness classification column in `gs log short` / `gs log long`, (3) `gs repo sync --retarget` to restack and re-push child PRs after a parent merges.

**Architecture:** All three features sit above the existing forge abstraction — no new `forge.Repository` methods, no new persistent state. They compose existing primitives (`BranchGraph`, `EditChange`, `RestackHandler`, `autostash`, existing `branchtree` renderer). Each feature ships as a standalone PR; PR order is independent but presented here in spec order (nav → log → sync).

**Tech Stack:** Go 1.23+, Kong (CLI), gomock (unit tests), `testdata/script` txtar-style scripts driven by `mise run test:script`, ShamHub in-test forge, `changie` for changelog entries.

**Source design:** `docs/superpowers/specs/2026-04-21-git-spice-stacked-pr-ergonomics-design.md` (approved).

---

## Ground rules for every task

- **Testing first.** Write the failing test, run it, confirm failure, *then* implement. Follow `CLAUDE.md` testing conventions: `silog.Nop()` unless asserting log output, `testify/assert` + `require`, `gomock.NewController(t)` (no `defer ctrl.Finish()`), mocks declared next to first use.
- **Verify compilation after each meaningful edit** with `mcp__gopls__go_diagnostics` if available, or `mise run build`.
- **Commit per task** unless the task is explicitly "merge into previous commit". Each commit message follows the Conventional Commits style used in the project (see `git log --oneline -20`).
- **80-character soft line limit**, 120-char hard limit (from `CLAUDE.md`).
- **No new third-party dependencies.**
- **Changelog entries via `mise run changie new --kind $kind --body $body`** — each of the three features gets one entry at the end of its phase.

---

## Phase 1 — Nav-comment `Depends on:` / `Next:` lines (PR 1)

**Goal of this PR:** When git-spice posts or updates a stack navigation comment on a PR, include two new summary lines below the existing tree: `Depends on: #N` (nearest submitted non-merged downstack ancestor) and `Next: #M` (nearest submitted upstack descendant). Each line is omitted if the corresponding value is empty.

**Files touched:**

- Modify: `internal/handler/submit/nav_comment.go` (around line 502, inside `generateStackNavigationComment`)
- Modify: `internal/handler/submit/nav_comment_test.go` (extend existing table-driven test at lines 32–344, and potentially add new cases)
- Create: `testdata/script/stack_submit_navcomment_depends.txt` (end-to-end assertion)
- Create: `.changes/unreleased/Added-*.yaml` via `changie`

### Task 1.1: Write failing unit test cases for Depends / Next rendering

**Files:**
- Modify: `internal/handler/submit/nav_comment_test.go:32-344` (the existing `tests := []struct{...}` table)

- [ ] **Step 1: Read existing test structure**

Read `internal/handler/submit/nav_comment_test.go` lines 24–100 and 460–483 to internalize:
- The `trackedBranch` struct (fields `Name`, `Base`, `ChangeID`, `MergedDownstack`)
- The existing `wantComments map[int]string` pattern, where the value is the tree body with header/footer/marker stripped (see lines 466–467)
- The `joinLines(...)` helper at line 971

- [ ] **Step 2: Add three new test cases to the `tests` slice**

Add the following three cases to the existing table (the exact `tests := []struct{...}` literal around lines 32–344). Keep the struct field order consistent with the existing entries in the file.

```go
{
    name: "LinearStack_DependsAndNext",
    trackedBranches: []trackedBranch{
        {Name: "b1", ChangeID: 1},
        {Name: "b2", Base: "b1", ChangeID: 2},
        {Name: "b3", Base: "b2", ChangeID: 3},
    },
    submit: []string{"b1", "b2", "b3"},
    wantComments: map[int]string{
        // Middle PR #2: has both Depends on (#1) and Next (#3).
        2: joinLines(
            // existing tree lines (copy shape from the "LinearStack" case)
            // ... tree rendering ...
            "",
            "Depends on: #1",
            "Next: #3",
        ),
    },
},
{
    name: "BottomOfStack_OnlyNext",
    trackedBranches: []trackedBranch{
        {Name: "b1", ChangeID: 1},
        {Name: "b2", Base: "b1", ChangeID: 2},
    },
    submit: []string{"b1", "b2"},
    wantComments: map[int]string{
        // Bottom PR #1: Next only, no Depends on.
        1: joinLines(
            // ... tree ...
            "",
            "Next: #2",
        ),
    },
},
{
    name: "TopOfStack_OnlyDepends",
    trackedBranches: []trackedBranch{
        {Name: "b1", ChangeID: 1},
        {Name: "b2", Base: "b1", ChangeID: 2},
    },
    submit: []string{"b1", "b2"},
    wantComments: map[int]string{
        // Top PR #2: Depends on only, no Next.
        2: joinLines(
            // ... tree ...
            "",
            "Depends on: #1",
        ),
    },
},
```

**Note:** The exact tree lines for each case must match what `stacknav.Print` already emits. Rather than guess, copy the tree shape from the existing `LinearStack` test case (at roughly line 80–94) and append the new `Depends on` / `Next` lines. The `wantComments` string is compared *after* the header/footer/marker are stripped (see lines 466–467), so only the body text matters.

- [ ] **Step 3: Add two edge-case tests — unsubmitted gap and merged ancestor**

```go
{
    name: "UnsubmittedGap_SkipsPastToNearestSubmitted",
    trackedBranches: []trackedBranch{
        {Name: "b1", ChangeID: 1},
        {Name: "b2", Base: "b1"},                 // unsubmitted
        {Name: "b3", Base: "b2", ChangeID: 3},
    },
    submit: []string{"b1", "b3"},
    wantComments: map[int]string{
        // Top PR #3: nearest SUBMITTED ancestor is #1 (skips unsubmitted b2).
        3: joinLines(
            // ... tree ...
            "",
            "Depends on: #1",
        ),
    },
},
{
    name: "MergedAncestor_SkipsPastMerged",
    trackedBranches: []trackedBranch{
        {Name: "b2", Base: "trunk", ChangeID: 2, MergedDownstack: []int{1}},
        {Name: "b3", Base: "b2", ChangeID: 3},
    },
    submit: []string{"b2", "b3"},
    wantComments: map[int]string{
        // Top PR #3: its ancestor #2 is open, Depends on: #2.
        3: joinLines(
            // ... tree ...
            "",
            "Depends on: #2",
        ),
        // Middle PR #2: no unmerged submitted ancestor (trunk is not a PR),
        // merged ancestor #1 is skipped → no Depends on line.
        2: joinLines(
            // ... tree ...
            "",
            "Next: #3",
        ),
    },
},
```

- [ ] **Step 4: Run the new tests and verify they fail**

Run:

```bash
go test ./internal/handler/submit -run TestUpdateNavigationComments -v
```

Expected: All five new subtests FAIL because the implementation does not yet emit `Depends on:` or `Next:` lines. Failures should point to unexpected output that contains the tree but not the new lines.

If a *previously-passing* test fails, stop and reread — you may have mismatched `joinLines` shape or stripped markers incorrectly.

- [ ] **Step 5: Commit the failing tests (do not commit implementation yet)**

This commit intentionally leaves tests red. The next task turns them green in a single follow-up commit.

```bash
git add internal/handler/submit/nav_comment_test.go
git commit -m "test(submit): add nav-comment Depends on/Next expectations

Extends the existing table-driven test with five cases covering linear
stacks, top/bottom positions, unsubmitted gaps, and merged ancestors.
Tests fail until the implementation is added in the next commit."
```

### Task 1.2: Implement Depends on / Next rendering

**Files:**
- Modify: `internal/handler/submit/nav_comment.go` (insert inside `generateStackNavigationComment` around line 502)

- [ ] **Step 1: Read the current renderer**

Open `internal/handler/submit/nav_comment.go` at lines 474–520. Confirm:
- `generateStackNavigationComment(nodes []*stackedChange, current int, marker string, f forge.Forge) string` is the signature
- `stacknav.Print(&sb, nodes, current, opts)` is called at roughly line 500 and writes the tree
- The footer write starts at `sb.WriteString("\n")` / `sb.WriteString(footer)` at roughly line 503–504

Also read `stackedChange` at lines 433–453 to confirm:
- `Base int` (index into `nodes`, `-1` if none)
- `Aboves []int` (indices of upstack children)
- `String()` method emits the change reference (e.g., `#123`)

- [ ] **Step 2: Add two helpers inside `nav_comment.go`**

Add two unexported helpers directly above `generateStackNavigationComment`. They walk the pre-computed `nodes` graph and return the nearest *submitted, non-merged* ancestor/descendant node index, or `-1` if none.

```go
// nearestSubmittedAncestor returns the index of the closest downstack node
// that represents an open submitted change, skipping nodes that are merged
// or unsubmitted. Returns -1 if no such ancestor exists.
func nearestSubmittedAncestor(nodes []*stackedChange, current int) int {
    for i := nodes[current].Base; i >= 0; i = nodes[i].Base {
        if isOpenSubmitted(nodes[i]) {
            return i
        }
    }
    return -1
}

// nearestSubmittedDescendant returns the index of the closest upstack node
// that represents an open submitted change (breadth-first across Aboves).
// Returns -1 if no such descendant exists.
func nearestSubmittedDescendant(nodes []*stackedChange, current int) int {
    queue := append([]int(nil), nodes[current].Aboves...)
    for len(queue) > 0 {
        idx := queue[0]
        queue = queue[1:]
        if isOpenSubmitted(nodes[idx]) {
            return idx
        }
        queue = append(queue, nodes[idx].Aboves...)
    }
    return -1
}

// isOpenSubmitted reports whether the node represents a change that has
// been submitted and is not merged. A node whose Change is zero-valued is
// considered unsubmitted. The caller is responsible for not classifying
// merged changes as "submitted" in the node graph.
func isOpenSubmitted(n *stackedChange) bool {
    return n != nil && n.Change != nil
}
```

**Clarification on merged nodes.** Read the node-construction site at roughly line 141–160 to confirm: the existing code already *excludes* merged-into-trunk branches from the `nodes` array (they're tracked in `MergedDownstack` on `LoadBranchItem`, not reified as nodes). If that's correct, `isOpenSubmitted` does not need a merged check — any node present in `nodes` with a non-nil `Change` is open. Verify by reading lines 141–160 and adjust the comment on `isOpenSubmitted` accordingly. If merged nodes *are* included, extend `isOpenSubmitted` to check a merged flag on `stackedChange` (and add one if needed).

- [ ] **Step 3: Insert rendering block into `generateStackNavigationComment`**

At the location inside `generateStackNavigationComment` *after* the `stacknav.Print(&sb, nodes, current, opts)` call and *before* the existing `sb.WriteString("\n")` / `sb.WriteString(footer)` block (roughly line 502), insert:

```go
// Summary lines: "Depends on: #N" and "Next: #M".
// Each line is omitted if the corresponding ancestor/descendant is absent.
var hasDeps, hasNext bool
if i := nearestSubmittedAncestor(nodes, current); i >= 0 {
    if !hasDeps {
        sb.WriteString("\n")
    }
    sb.WriteString("Depends on: ")
    sb.WriteString(nodes[i].String())
    sb.WriteString("\n")
    hasDeps = true
}
if i := nearestSubmittedDescendant(nodes, current); i >= 0 {
    if !hasDeps && !hasNext {
        sb.WriteString("\n")
    }
    sb.WriteString("Next: ")
    sb.WriteString(nodes[i].String())
    sb.WriteString("\n")
    hasNext = true
}
```

The blank-line logic ensures a single blank line separates the tree from the summary lines when at least one is rendered, and no extra blank line if both are absent.

- [ ] **Step 4: Run the new tests and verify they pass**

```bash
go test ./internal/handler/submit -run TestUpdateNavigationComments -v
```

Expected: all subtests pass, including the five new ones. If `LinearStack_DependsAndNext` passes but `UnsubmittedGap_SkipsPastToNearestSubmitted` fails with `Next: #2` appearing unexpectedly, you're walking `Aboves` into a node that has `Change == nil` — confirm `isOpenSubmitted` is correct.

- [ ] **Step 5: Run the full submit-package tests and confirm no regressions**

```bash
go test ./internal/handler/submit/...
```

Expected: all existing tests still pass. If `updateNavigationComments` cases fail due to tree-only assertions now observing extra summary lines, the pre-existing `wantComments` entries need an empty `Next:` / `Depends on:` line appended wherever applicable. Update only the cases that now semantically have a submitted ancestor/descendant. Unsubmitted / single-PR cases should be unaffected.

- [ ] **Step 6: Commit**

```bash
git add internal/handler/submit/nav_comment.go internal/handler/submit/nav_comment_test.go
git commit -m "feat(submit): add Depends on/Next lines to stack nav-comment

Renders the nearest submitted non-merged downstack ancestor and upstack
descendant as explicit 'Depends on:' and 'Next:' lines below the
existing stack tree. Each line is omitted if the corresponding value is
empty (bottom of stack has no Depends on; top has no Next).

Fixes the reviewer use case: understanding stack position from a PR
alone, without the stack tree being the only signal."
```

### Task 1.3: End-to-end script test

**Files:**
- Create: `testdata/script/stack_submit_navcomment_depends.txt`

- [ ] **Step 1: Pattern-match an existing script test**

Read `testdata/script/stack_submit.txt` end-to-end (under 200 lines). Note the setup block (`shamhub init`, `shamhub register`, `git push origin main`), the `gs branch create` / `gs stack submit` flow, and how comments are asserted with `shamhub dump comments`.

- [ ] **Step 2: Write the new script test**

Create `testdata/script/stack_submit_navcomment_depends.txt`. Use the `test-script` skill (Skill(test-script)) conventions if there's ambiguity. The script should:

1. Set up ShamHub + a 3-branch linear stack.
2. `gs stack submit` all three.
3. `shamhub dump comments` and `cmp stdout $WORK/golden/comments.txt`.
4. Embed an inline golden file at the bottom (after `-- golden/comments.txt --`) containing the expected nav-comment bodies for all three PRs. The middle PR's comment must include both `Depends on: #1` and `Next: #3`. The bottom PR must include only `Next: #2`. The top PR must include only `Depends on: #2`.

Full working skeleton (adapt `trackedBranch` names and content to match existing conventions in `stack_submit.txt`):

```
# Verify the nav-comment includes Depends on/Next summary lines.

as 'Test User <test@example.com>'
at '2026-04-21T12:00:00Z'

cd repo
git init
git commit --allow-empty -m 'Initial commit'
shamhub init
shamhub new origin alice/example.git
shamhub register alice
git push origin main

# Create three stacked branches, each with one commit.
gs branch create feat-a -m 'feat a'
gs branch create feat-b -m 'feat b'
gs branch create feat-c -m 'feat c'

# Submit the whole stack at once.
gs stack submit --fill

# The three PRs should have nav-comments with correct Depends/Next lines.
shamhub dump comments
cmp stdout $WORK/golden/comments.txt

-- repo/.gitignore --
-- golden/comments.txt --
# (fill in with the actual expected shamhub dump comments output after
# running the test once with --update; see the "verify then capture"
# pattern below in Step 3)
```

- [ ] **Step 3: Run with `--update` to capture the golden file**

```bash
mise run test:script --run stack_submit_navcomment_depends --update
```

This runs the script and writes observed output into the `-- golden/comments.txt --` block. Inspect the updated file manually and verify the expected nav-comment lines (`Depends on: #N`, `Next: #M`) appear in the right places.

- [ ] **Step 4: Re-run without `--update` and confirm it passes**

```bash
mise run test:script --run stack_submit_navcomment_depends
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add testdata/script/stack_submit_navcomment_depends.txt
git commit -m "test(script): verify Depends on/Next in stack nav-comment

End-to-end: 3-branch stack → submit → assert all three PR nav-comments
contain the expected Depends on/Next lines at each stack position."
```

### Task 1.4: Changelog entry and PR 1 completion

- [ ] **Step 1: Add changelog entry**

```bash
mise run changie new --kind Added --body "submit: Show 'Depends on' and 'Next' lines in stack navigation comment"
```

- [ ] **Step 2: Run the full-phase verification**

```bash
mise run fmt
mise run lint
mise run build
go test ./internal/handler/submit/...
mise run test:script --run 'stack_submit_navcomment.*'
```

All must pass. Fix any lint or build issues inline before committing.

- [ ] **Step 3: Commit the changelog**

```bash
git add .changes/unreleased
git commit -m "chore: changelog for nav-comment Depends on/Next"
```

- [ ] **Step 4: PR 1 ready for review. Stop here before starting Phase 2.**

---

## Phase 2 — Readiness classification in `gs log short` / `gs log long` (PR 2)

**Goal of this PR:** Every branch line in `gs log short` and `gs log long` shows a readiness badge: one of `ready`, `blocked by #N`, `draft`, `merged`, `unsubmitted`. Classification is purely local (no live API calls). Always-on, no flag.

**Files touched:**

- Create: `internal/handler/list/readiness.go` (classifier)
- Create: `internal/handler/list/readiness_test.go` (table-driven unit test)
- Modify: `internal/handler/list/` — the `Item` type that `branchtree.Item` is built from (inspect during Task 2.2)
- Modify: `log.go` — the `graphLogPresenter` that builds `branchtree.Item` (lines ~167–264)
- Modify: `log_short.go` / `log_long.go` only if a divergent renderer is needed (likely untouched)
- Create: `testdata/script/log_short_readiness.txt`
- Changelog entry

### Task 2.1: Classifier with unit tests (TDD)

**Files:**
- Create: `internal/handler/list/readiness.go`
- Create: `internal/handler/list/readiness_test.go`

- [ ] **Step 1: Write the failing unit test**

Create `internal/handler/list/readiness_test.go`:

```go
package list

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "go.abhg.dev/gs/internal/forge"
    "go.abhg.dev/gs/internal/spice"
)

func TestClassify(t *testing.T) {
    // Minimal fakes; adjust imports/types to the real spice.LoadBranchItem
    // and forge.ChangeMetadata interfaces (read branch.go:384-410 for the
    // exact shape before writing this test).

    trunkName := "main"

    tests := []struct {
        name   string
        item   ClassifyInput
        want   Readiness
        wantBy forge.ChangeID // non-nil when Readiness == Blocked
    }{
        {
            name: "NoChange_Unsubmitted",
            item: ClassifyInput{Branch: "feat-a", Base: trunkName, Change: nil},
            want: ReadinessUnsubmitted,
        },
        {
            name: "Merged",
            item: ClassifyInput{
                Branch: "feat-a", Base: trunkName,
                Change: &fakeChange{state: forge.ChangeMerged, id: fakeID(1)},
            },
            want: ReadinessMerged,
        },
        {
            name: "Draft_EvenIfParentMerged",
            item: ClassifyInput{
                Branch: "feat-b", Base: "feat-a",
                Change: &fakeChange{state: forge.ChangeOpen, draft: true, id: fakeID(2)},
                Ancestors: []ClassifyAncestor{{
                    Change: &fakeChange{state: forge.ChangeMerged, id: fakeID(1)},
                }},
            },
            want: ReadinessDraft,
        },
        {
            name: "BlockedBySubmittedParent",
            item: ClassifyInput{
                Branch: "feat-b", Base: "feat-a",
                Change: &fakeChange{state: forge.ChangeOpen, id: fakeID(2)},
                Ancestors: []ClassifyAncestor{{
                    Change: &fakeChange{state: forge.ChangeOpen, id: fakeID(1)},
                }},
            },
            want:   ReadinessBlocked,
            wantBy: fakeID(1),
        },
        {
            name: "ReadyWhenParentMerged",
            item: ClassifyInput{
                Branch: "feat-b", Base: "feat-a",
                Change: &fakeChange{state: forge.ChangeOpen, id: fakeID(2)},
                Ancestors: []ClassifyAncestor{{
                    Change: &fakeChange{state: forge.ChangeMerged, id: fakeID(1)},
                }},
            },
            want: ReadinessReady,
        },
        {
            name: "ReadyOnTrunk",
            item: ClassifyInput{
                Branch: "feat-a", Base: trunkName,
                Change: &fakeChange{state: forge.ChangeOpen, id: fakeID(1)},
            },
            want: ReadinessReady,
        },
        {
            name: "UnsubmittedAncestor_IgnoredWhenClassifyingCurrent",
            item: ClassifyInput{
                Branch: "feat-c", Base: "feat-b",
                Change: &fakeChange{state: forge.ChangeOpen, id: fakeID(3)},
                Ancestors: []ClassifyAncestor{
                    {Change: nil}, // feat-b unsubmitted
                    {Change: &fakeChange{state: forge.ChangeOpen, id: fakeID(1)}},
                },
            },
            // Any non-merged submitted ancestor blocks; feat-b unsubmitted
            // doesn't block, but the grandparent #1 does.
            want:   ReadinessBlocked,
            wantBy: fakeID(1),
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, by := Classify(tt.item)
            assert.Equal(t, tt.want, got)
            if tt.wantBy != nil {
                assert.Equal(t, tt.wantBy, by)
            } else {
                assert.Nil(t, by)
            }
        })
    }
}

// The following helpers MUST be adapted to the real interfaces.
// Before implementing, read:
//   - internal/spice/branch.go:384-410 for LoadBranchItem
//   - internal/forge/forge.go for ChangeMetadata and ChangeState
// Then define ClassifyInput / ClassifyAncestor so they expose only the
// fields the classifier actually needs. The fake types below are
// placeholders to be replaced with real test doubles.

type fakeChange struct {
    state forge.ChangeState
    draft bool
    id    forge.ChangeID
}

func (f *fakeChange) ChangeID() forge.ChangeID     { return f.id }
func (f *fakeChange) ChangeState() forge.ChangeState { return f.state }
func (f *fakeChange) IsDraft() bool                { return f.draft }

func fakeID(n int) forge.ChangeID {
    // Use the shamhub ChangeID constructor for simplicity; adjust per
    // the actual forge.ChangeID interface.
    return shamhubChangeID(n)
}
```

**Adaptation note.** Before running the test, read:
- `internal/spice/branch.go:384-410` for the real `LoadBranchItem` shape
- `internal/forge/forge.go` for the real `ChangeMetadata`, `ChangeID`, and `ChangeState` types (grep for `ChangeOpen`, `ChangeMerged`, `ChangeClosed`)
- `internal/forge/shamhub` for a real `ChangeID` constructor usable in tests

Replace `ClassifyInput`, `ClassifyAncestor`, `fakeChange`, and `fakeID` with types that line up with the real interfaces. The test *intent* — exercising each classifier rule once — is what matters; adjust the *shape* to fit reality.

- [ ] **Step 2: Run the test and verify compile failure then test failure**

```bash
go test ./internal/handler/list -run TestClassify -v
```

Expected: compile error because `Classify`, `ClassifyInput`, `Readiness`, `ReadinessReady`, etc. don't exist yet. That is the "red" state.

- [ ] **Step 3: Implement the classifier**

Create `internal/handler/list/readiness.go`:

```go
package list

import "go.abhg.dev/gs/internal/forge"

// Readiness is the local classification of a branch's merge-readiness.
// It is computed entirely from information already present in the
// branch/change store; it never performs live API calls.
type Readiness int

const (
    // ReadinessUnsubmitted: the branch is tracked but has no open change.
    ReadinessUnsubmitted Readiness = iota
    // ReadinessMerged: the branch's change has been merged.
    ReadinessMerged
    // ReadinessDraft: the branch's change is a draft PR.
    ReadinessDraft
    // ReadinessBlocked: the branch's change is open, but a non-merged
    // submitted downstack ancestor still blocks merge order.
    ReadinessBlocked
    // ReadinessReady: the branch is submitted, not a draft, and every
    // submitted ancestor is merged (or there is no submitted ancestor).
    ReadinessReady
)

// ClassifyInput is the minimum information the classifier needs about
// a single branch and its downstack ancestry.
type ClassifyInput struct {
    Branch string
    Base   string

    // Change is the branch's own change metadata, or nil if unsubmitted.
    Change ClassifyChange

    // Ancestors lists downstack ancestors in order from closest parent
    // to furthest. The trunk itself is NOT included.
    Ancestors []ClassifyAncestor
}

// ClassifyAncestor captures the subset of ancestor data the classifier
// cares about: whether it has a submitted change, and what state that
// change is in.
type ClassifyAncestor struct {
    Change ClassifyChange
}

// ClassifyChange is the subset of forge.ChangeMetadata that the
// classifier uses. Using a narrow interface keeps tests simple.
type ClassifyChange interface {
    ChangeID() forge.ChangeID
    ChangeState() forge.ChangeState
    IsDraft() bool
}

// Classify returns the Readiness for a single branch and the ChangeID
// of the blocking ancestor, if Readiness == ReadinessBlocked.
// The returned ChangeID is nil for every other Readiness value.
func Classify(in ClassifyInput) (Readiness, forge.ChangeID) {
    if in.Change == nil {
        return ReadinessUnsubmitted, nil
    }
    switch in.Change.ChangeState() {
    case forge.ChangeMerged:
        return ReadinessMerged, nil
    case forge.ChangeClosed:
        // Closed-not-merged: treat as unsubmitted (the design spec calls
        // this out explicitly — user must resubmit).
        return ReadinessUnsubmitted, nil
    }
    if in.Change.IsDraft() {
        return ReadinessDraft, nil
    }
    for _, a := range in.Ancestors {
        if a.Change == nil {
            continue // unsubmitted ancestor doesn't block
        }
        if a.Change.ChangeState() == forge.ChangeMerged {
            continue // merged ancestor doesn't block
        }
        return ReadinessBlocked, a.Change.ChangeID()
    }
    return ReadinessReady, nil
}
```

**Adaptation note.** Some method names above (`ChangeState()`, `IsDraft()`) may not exist verbatim on the real `forge.ChangeMetadata` interface. Before writing, read `internal/forge/forge.go` and align the `ClassifyChange` interface with *actual* method names. The rule chain logic stays identical.

- [ ] **Step 4: Run the test and verify pass**

```bash
go test ./internal/handler/list -run TestClassify -v
```

Expected: all subtests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/handler/list/readiness.go internal/handler/list/readiness_test.go
git commit -m "feat(list): add readiness classifier

Pure local classification of a branch's merge-readiness into one of
ready / blocked / draft / merged / unsubmitted. Does not perform any
live API calls; operates entirely on data already present in the
branch and change store.

Used in the next commit by 'gs log short' / 'gs log long' to surface
stack readiness inline with the log output."
```

### Task 2.2: Add readiness to `branchtree.Item` and render

**Files:**
- Modify: `internal/branchtree/` (the `Item` type — inspect the package during Step 1)
- Modify: `log.go` — `graphLogPresenter` around lines 167–264 (the loop that builds items from `res.Branches`)

- [ ] **Step 1: Read the branchtree renderer**

Open `internal/branchtree/` (find the file declaring `Item` and `Write`). Record:
- The `Item` struct fields
- Where/how the branch line is rendered (string template, prefix, etc.)
- Whether any "state" field already controls a symbol/color, and what style convention it uses

- [ ] **Step 2: Add a `Readiness` field to `branchtree.Item`**

Extend the struct with an optional `Readiness` field plus an optional `BlockedBy` ID. Keep zero-value backward-compatible: if `Readiness` is the zero value, the renderer must render exactly what it did before (no badge).

```go
type Item struct {
    // ... existing fields ...

    // Readiness is the local classification of this branch's
    // merge-readiness. Zero value means "do not render a badge".
    Readiness list.Readiness
    // BlockedBy is populated only when Readiness == ReadinessBlocked.
    BlockedBy forge.ChangeID
}
```

**Dependency direction note.** `branchtree` is a lower-level package than `list`. Importing `list` from `branchtree` may introduce a cycle. Two acceptable alternatives:

1. Redefine a local enum in `branchtree` and convert in the caller. Simpler, keeps layers clean.
2. Extract the `Readiness` enum to a neutral package (e.g., `internal/readiness`). More correct long-term.

Pick option 1 for this PR unless option 2 is trivially clean. If going with option 1, the `branchtree.Readiness` constants must mirror the `list.Readiness` values exactly; add a conversion function `fromList(r list.Readiness) Readiness` in the caller.

- [ ] **Step 3: Render the badge**

In the branchtree renderer, when `Readiness != zero`, prepend or append a colored symbol to the branch line. Suggested mapping:

| Readiness | Symbol | Suffix |
|---|---|---|
| Ready | `✔` (green) | empty |
| Blocked | `⏳` (yellow) | `blocked by #N` |
| Draft | `📝` (dim) | `draft` |
| Merged | `✅` (dim) | `merged` |
| Unsubmitted | `•` (dim) | empty |

If the existing renderer already prints the PR state (e.g., `#123 open`), the new badge should *replace* that state column, not duplicate it. Confirm by reading the current render call.

- [ ] **Step 4: Wire classifier into the log loop**

In `log.go`, inside the branch-iteration loop in `graphLogPresenter` (around line 184–234), after the `res.Branches[i]` / `LoadBranchItem` is in hand, compute the `ClassifyInput`:

```go
input := list.ClassifyInput{
    Branch: b.Name,
    Base:   b.Base,
    Change: b.Change, // adapt: may need to wrap or assert interface
}
// Walk downstack ancestors (excluding trunk) via BranchGraph.
for name := range graph.Downstack(b.Name) {
    if name == trunk {
        break
    }
    anc, _ := graph.Lookup(name)
    input.Ancestors = append(input.Ancestors, list.ClassifyAncestor{
        Change: anc.Change,
    })
}
readiness, blockedBy := list.Classify(input)

item := branchtree.Item{
    // ... existing fields ...
    Readiness: fromList(readiness),
    BlockedBy: blockedBy,
}
```

Exact location of the loop variables and the `graph` / `trunk` names must be read from `log.go:184-234`. Adapt variable names to whatever is in scope.

- [ ] **Step 5: Rebuild and run unit tests**

```bash
mise run build
go test ./internal/branchtree/... ./internal/handler/list/...
```

Expected: build succeeds, tests pass. If build fails with "undefined: forge.ChangeID" in `branchtree`, you need the import path — add it or move the `Readiness` conversion out of the lower package.

- [ ] **Step 6: Commit**

```bash
git add internal/branchtree internal/handler/list log.go
git commit -m "feat(log): show readiness badge in stack log

Every branch in 'gs log short' and 'gs log long' now renders one of
ready / blocked / draft / merged / unsubmitted, computed locally from
store state. Blocked branches show which change they are blocked by."
```

### Task 2.3: Script test for readiness rendering

**Files:**
- Create: `testdata/script/log_short_readiness.txt`

- [ ] **Step 1: Draft the script**

Build a 3-branch stack in ShamHub where:
- Bottom: open (not merged)
- Middle: draft
- Top: unsubmitted (no `gs branch submit`)

Run `gs log short`, compare stderr (or stdout — verify which `log short` writes to) against a golden file that shows `ready`, `draft`, and `unsubmitted` badges respectively.

Then merge the bottom PR via `shamhub merge-change`, run again, and compare against a second golden file: expect bottom=`merged`, middle=`ready` (now unblocked), top still `unsubmitted`.

- [ ] **Step 2: Capture goldens with `--update`**

```bash
mise run test:script --run log_short_readiness --update
```

- [ ] **Step 3: Manually verify the goldens look right, then re-run**

```bash
mise run test:script --run log_short_readiness
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add testdata/script/log_short_readiness.txt
git commit -m "test(script): verify readiness badges in gs log short"
```

### Task 2.4: Changelog + full-phase check

- [ ] **Step 1: Changelog entry**

```bash
mise run changie new --kind Added --body "log: Show readiness badge (ready/blocked/draft/merged/unsubmitted) per branch"
```

- [ ] **Step 2: Full-phase verification**

```bash
mise run fmt
mise run lint
mise run build
mise run generate
go test ./internal/handler/list/... ./internal/branchtree/...
mise run test:script --run 'log_short.*'
```

All must pass. `mise run generate` may update the CLI reference if any flag changed (this phase adds no flags, so it should be a no-op).

- [ ] **Step 3: Commit the changelog**

```bash
git add .changes/unreleased
git commit -m "chore: changelog for log readiness"
```

- [ ] **Step 4: PR 2 ready for review. Stop before starting Phase 3.**

---

## Phase 3 — `gs repo sync --retarget` + `--dry-run` (PR 3)

**Goal of this PR:** `gs repo sync` gains a `--retarget` flag (default `false`, configurable via `spice.sync.retarget`) that, when set, detects child branches whose parent just merged and transparently retargets them: local restack + forge `EditChange(Base: ...)` + force-with-lease push. A `--dry-run` flag prints the plan without executing.

**Files touched:**

- Modify: `repo_sync.go` — flag wiring
- Modify: `internal/handler/sync/handler.go` — insert retarget phase between merged-detect and merged-delete
- Create: `internal/handler/sync/retarget.go` — planner + executor
- Create: `internal/handler/sync/retarget_test.go` — planner unit tests
- Create: `testdata/script/repo_sync_retarget_basic.txt`
- Create: `testdata/script/repo_sync_retarget_dirty.txt`
- Create: `testdata/script/repo_sync_retarget_cascaded.txt`
- Modify: `doc/src/` — document new flag + config key
- Changelog entry

### Task 3.1: Add `Retarget` and `DryRun` flags + config key

**Files:**
- Modify: `repo_sync.go`
- Modify: `internal/handler/sync/handler.go` — `TrunkOptions` struct at lines 161–167

- [ ] **Step 1: Read existing flag patterns**

Read `repo_sync.go` (30 lines) and `internal/handler/sync/handler.go:161-167` for `TrunkOptions`. Confirm:
- `Restack bool` and `ClosedChanges ClosedChanges` are the two existing flags
- `ClosedChanges` has `config:"repoSync.closedChanges"` tag

- [ ] **Step 2: Add the two new fields to `TrunkOptions`**

In `internal/handler/sync/handler.go:161-167`, extend:

```go
type TrunkOptions struct {
    Restack       bool          `help:"Restack the current stack after syncing"`
    ClosedChanges ClosedChanges `default:"ask" config:"repoSync.closedChanges" enum:"ask,ignore" help:"..."`

    // Retarget, when true, restacks and re-pushes child branches whose
    // parent has just merged, updating their PR base on the forge.
    Retarget bool `config:"sync.retarget" help:"Restack and re-push children of merged branches"`

    // DryRun prints the retarget plan without executing it. Has no
    // effect unless Retarget is also true.
    DryRun bool `help:"Show what retarget would do without making changes"`
}
```

- [ ] **Step 3: Build and confirm no other code reads `TrunkOptions` in a way the new fields break**

```bash
mise run build
```

Expected: clean build. If build fails, fix inline.

- [ ] **Step 4: Commit**

```bash
git add repo_sync.go internal/handler/sync/handler.go
git commit -m "feat(repo sync): add --retarget and --dry-run flags

Flags are wired but not yet implemented. The next commits add the
planner and executor; until then, --retarget is silently a no-op.

Also adds the spice.sync.retarget config key (default false)."
```

### Task 3.2: Planner with unit tests (TDD)

**Files:**
- Create: `internal/handler/sync/retarget_test.go`
- Create: `internal/handler/sync/retarget.go` (planner half)

- [ ] **Step 1: Write failing planner tests**

Create `internal/handler/sync/retarget_test.go`:

```go
package sync

import (
    "testing"

    "github.com/stretchr/testify/assert"
)

func TestComputeRetargetPlan(t *testing.T) {
    // PlanInput and RetargetAction are defined in retarget.go.
    // Each test builds a synthetic stack state and asserts the plan.

    tests := []struct {
        name string
        in   PlanInput
        want []RetargetAction
    }{
        {
            name: "ParentMerged_ChildRetargetsToTrunk",
            in: PlanInput{
                Trunk: "main",
                Branches: []PlanBranch{
                    {Name: "b1", Base: "main", Merged: true, ChangeID: fakeID(1)},
                    {Name: "b2", Base: "b1", Merged: false, ChangeID: fakeID(2)},
                },
            },
            want: []RetargetAction{
                {Branch: "b2", OldBase: "b1", NewBase: "main", ChangeID: fakeID(2)},
            },
        },
        {
            name: "GrandparentMerged_ParentNotMerged_NoAction",
            in: PlanInput{
                Trunk: "main",
                Branches: []PlanBranch{
                    {Name: "b1", Base: "main", Merged: true, ChangeID: fakeID(1)},
                    {Name: "b2", Base: "b1", Merged: false, ChangeID: fakeID(2)},
                    {Name: "b3", Base: "b2", Merged: false, ChangeID: fakeID(3)},
                },
            },
            // b2 retargets to main. b3 stays on b2 (its parent wasn't merged).
            want: []RetargetAction{
                {Branch: "b2", OldBase: "b1", NewBase: "main", ChangeID: fakeID(2)},
            },
        },
        {
            name: "CascadedMerges_TwoGenerations",
            in: PlanInput{
                Trunk: "main",
                Branches: []PlanBranch{
                    {Name: "b1", Base: "main", Merged: true, ChangeID: fakeID(1)},
                    {Name: "b2", Base: "b1", Merged: true, ChangeID: fakeID(2)},
                    {Name: "b3", Base: "b2", Merged: false, ChangeID: fakeID(3)},
                },
            },
            // b3 retargets straight to main (nearest non-merged ancestor).
            want: []RetargetAction{
                {Branch: "b3", OldBase: "b2", NewBase: "main", ChangeID: fakeID(3)},
            },
        },
        {
            name: "UnsubmittedChild_LocalRetargetOnly",
            in: PlanInput{
                Trunk: "main",
                Branches: []PlanBranch{
                    {Name: "b1", Base: "main", Merged: true, ChangeID: fakeID(1)},
                    {Name: "b2", Base: "b1", Merged: false, ChangeID: nil},
                },
            },
            // b2 retargets locally but has no ChangeID → push step is
            // skipped by the executor. Plan still records the retarget.
            want: []RetargetAction{
                {Branch: "b2", OldBase: "b1", NewBase: "main", ChangeID: nil},
            },
        },
        {
            name: "NoMergedParents_EmptyPlan",
            in: PlanInput{
                Trunk: "main",
                Branches: []PlanBranch{
                    {Name: "b1", Base: "main", Merged: false, ChangeID: fakeID(1)},
                    {Name: "b2", Base: "b1", Merged: false, ChangeID: fakeID(2)},
                },
            },
            want: nil,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := ComputeRetargetPlan(tt.in)
            assert.Equal(t, tt.want, got)
        })
    }
}

// fakeID creates a ChangeID-compatible value for tests. Replace with
// the project's canonical test constructor (likely in forge/shamhub).
func fakeID(n int) forge.ChangeID { /* adapt */ return nil }
```

- [ ] **Step 2: Run the test and verify compile failure**

```bash
go test ./internal/handler/sync -run TestComputeRetargetPlan -v
```

Expected: compile error — `PlanInput`, `PlanBranch`, `RetargetAction`, `ComputeRetargetPlan` undefined.

- [ ] **Step 3: Implement the planner**

Create `internal/handler/sync/retarget.go`:

```go
package sync

import "go.abhg.dev/gs/internal/forge"

// PlanInput describes the local+merged state the planner operates on.
type PlanInput struct {
    Trunk    string
    Branches []PlanBranch
}

// PlanBranch is a flat view of a single tracked branch:
// its local base, whether the forge says it's merged,
// and its ChangeID (nil if unsubmitted).
type PlanBranch struct {
    Name     string
    Base     string
    Merged   bool
    ChangeID forge.ChangeID
}

// RetargetAction is one planned base change for a single branch.
// OldBase is the current local base; NewBase is the nearest non-merged
// ancestor (or Trunk). ChangeID is the branch's PR ID, or nil if
// the branch is not submitted (in which case the executor must NOT
// attempt a forge EditChange or force-push).
type RetargetAction struct {
    Branch   string
    OldBase  string
    NewBase  string
    ChangeID forge.ChangeID
}

// ComputeRetargetPlan returns the list of retargets implied by in.
// A branch requires a retarget if its current Base refers to a merged
// branch. NewBase is found by walking Base links toward Trunk and
// picking the first non-merged ancestor (Trunk itself if the entire
// chain is merged).
func ComputeRetargetPlan(in PlanInput) []RetargetAction {
    byName := make(map[string]PlanBranch, len(in.Branches))
    for _, b := range in.Branches {
        byName[b.Name] = b
    }

    var out []RetargetAction
    for _, b := range in.Branches {
        if b.Name == in.Trunk {
            continue
        }
        parent, ok := byName[b.Base]
        if !ok || !parent.Merged {
            continue // base is trunk or not merged → nothing to do
        }
        // Walk toward trunk.
        newBase := in.Trunk
        for cur := parent; cur.Merged; {
            if cur.Base == in.Trunk {
                newBase = in.Trunk
                break
            }
            next, ok := byName[cur.Base]
            if !ok {
                newBase = in.Trunk
                break
            }
            if !next.Merged {
                newBase = next.Name
                break
            }
            cur = next
        }
        out = append(out, RetargetAction{
            Branch:   b.Name,
            OldBase:  b.Base,
            NewBase:  newBase,
            ChangeID: b.ChangeID,
        })
    }
    return out
}
```

- [ ] **Step 4: Run the planner tests**

```bash
go test ./internal/handler/sync -run TestComputeRetargetPlan -v
```

Expected: all subtests pass. Fix any assertion mismatches by adjusting the planner (not the tests).

- [ ] **Step 5: Commit**

```bash
git add internal/handler/sync/retarget.go internal/handler/sync/retarget_test.go
git commit -m "feat(sync): add retarget planner

Pure function ComputeRetargetPlan that, given local branches and their
merged status, returns the list of base changes needed to retarget
children of merged branches to the nearest non-merged ancestor.

Planner is TDD-tested in isolation; the executor and wiring land in
subsequent commits."
```

### Task 3.3: Executor + integration into `SyncTrunk`

**Files:**
- Modify: `internal/handler/sync/retarget.go` (add executor)
- Modify: `internal/handler/sync/handler.go` (call into the executor between merged-detect and merged-delete)

- [ ] **Step 1: Read the SyncTrunk sequence**

Reread `internal/handler/sync/handler.go:174–455`. Identify:
- Line ~390: `findLocalMergedBranches` or equivalent
- Line ~396: `findForgeFinishedBranches` or equivalent
- Line ~400: end of merged-detection phase
- Lines 402–434: the branch-deletion phase
- Line 208: `h.Autostash.BeginAutostash(...)` — already running before any destructive work

The insertion point is **between line 400 and line 402** — after merged branches are known, before they're deleted.

- [ ] **Step 2: Add `ExecuteRetargetPlan` to `retarget.go`**

Append to `internal/handler/sync/retarget.go`:

```go
// ExecutorDeps is the subset of handler services the retarget executor
// needs. Declaring them as an interface-like struct of function values
// keeps the executor trivially testable.
type ExecutorDeps struct {
    // LocalRebase restacks Branch onto NewBase in the local worktree.
    // Must leave the working tree clean on success.
    LocalRebase func(ctx context.Context, branch, newBase string) error

    // UpdateStoreBase persists the new base in git-spice's state store.
    UpdateStoreBase func(ctx context.Context, branch, newBase string) error

    // EditChangeBase calls EditChange on the appropriate forge for the
    // given change, setting its base branch to newBase. The executor
    // skips this step when changeID == nil.
    EditChangeBase func(ctx context.Context, id forge.ChangeID, newBase string) error

    // PushBranch force-with-lease pushes Branch to its upstream.
    PushBranch func(ctx context.Context, branch string) error

    // Log is the logger for plan/progress output.
    Log *silog.Logger
}

// ExecuteRetargetPlan runs the planned retargets in order. Each action
// is treated independently: a failure in one action does not prevent
// others from being attempted. Returns the first error observed; if
// any action failed, the caller should treat the overall result as
// partial and surface the count of failures.
func ExecuteRetargetPlan(ctx context.Context, deps ExecutorDeps, plan []RetargetAction, dryRun bool) (failed int, firstErr error) {
    for _, a := range plan {
        if dryRun {
            deps.Log.Infof("retarget %s #%v: %s -> %s (dry-run)",
                a.Branch, a.ChangeID, a.OldBase, a.NewBase)
            continue
        }
        if err := deps.LocalRebase(ctx, a.Branch, a.NewBase); err != nil {
            failed++
            if firstErr == nil {
                firstErr = fmt.Errorf("retarget %s: rebase: %w", a.Branch, err)
            }
            deps.Log.Warnf("skip %s: rebase failed: %v", a.Branch, err)
            continue
        }
        if err := deps.UpdateStoreBase(ctx, a.Branch, a.NewBase); err != nil {
            failed++
            if firstErr == nil {
                firstErr = fmt.Errorf("retarget %s: store: %w", a.Branch, err)
            }
            continue
        }
        if a.ChangeID != nil {
            if err := deps.EditChangeBase(ctx, a.ChangeID, a.NewBase); err != nil {
                failed++
                if firstErr == nil {
                    firstErr = fmt.Errorf("retarget %s: edit change: %w", a.Branch, err)
                }
                continue
            }
            if err := deps.PushBranch(ctx, a.Branch); err != nil {
                failed++
                if firstErr == nil {
                    firstErr = fmt.Errorf("retarget %s: push: %w", a.Branch, err)
                }
                continue
            }
        }
        deps.Log.Infof("retargeted %s: %s -> %s", a.Branch, a.OldBase, a.NewBase)
    }
    return failed, firstErr
}
```

Add the required imports (`context`, `fmt`, `go.abhg.dev/gs/internal/silog`).

- [ ] **Step 3: Wire the executor into `SyncTrunk`**

In `internal/handler/sync/handler.go`, between the end of merged-detection (line ~400) and the start of branch deletion (line ~402), insert:

```go
if opts.Retarget {
    plan := ComputeRetargetPlan(PlanInput{
        Trunk:    trunk, // whatever variable holds the trunk name in scope
        Branches: buildPlanBranches(branches, mergedSet),
    })
    if len(plan) == 0 {
        h.Log.Info("retarget: nothing to do")
    } else {
        h.Log.Infof("retarget: %d action(s) planned", len(plan))
        deps := ExecutorDeps{
            LocalRebase:     h.restackBranchOnto, // adapt to real helper
            UpdateStoreBase: h.Store.UpdateBranchBase, // adapt
            EditChangeBase:  h.editChangeBase,    // adapt
            PushBranch:      h.pushBranch,        // adapt
            Log:             h.Log,
        }
        failed, err := ExecuteRetargetPlan(ctx, deps, plan, opts.DryRun)
        if err != nil {
            // Do not abort — continue to deletion phase. The spec says
            // partial failure reports non-zero exit; the caller in
            // repo_sync.go surfaces that.
            h.Log.Warnf("retarget: %d action(s) failed; continuing", failed)
            if retErr == nil {
                retErr = err // return the first error at function end
            }
        }
    }
}
```

**`buildPlanBranches` helper.** Add a small private helper in `retarget.go` that converts the existing `branches []LoadBranchItem` + `mergedSet map[string]bool` into `[]PlanBranch`:

```go
func buildPlanBranches(branches []spice.LoadBranchItem, merged map[string]bool) []PlanBranch {
    out := make([]PlanBranch, 0, len(branches))
    for _, b := range branches {
        pb := PlanBranch{Name: b.Name, Base: b.Base, Merged: merged[b.Name]}
        if b.Change != nil {
            pb.ChangeID = b.Change.ChangeID()
        }
        out = append(out, pb)
    }
    return out
}
```

Adapt to the real types (`branches` may be `[]*LoadBranchItem`, and the merged set may be keyed differently — inspect the existing sync handler to confirm).

- [ ] **Step 4: Hook up the four executor dependencies**

The four function fields (`LocalRebase`, `UpdateStoreBase`, `EditChangeBase`, `PushBranch`) need backing implementations on the handler. The concrete implementations already exist elsewhere in the codebase; the handler just needs thin adapters. Read:

- `internal/handler/restack/handler.go:78-79` for the single-branch restack
- `internal/spice/state/branch.go` for the store's update-base API (grep for `UpdateBase` or `SetBase`)
- `internal/forge/forge.go:236` for `EditChange` and its `EditChangeOptions.Base` field
- Existing push logic in `branch_submit.go` / `internal/handler/submit/` for the force-with-lease push helper

If a helper is not already on `h *Handler`, add it as a private method. Keep the diff localized.

- [ ] **Step 5: Build and run all sync tests**

```bash
mise run build
go test ./internal/handler/sync/...
```

Expected: PASS. No existing test should change behavior, because `opts.Retarget` is `false` by default.

- [ ] **Step 6: Commit**

```bash
git add internal/handler/sync
git commit -m "feat(sync): retarget children of merged branches

Adds an executor that runs a RetargetPlan: local rebase, update store
base, EditChange on the forge, force-with-lease push. Wired into
SyncTrunk behind opts.Retarget, with opts.DryRun skipping all side
effects. Failures are per-action: one branch's failure does not abort
the others; the first error is returned from SyncTrunk."
```

### Task 3.4: Script test — basic retarget

**Files:**
- Create: `testdata/script/repo_sync_retarget_basic.txt`

- [ ] **Step 1: Draft the script**

Pattern-match an existing `repo_sync*.txt` script (look in `testdata/script/`). The script should:

1. Set up ShamHub and a 3-branch linear stack, submit all three.
2. Merge the bottom PR via ShamHub.
3. Run `gs repo sync --retarget --dry-run` — assert the plan mentions `b2` retargeting to `main`.
4. Run `gs repo sync --retarget` — assert success.
5. `shamhub dump changes` and verify `b2`'s PR has `base = main` and `b3`'s PR still has `base = b2`.

- [ ] **Step 2: Capture golden and re-run**

```bash
mise run test:script --run repo_sync_retarget_basic --update
mise run test:script --run repo_sync_retarget_basic
```

Second invocation must PASS without `--update`.

- [ ] **Step 3: Commit**

```bash
git add testdata/script/repo_sync_retarget_basic.txt
git commit -m "test(script): basic repo sync --retarget"
```

### Task 3.5: Script test — dirty worktree refusal

**Files:**
- Create: `testdata/script/repo_sync_retarget_dirty.txt`

- [ ] **Step 1: Draft the script**

Same setup as basic. After merging the bottom PR, dirty the worktree (add an uncommitted change), run `gs repo sync --retarget`. `autostash` should either stash + restore cleanly (in which case the test asserts success and the stash restoration), or refuse (in which case the test asserts the refusal error and a non-zero exit).

The behavior here depends on how `autostash` is configured at the sync level. Read `internal/handler/autostash/handler.go` and the caller in `SyncTrunk` to see which mode applies to `repo sync`. The script test should assert *whatever the existing behavior is* for `repo sync` with a dirty tree (autostash almost certainly stashes transparently); the design's statement that `--retarget` *refuses* dirty trees should be softened to "inherits existing repo sync autostash behavior".

**If autostash is transparent**: the script asserts the retarget succeeds end-to-end and the stash is reapplied.

**If autostash refuses or prompts**: the script asserts the refusal and non-zero exit.

Update the design doc (`docs/superpowers/specs/2026-04-21-git-spice-stacked-pr-ergonomics-design.md`) if the reality diverges from the stated design — prefer adjusting docs to match code behavior.

- [ ] **Step 2: Capture / run / commit**

```bash
mise run test:script --run repo_sync_retarget_dirty --update
mise run test:script --run repo_sync_retarget_dirty
git add testdata/script/repo_sync_retarget_dirty.txt
git commit -m "test(script): repo sync --retarget with dirty worktree"
```

### Task 3.6: Script test — cascaded merges

**Files:**
- Create: `testdata/script/repo_sync_retarget_cascaded.txt`

- [ ] **Step 1: Draft and run**

1. 3-branch linear stack; submit all.
2. Merge the bottom *and* the middle in ShamHub (back-to-back).
3. `gs repo sync --retarget`.
4. Assert the top PR (`b3`) now has `base = main` directly.

- [ ] **Step 2: Capture / run / commit**

```bash
mise run test:script --run repo_sync_retarget_cascaded --update
mise run test:script --run repo_sync_retarget_cascaded
git add testdata/script/repo_sync_retarget_cascaded.txt
git commit -m "test(script): repo sync --retarget with cascaded merges"
```

### Task 3.7: Documentation

**Files:**
- Modify: `doc/src/` — find the `repo sync` page (grep `doc/src -l "repo sync"`)

- [ ] **Step 1: Document the new flags and config key**

Add a subsection to the `repo sync` docs that:
- Introduces `--retarget` (purpose, default, forge support)
- Introduces `--dry-run`
- Documents `spice.sync.retarget` (bool, default `false`, link to "configuring git-spice" page)
- Includes one end-to-end example: "after merging the bottom PR of a 3-deep stack, run `gs repo sync --retarget`; the child PRs' base branches are updated on the forge and their local branches are restacked".

Use the `<!-- gs:version unreleased -->` marker per `CLAUDE.md`.

- [ ] **Step 2: Regenerate CLI reference**

```bash
mise run generate
```

This updates `doc/includes/cli-reference.md` to reflect the new flags.

- [ ] **Step 3: Commit**

```bash
git add doc
git commit -m "docs(repo sync): document --retarget and --dry-run flags"
```

### Task 3.8: Changelog and final check

- [ ] **Step 1: Changelog entry**

```bash
mise run changie new --kind Added --body "repo sync: Add --retarget to restack and re-push child PRs after a parent merges"
```

- [ ] **Step 2: Full verification**

```bash
mise run fmt
mise run lint
mise run build
mise run generate
go test ./internal/handler/sync/... ./internal/handler/list/... ./internal/handler/submit/...
mise run test:script --run 'repo_sync_retarget.*'
mise run test:script --run 'stack_submit_navcomment.*'
mise run test:script --run 'log_short_readiness.*'
```

All must pass.

- [ ] **Step 3: Commit changelog**

```bash
git add .changes/unreleased
git commit -m "chore: changelog for repo sync --retarget"
```

- [ ] **Step 4: PR 3 ready for review.**

---

## Cross-cutting wrap-up

- Three PRs, each ready to submit independently.
- Every PR has: feature tests (unit + script), changelog entry, docs updated if a flag changed, `mise run fmt`/`lint`/`build`/`generate` clean.
- Forge coverage is uniform: all features sit above the existing forge abstraction. Non-GitHub forges (GitLab, Bitbucket) get the features for free — spot-check by running the existing integration tests for each forge if the CI config includes them; do not add new per-forge test variants in this initiative.

## Known risks and follow-ups (not in scope for these PRs)

- **Eventual default flip of `spice.sync.retarget` to `true`.** Ship a follow-up issue proposing the flip after a few releases of bake-in.
- **Live "mergeable" status in readiness**. The design deliberately excludes this; a future enhancement could add a `--checks` flag to `gs log short` that fetches live CI/review state. Not in scope here.
- **Multi-trunk repositories** (repos with more than one trunk, e.g., release branches). The planner assumes a single trunk; behavior on multi-trunk repos is undefined. Out of scope.
