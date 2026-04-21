# git-spice stacked-PR ergonomics: design

**Date:** 2026-04-21
**Scope:** Three additive enhancements to git-spice, delivered as three
independent PRs.
**Source spec:** `git-spice-enhancement-spec.md` (user-supplied, 2026-04-20).

## Goals

Make stacked-PR day-to-day operation easier without building new
services, UIs, or persistent state:

1. Reviewers can understand the stack from a single submitted PR alone.
2. Users can see at-a-glance which PR is ready to merge from the CLI.
3. After a lower PR merges, child PRs retarget and push cleanly in one
   command.

## Non-goals

- No new service, UI, GitHub App, or hosted component.
- No new persistent state; reuse the existing branch/change store.
- No new methods on the `forge.Repository` interface.
- No live "mergeable" computation from GitHub (spec explicit: noisy,
  expensive, overpromising).
- No merge automation, no multi-trunk orchestration, no cross-stack
  retargeting.

## Context from the current codebase

Key findings from the pre-design survey that shape the design:

- **Navigation comments already exist.** `internal/handler/submit/nav_comment.go`
  posts and updates a per-PR stack-tree comment wrapped in
  `<!-- gs:navigation comment -->` markers. This supersedes the source
  spec's Feature 1 ("managed PR body block"): the user value is already
  delivered by the existing comment. Work shifts to *enriching* that
  comment rather than adding a parallel body-annotation system.
- **Branch → PR mapping is in the store.** `internal/spice/state/branch.go`
  persists forge metadata per branch. `internal/spice/branch_graph.go`
  provides `BranchGraph` with `.Aboves()` / `.Below()` traversal.
- **Merged downstack is already tracked.** `LoadBranchItem.MergedDownstack`
  (`internal/spice/branch.go`) records merged descendants.
- **`log short` / `log long` already fetch PR state.** They pull PR
  status, comment counts, and push-sync status. Adding a readiness badge
  is incremental — no new fetching.
- **`repo sync` already deletes merged-PR branches** but does not
  retarget or push children. That's the remaining gap.
- **All forges (`github`, `gitlab`, `bitbucket`, `shamhub`) already
  implement `EditChange`** — which supports updating a PR's base branch.
  No forge-layer work is needed for retarget.

## Architecture summary

All three features live above the forge abstraction. They compose
existing primitives:

```
┌─────────────────────────────────────────────────────────────┐
│  CLI: branch/stack/downstack/upstack submit | log | repo    │
└─────────────┬────────────────┬──────────────┬───────────────┘
              │                │              │
     ┌────────▼───────┐  ┌─────▼─────┐  ┌─────▼──────────┐
     │ nav-comment    │  │ list/     │  │ sync/          │
     │ renderer       │  │ readiness │  │ retarget       │
     │ (F1)           │  │ (F2)      │  │ planner (F3)   │
     └────────┬───────┘  └─────┬─────┘  └─────┬──────────┘
              │                │              │
              └────────────────┼──────────────┘
                               │
                   ┌───────────▼───────────┐
                   │  spice.BranchGraph    │
                   │  + walkSubmitted*     │ ← new helpers
                   │  (shared)             │
                   └───────────┬───────────┘
                               │
                   ┌───────────▼───────────┐
                   │  forge.Repository     │
                   │  (unchanged interface)│
                   └───────────────────────┘
```

**Shared helpers (new).** `internal/spice/branch_graph.go` gains two
small helpers used by F1 and F2:

- `walkSubmittedAncestors(branch)` — iterator yielding the nearest
  submitted non-merged downstack ancestors, skipping unsubmitted and
  merged branches.
- `walkSubmittedDescendants(branch)` — mirror, upstack.

Both terminate at trunk / stack boundary, respect the existing cycle
guard in `BranchGraph`, and return an empty iterator on no match.

## Feature 1 — Nav-comment `Depends on` / `Next` lines

**Where it lives.** `internal/handler/submit/nav_comment.go`
(`generateStackNavigationComment()`).

**What it adds.** After the existing stack tree, two optional summary
lines:

```
Depends on: #123
Next: #125
```

Values come from `walkSubmittedAncestors` / `walkSubmittedDescendants`.
Each line is omitted if its value is empty.

**Rendering rules.**

- Lines live inside the existing `<!-- gs:navigation comment -->`
  markers so existing update machinery covers them.
- `Depends on:` = PR number of nearest submitted, non-merged downstack
  ancestor.
- `Next:` = PR number of nearest submitted upstack descendant.
- Neither line renders if empty — no dangling labels.
- Markdown format; no per-node badges (those belong in the CLI, where
  data is fresh).

**Edge cases.**

