package forgejo

// This file tests basic, end-to-end interactions with the Forgejo API
// using recorded VCR fixtures.
//
// In replay mode (default), tests replay from committed cassettes in
// testdata/fixtures/. When a cassette is absent, the test is skipped.
//
// To record cassettes against a live Forgejo instance (codeberg.org),
// set the environment variables and run with -update:
//
//	FORGEJO_TOKEN=<token> \
//	FORGEJO_TEST_OWNER=<your-codeberg-username> \
//	FORGEJO_TEST_REPO=git-spice-test-forgejo \
//	FORGEJO_TEST_REVIEWER=<reviewer-username> \
//	FORGEJO_TEST_ASSIGNEE=<assignee-username> \
//	go test ./internal/forge/forgejo/ -run TestIntegration -update

import (
	"crypto/rand"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.abhg.dev/gs/internal/fixturetest"
	"go.abhg.dev/gs/internal/forge"
	"go.abhg.dev/gs/internal/forge/forgetest"
	forgejogw "go.abhg.dev/gs/internal/gateway/forgejo"
	"go.abhg.dev/gs/internal/httptest"
	"go.abhg.dev/gs/internal/silog/silogtest"
	"gopkg.in/dnaeon/go-vcr.v4/pkg/recorder"
)

// _forgejoFixtures configures the fixturetest system for Forgejo integration tests.
var _forgejoFixtures = fixturetest.Config{Update: forgetest.Update}

// canonicalForgejoConfig returns canonical placeholders for Forgejo fixtures.
// Reviewer and Assignee share the same value to handle collisions
// when a single test account serves both roles.
func canonicalForgejoConfig() forgetest.ForgeConfig {
	return forgetest.ForgeConfig{
		Owner:    forgetest.CanonicalOwner,
		Repo:     forgetest.CanonicalRepo,
		Reviewer: "test-reviewer",
		Assignee: "test-reviewer", // same as reviewer to avoid collisions
	}
}

// forgejoConfig returns the Forgejo test configuration.
// In replay mode it returns canonical placeholders with no sanitizers.
// In update mode it reads from environment variables and builds sanitizers.
func forgejoConfig(
	t *testing.T,
) (cfg forgetest.ForgeConfig, sanitizers []httptest.Sanitizer) {
	t.Helper()

	canonical := canonicalForgejoConfig()

	if !forgetest.Update() {
		// Replay mode: canonical placeholders must match sanitized fixtures.
		return canonical, nil
	}

	// Update mode: real values from environment variables.
	owner := os.Getenv("FORGEJO_TEST_OWNER")
	if owner == "" {
		t.Fatal("FORGEJO_TEST_OWNER must be set in update mode")
	}
	repo := os.Getenv("FORGEJO_TEST_REPO")
	if repo == "" {
		repo = "git-spice-test-forgejo"
	}
	reviewer := os.Getenv("FORGEJO_TEST_REVIEWER")
	if reviewer == "" {
		reviewer = owner
	}
	assignee := os.Getenv("FORGEJO_TEST_ASSIGNEE")
	if assignee == "" {
		assignee = owner
	}

	cfg = forgetest.ForgeConfig{
		Owner:    owner,
		Repo:     repo,
		Reviewer: reviewer,
		Assignee: assignee,
	}

	sanitizers = forgetest.ConfigSanitizers(cfg, canonical)

	// Also sanitize the token itself so it cannot appear in cassettes.
	if token := os.Getenv("FORGEJO_TOKEN"); token != "" {
		sanitizers = append(sanitizers, httptest.Sanitizer{
			Replace: token,
			With:    "REDACTED",
		})
	}

	return cfg, sanitizers
}

// newForgejoRecorder creates a VCR recorder for the given test name.
// In replay mode, it skips the test if the cassette file is absent.
// In update mode, it records against the live Forgejo API.
func newForgejoRecorder(
	t *testing.T,
	name string,
	sanitizers []httptest.Sanitizer,
) *recorder.Recorder {
	t.Helper()

	t.Cleanup(func() {
		if t.Failed() && !forgetest.Update() {
			t.Logf("To update the test fixtures, run:")
			t.Logf(
				"    FORGEJO_TOKEN=<token> go test -update -run '^%s$'",
				t.Name(),
			)
		}
	})

	// In replay mode, skip gracefully when no cassette exists.
	if !forgetest.Update() {
		cassettePath := filepath.Join(
			"testdata", "fixtures", name+".yaml",
		)
		if _, err := os.Stat(cassettePath); errors.Is(err, os.ErrNotExist) {
			t.Skipf(
				"no cassette for %s; record with: "+
					"FORGEJO_TOKEN=<token> go test -update -run %q",
				name, t.Name(),
			)
		}
	}

	return forgetest.NewHTTPRecorder(t, name, sanitizers)
}

// openForgejoRepository opens a Forgejo Repository using the given
// HTTP client so requests go through the VCR recorder.
func openForgejoRepository(
	t *testing.T,
	httpClient *http.Client,
	owner, repo string,
) forge.Repository {
	t.Helper()

	token := forgetest.Token(t, DefaultURL, "FORGEJO_TOKEN")
	client, err := forgejogw.NewClient(
		forgejogw.StaticTokenSource(forgejogw.Token{
			Type:  forgejogw.TokenTypeAccessToken,
			Value: token,
		}),
		&forgejogw.ClientOptions{
			BaseURL:    DefaultURL,
			HTTPClient: httpClient,
		},
	)
	require.NoError(t, err)

	r, err := newRepository(
		t.Context(),
		&Forge{Log: silogtest.New(t)},
		owner, repo,
		silogtest.New(t),
		client,
		nil,
	)
	require.NoError(t, err)
	return r
}

// TestIntegration_Repository verifies that a Forgejo repository can be opened.
func TestIntegration_Repository(t *testing.T) {
	cfg, sanitizers := forgejoConfig(t)
	rec := newForgejoRecorder(t, t.Name(), sanitizers)
	openForgejoRepository(t, rec.GetDefaultClient(), cfg.Owner, cfg.Repo)
}