- Current PR is bottom of stack → only `Next:` renders.
- Current PR is top of stack → only `Depends on:` renders.
- Unsubmitted gap in ancestors → skip past it until a submitted
  non-merged PR is found.
- Merged ancestor → treated as "below the waterline"; skip past to next
  unmerged submitted PR. (A PR whose parent has merged shows no
  `Depends on:` if the only ancestor was that merged PR.)
- Single-PR stack on trunk → neither line renders.

**Tests.**

- Unit tests in `nav_comment_test.go` for: single PR, 2-deep stack,
  3-deep with current in middle, unsubmitted gap, merged parent, top of
  stack, bottom of stack.
- Script test `stack_submit_navcomment_depends.txt` asserts both lines
  appear in all submitted PRs' nav-comments after a resubmit.

**Config / flags.** None. Reuses existing `spice.submit.navComment`
gating.

**Changelog kind.** `Added`. Example body:
`submit: Show "Depends on" and "Next" lines in stack navigation
comment`.

## Feature 2 — Readiness classification in `log short` / `log long`

**Where it lives.**

- Classifier: new file `internal/handler/list/readiness.go`.
- Renderer: `log_short.go` and `log_long.go`.
- Data source: existing `LoadBranchItem` from `internal/handler/list/`
  (already includes change metadata and `MergedDownstack`).

**Classifier.**

```go
type Readiness int

const (
    ReadinessUnsubmitted Readiness = iota
    ReadinessMerged
    ReadinessDraft
    ReadinessBlocked
    ReadinessReady
)

func Classify(item LoadBranchItem, graph *BranchGraph) Readiness
```

**Rules** (first match wins):

1. No change record → `Unsubmitted`.
2. Change record reports merged → `Merged`.
3. Change record reports draft → `Draft`.
4. Any non-trunk downstack ancestor is submitted and not merged →
   `Blocked`.
5. Otherwise → `Ready`.

Pure function. No forge calls. No network. Deterministic from inputs.

**Rendering.**

- Short form: render a single symbol + optional `blocked by #N` suffix
  on the existing branch line.
  - `✔ ready`, `⏳ blocked by #123`, `📝 draft`, `✅ merged`,
    `• unsubmitted`.
  - Symbols align with existing `log short` styling conventions
    (confirm in implementation).
- Long form: same symbol, rendered above the existing PR details block.

**No flags.** Always-on. Classification is free (data already loaded).
No `--no-readiness` unless someone complains.

**Edge cases.**

- Branch tracked but PR closed-without-merge → `Unsubmitted` (no live
  PR; user must resubmit).
- Cycle in branch graph → existing `BranchGraph` cycle guard applies.
- Cross-stack branches → classifier stays inside the current stack; does
  not reach into unrelated stacks.

**Tests.**

- Table-driven unit test for `Classify` covering every rule row plus
  ancestor-walk behavior (merged-parent passthrough, unsubmitted-parent
  passthrough, draft override, trunk termination).
- Script test `log_short_readiness.txt` builds a 3-deep stack in ShamHub
  with one merged, one draft, one open PR; asserts rendered symbols and
  `blocked by #N` suffix.

**Config / flags.** None.

**Changelog kind.** `Added`. Example body:
`log: Show readiness (ready/blocked/draft/merged/unsubmitted) per
branch`.

## Feature 3 — `gs repo sync --retarget` + `--dry-run`

**Where it lives.** `repo_sync.go` (flag wiring) and
`internal/handler/sync/` (planner + executor).

**Flags.**

- `--retarget` — enable retarget/push step. Default = value of
  `spice.sync.retarget` config. Override per-invocation with
  `--retarget=false`.