// TestIntegration runs the full forge integration suite against Forgejo
// using VCR-recorded cassettes.
func TestIntegration(t *testing.T) {
	cfg, sanitizers := forgejoConfig(t)
	remoteURL := DefaultURL + "/" + cfg.Owner + "/" + cfg.Repo

	// Skip this test if no cassettes have been recorded yet.
	// The suite uses both VCR cassettes and fixturetest value files;
	// without either, subtests will fail rather than skip.
	if !forgetest.Update() {
		cassetteDir := filepath.Join(
			"testdata", "fixtures", t.Name(),
		)
		if _, err := os.Stat(cassetteDir); errors.Is(err, os.ErrNotExist) {
			t.Skipf(
				"no cassettes for %s; record with: "+
					"FORGEJO_TOKEN=<token> "+
					"FORGEJO_TEST_OWNER=%s FORGEJO_TEST_REPO=%s "+
					"go test -update -run '^%s$'",
				t.Name(), cfg.Owner, cfg.Repo, t.Name(),
			)
		}
	}

	t.Cleanup(func() {
		if t.Failed() && !forgetest.Update() {
			t.Logf("To update the test fixtures, run:")
			t.Logf(
				"    FORGEJO_TOKEN=<token> "+
					"FORGEJO_TEST_OWNER=%s FORGEJO_TEST_REPO=%s "+
					"go test -update -run '^%s$'",
				cfg.Owner, cfg.Repo, t.Name(),
			)
		}
	})

	forgejoForge := Forge{Log: silogtest.New(t)}

	forgetest.RunIntegration(t, forgetest.IntegrationConfig{
		RemoteURL:  remoteURL,
		Forge:      &forgejoForge,
		Sanitizers: sanitizers,
		OpenRepository: func(
			t *testing.T, httpClient *http.Client,
		) forge.Repository {
			return openForgejoRepository(
				t, httpClient, cfg.Owner, cfg.Repo,
			)
		},
		MergeChange: func(
			t *testing.T,
			repo forge.Repository,
			id forge.ChangeID,
		) {
			require.NoError(t,
				repo.MergeChange(
					t.Context(),
					id,
					forge.MergeChangeOptions{},
				),
			)
		},
		// CloseChange is only needed when SkipMerge is false.
		// Since the Forgejo gateway does not expose a close-PR operation,
		// SkipMerge is set to true and this function is never called.
		CloseChange: func(
			t *testing.T,
			_ forge.Repository,
			_ forge.ChangeID,
		) {
			t.Fatal("CloseChange should not be called when SkipMerge is true")
		},
		Reviewers: []string{cfg.Reviewer},
		Assignees: []string{cfg.Assignee},
		// SetCommentsPageSize is unused when SkipCommentPagination is true.
		// _commentPageSize in this package is a const and cannot be stubbed.
		SetCommentsPageSize: nil,
		// Forgejo-specific limitations:
		// SkipMerge: the Forgejo gateway lacks a PullRequestClose API,
		// so TestChangeStates (which requires closing a PR) is skipped.
		SkipMerge: true,
		// SkipTemplates: Forgejo's ChangeTemplatePaths returns only
		// full file paths ending in ".md" — the integration suite
		// requires a bare directory path for creating test templates.
		SkipTemplates: true,
		// SkipCommentPagination: _commentPageSize is a package constant
		// and cannot be overridden from a test.
		SkipCommentPagination: true,
		// SkipCommentCounts: Forgejo's API does not expose
		// thread-resolution state; Resolved/Unresolved are always zero,
		// which would break the assertion total == resolved + unresolved.
		SkipCommentCounts: true,
	})
}

// TestIntegration_ChangeChecksState_noCICommit verifies that
// ChangeChecksState returns ChecksPassed for a PR whose head commit
// has no CI pipeline configured.
// Forgejo maps an empty combined status (total_count == 0) to ChecksPassed.
func TestIntegration_ChangeChecksState_noCICommit(t *testing.T) {
	cfg, sanitizers := forgejoConfig(t)
	rec := newForgejoRecorder(t, t.Name(), sanitizers)

	branchName := newForgejoRandomBranch(t)
	t.Logf("Creating branch: %s", branchName)

	if forgetest.Update() {
		cloneAndPushBranch(t, cfg, branchName)
	}

	repo := openForgejoRepository(
		t, rec.GetDefaultClient(), cfg.Owner, cfg.Repo,
	)

	change, err := repo.SubmitChange(t.Context(), forge.SubmitChangeRequest{
		Subject: "Test no-CI checks state " + branchName,
		Body:    "Verifies ChecksPassed is returned when no CI is configured.",
		Base:    "main",
		Head:    branchName,
	})
	require.NoError(t, err, "could not create PR")

	got, err := repo.ChangeChecksState(t.Context(), change.ID)
	require.NoError(t, err, "could not get checks state")
	assert.Equal(t, forge.ChecksPassed, got,
		"expected ChecksPassed when no CI is configured")
}

// TestIntegration_MergeChange_squash verifies that MergeChange with
// MergeMethodSquash squashes a PR into its base branch.
func TestIntegration_MergeChange_squash(t *testing.T) {
	cfg, sanitizers := forgejoConfig(t)
	rec := newForgejoRecorder(t, t.Name(), sanitizers)

	branchName := newForgejoRandomBranch(t)
	t.Logf("Creating branch: %s", branchName)

	if forgetest.Update() {
		cloneAndPushBranch(t, cfg, branchName)
	}

	repo := openForgejoRepository(
		t, rec.GetDefaultClient(), cfg.Owner, cfg.Repo,
	)

	change, err := repo.SubmitChange(t.Context(), forge.SubmitChangeRequest{
		Subject: "Test squash merge " + branchName,
		Body:    "Disposable PR for squash merge integration test.",
		Base:    "main",
		Head:    branchName,
	})
	require.NoError(t, err, "could not create PR")

	err = repo.MergeChange(t.Context(), change.ID, forge.MergeChangeOptions{
		Method: forge.MergeMethodSquash,
	})
	require.NoError(t, err, "could not squash-merge PR")

	// Verify the PR is now in the merged state.
	statuses, err := repo.ChangeStatuses(
		t.Context(), []forge.ChangeID{change.ID},
	)
	require.NoError(t, err, "could not get change statuses")
	require.Len(t, statuses, 1)
	assert.Equal(t, forge.ChangeMerged, statuses[0].State,
		"PR should be in merged state after squash merge")
}

// newForgejoRandomBranch generates a random branch name fixture
// for Forgejo integration tests.
func newForgejoRandomBranch(t *testing.T) string {
	t.Helper()
	fix := fixturetest.New(_forgejoFixtures, "branch", func() string {
		return forgejoRandomString(8)
	})
	return fix.Get(t)
}

// cloneAndPushBranch clones the test repository, creates a new branch,
// writes a file, commits, and pushes to the remote.
// It registers a cleanup to delete the remote branch.
// Only called in update mode.
func cloneAndPushBranch(
	t *testing.T,
	cfg forgetest.ForgeConfig,
	branchName string,
) {
	t.Helper()

	t.Setenv("GIT_AUTHOR_EMAIL", "bot@example.com")
	t.Setenv("GIT_AUTHOR_NAME", "gs-test[bot]")
	t.Setenv("GIT_COMMITTER_EMAIL", "bot@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "gs-test[bot]")

	output := t.Output()
	repoDir := t.TempDir()

	cloneURL := DefaultURL + "/" + cfg.Owner + "/" + cfg.Repo + ".git"
	cloneCmd := exec.Command("git", "clone", cloneURL, repoDir)
	cloneCmd.Stdout = output
	cloneCmd.Stderr = output
	require.NoError(t, cloneCmd.Run(), "failed to clone repository")

	branchCmd := exec.CommandContext(
		t.Context(),
		"git", "-C", repoDir, "checkout", "-b", branchName,
	)
	branchCmd.Stdout = output
	branchCmd.Stderr = output
	require.NoError(t, branchCmd.Run(), "failed to create branch")

	require.NoError(t,
		os.WriteFile(
			filepath.Join(repoDir, branchName+".txt"),
			[]byte(forgejoRandomString(32)),
			0o644,
		), "could not write file")

	addCmd := exec.CommandContext(
		t.Context(),
		"git", "-C", repoDir, "add", ".",
	)
	addCmd.Stdout = output
	addCmd.Stderr = output
	require.NoError(t, addCmd.Run(), "git add failed")

	commitCmd := exec.CommandContext(
		t.Context(),
		"git", "-C", repoDir, "commit", "-m", "commit from integration test",
	)
	commitCmd.Stdout = output
	commitCmd.Stderr = output
	require.NoError(t, commitCmd.Run(), "git commit failed")

	pushCmd := exec.CommandContext(
		t.Context(),
		"git", "-C", repoDir, "push", "origin", branchName,
	)
	pushCmd.Stdout = output
	pushCmd.Stderr = output
	require.NoError(t, pushCmd.Run(), "git push failed")

	t.Cleanup(func() {
		delCmd := exec.Command(
			"git", "-C", repoDir,
			"push", "origin", ":"+branchName,
		)
		delCmd.Stdout = output
		delCmd.Stderr = output
		assert.NoError(t, delCmd.Run(), "failed to delete remote branch")
	})
}

// forgejoRandomString generates a random alphanumeric string of length n.
func forgejoRandomString(n int) string {
	const alnum = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		var buf [1]byte
		_, _ = rand.Read(buf[:])
		idx := int(buf[0]) % len(alnum)
		b[i] = alnum[idx]
	}
	return string(b)
}