- `--dry-run` — print plan only, make no changes. Works with or without
  `--retarget` (with `--retarget` off, `--dry-run` prints "no actions
  planned").

**Config.** `spice.sync.retarget` (bool, default `false` in v1). Intent
is to flip default to `true` after bake-in across a couple of releases;
that flip is a separate future change, not part of this design.

**Execution order inside `sync` handler.**

1. **Fetch + detect** (existing behavior, unchanged).
2. **Plan retargets** (new). For each local branch whose direct parent
   is a merged-but-not-yet-deleted branch with a known PR:
   - New parent = nearest non-merged downstack ancestor. Trunk is a
     valid terminator.
   - Record `RetargetAction{branch, oldBase, newBase, prNumber}`.
3. **Plan pushes** (new). For each retargeted branch with a submitted
   PR, record `PushAction{branch}` (force-with-lease).
4. **Print plan.** Always printed. With `--dry-run`, stop here.
5. **Execute retargets** (bottom-up):
   - Update local metadata via existing store API.
   - `RestackHandler` on new base (already exists).
   - `EditChange(base=newBase)` on the forge (already exists across all
     forges).
   - Push via existing push handler.
6. **Delete merged branches** (existing behavior, unchanged — but now
   runs *after* retargets, not before).

**Safety constraints.**

- Dirty working tree → refuse run (reuse existing dirty-tree check).
  Flag `--force` not introduced in v1; user must stash.
- Rebase / merge / cherry-pick in progress → refuse run.
- Ambiguous branch→PR mapping (duplicate claims, or stored PR missing
  upstream) → refuse *that action* with `ambiguous: ...` naming the
  branch. Other independent retargets proceed.
- Non-fast-forward upstream change on a child branch → skip that child
  with a warning, continue others.
- Per-action refusal: run exits non-zero if any action was skipped.

**Plan output format.**

```
Plan:
  retarget feat/auth-model   #124   base: feat/base-api → main
  retarget feat/ui-cleanup   #125   base: feat/auth-model → feat/auth-model (unchanged)
  push     feat/auth-model   (force-with-lease)

2 actions planned, 1 skipped (already in sync).
```

With `--dry-run`, follow with `(dry-run: no changes made)`.

**Out of scope.**

- Does not create new PRs (use `gs stack submit`).
- Does not resolve merge conflicts.
- Does not handle multi-trunk, multi-stack, or cross-stack retargets.
- Does not auto-merge.

**Tests.**

- Unit: `planner_test.go` for `computeRetargetPlan(graph, mergedSet)`
  covering: merged parent → trunk, merged parent → non-trunk ancestor,
  cascaded merges (two generations merged), unsubmitted child (local
  retarget only, no push), ambiguous mapping (error),
  already-in-sync (no action).
- Unit: executor test asserting dirty-tree refusal.
- Script: `repo_sync_retarget_basic.txt` — 3-deep stack in ShamHub,
  merge bottom PR, run `--retarget --dry-run` then `--retarget`; assert
  plan, assert remote base updated, assert push happened.
- Script: `repo_sync_retarget_dirty.txt` — dirty working tree → refusal,
  non-zero exit.
- Script: `repo_sync_retarget_cascaded.txt` — merge two consecutive
  PRs; top PR retargets to trunk directly.

**Changelog kind.** `Added`. Example body:
`repo sync: Add --retarget to restack and re-push child PRs after a
parent merges`.

## Cross-cutting

**Forge-layer changes.** None. No new methods on `forge.Repository`;
all three features compose existing primitives.

**Error conventions.** Lowercase, action-oriented, `%w` wrapping. New
error sentinels only where needed (`errAmbiguousMapping` in F3).

**Testing strategy.**

- Unit tests for all pure logic (renderer, classifier, planner).
- Script tests in `testdata/script/` for end-to-end through ShamHub.
- No new integration tests against live GitHub/GitLab/Bitbucket
  required; existing integration harness exercises the forge methods
  we reuse.

**Rollout / PR order.** Three independent PRs, landable in any order:

| PR | Feature | Risk | Default behavior change? |
|----|---|---|---|
| 1  | nav-comment `Depends on`/`Next` | low | nav-comment content changes when enabled |
| 2  | readiness in `log short/long`   | low | `log` output adds a symbol column |
| 3  | `repo sync --retarget`          | medium | none (opt-in flag) |

Each PR ships a `changie` entry. PR 3 adds doc for
`spice.sync.retarget` in `doc/src/`.

## Success criteria (v1 cut line)

Ship only if all three are true:

- PR 1: nav-comment reliably includes accurate `Depends on` / `Next`
  lines across the test matrix, with no regressions to existing
  nav-comment behavior.
- PR 2: `log short` / `log long` readiness classification passes the
  unit test matrix and at least one multi-state script test.
- PR 3: `--dry-run` prints the expected plan for basic + cascaded
  cases; `--retarget` executes it end-to-end in ShamHub; dirty-tree and
  ambiguous-mapping refusals work; no regression to existing
  `repo sync` behavior when `--retarget` is off.

## Explicit things NOT in this design

- Managed PR-body block (superseded by existing nav-comment).
- New `gs status` or `gs stack status` command.
- New `gs pr` subcommand group.
- New `gs stack sync` command.
- Live CI / mergeable / review-state queries.
- New forge interface methods.
- Retarget-on by default in v1.

## Open questions for the upstream maintainers

Per the source spec, these should be raised in the PR descriptions, not
resolved unilaterally:

1. Does the nav-comment format need to remain strictly backward-compatible
   with any external parsers? (If so, `Depends on` / `Next` lines should
   sit inside a marked sub-block.)
2. Is `spice.sync.retarget` the right config key name, or should it
   match an existing naming convention we haven't found?
3. Is there an existing dirty-tree check helper the retarget executor
   should reuse, or should it introduce its own?
