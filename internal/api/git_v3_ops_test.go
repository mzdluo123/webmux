package api

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/contract"
)

func TestGitV3LineStageDiscardAndSmartCheckoutAPI(t *testing.T) {
	env := newGitAPITestEnv(t)
	repoURL := "/api/repositories/" + env.repo.ID
	gitURL := repoURL + "/git"
	wtURL := repoURL + "/worktrees/" + env.mainWT.ID + "/git"

	// 1. Auth (401) and Origin (403) checks on every V3 endpoint.
	for _, call := range [][2]string{
		{"POST", wtURL + "/discard-preview"},
		{"POST", wtURL + "/discard"},
		{"POST", wtURL + "/smart-checkout-preview"},
		{"POST", wtURL + "/smart-checkout"},
	} {
		r401 := env.doRaw(call[0], call[1], "{}", nil, "")
		r401.Body.Close()
		if r401.StatusCode != 401 {
			t.Fatalf("%s %s without token: got %d want 401", call[0], call[1], r401.StatusCode)
		}
		for _, badOrigin := range []string{"https://evil.example", "null"} {
			r403 := env.doRaw(call[0], call[1], "{}", map[string]string{
				"Authorization": "Bearer secret",
				"Origin":        badOrigin,
			}, "")
			r403.Body.Close()
			if r403.StatusCode != 403 {
				t.Fatalf("%s %s origin %q: got %d want 403", call[0], call[1], badOrigin, r403.StatusCode)
			}
		}
	}

	// 2. Sub-hunk line stage + unstage over HTTP.
	linesFile := filepath.Join(env.root, "lines.txt")
	if err := os.WriteFile(linesFile, []byte("top\no1\no2\no3\no4\no5\nbot\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env.gitOut(env.root, "add", "lines.txt")
	env.gitOut(env.root, "commit", "-m", "feat: add lines.txt")

	if err := os.WriteFile(linesFile, []byte("top\nn1\nn2\nn3\nn4\nn5\nbot\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var diff contract.GitStagingFileDiff
	if err := json.Unmarshal(env.doJSON("GET", wtURL+"/staging/diff?area=unstaged&path=lines.txt", nil, 200), &diff); err != nil {
		t.Fatal(err)
	}
	var stageLineIndices []int
	for idx, l := range diff.Hunks[0].Lines {
		if l.Text == "o2" || l.Text == "n2" || l.Text == "o4" || l.Text == "n4" {
			stageLineIndices = append(stageLineIndices, idx)
		}
	}
	env.doJSON("POST", wtURL+"/stage", contract.GitStageRequest{
		Identity: diff.Identity,
		LineSelections: []contract.GitLineSelection{
			{HunkID: diff.Hunks[0].ID, LineIndices: stageLineIndices},
		},
	}, 204)

	if got := env.gitOut(env.root, "cat-file", "-p", ":lines.txt"); got != "top\no1\nn2\no3\nn4\no5\nbot" {
		t.Fatalf("unexpected staged index after HTTP line stage:\n%s", got)
	}

	// 3. Discard Changes preview + 409 stale guard + execution + recovery OID in operation record.
	var stalePrev contract.GitDiscardPreview
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/discard-preview", contract.GitDiscardPreviewRequest{
		Scope: "files",
		Paths: []string{"lines.txt"},
	}, 200), &stalePrev); err != nil || stalePrev.ConfirmationID == "" {
		t.Fatalf("discard-preview failed: %+v (%v)", stalePrev, err)
	}
	// Simulate concurrent CLI agent write -> 409 reason="stale".
	if err := os.WriteFile(linesFile, []byte("top\nn1\nn2\nn3\nn4\nn5\nbot\nagent-line\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var staleConflict contract.GitConflict
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/discard", contract.GitDiscardRequest{
		ConfirmationID: stalePrev.ConfirmationID,
	}, 409), &staleConflict); err != nil || staleConflict.Reason != "stale" {
		t.Fatalf("expected 409 reason=stale, got %+v (%v)", staleConflict, err)
	}

	// Re-preview and execute discard -> worktree reverts to STAGED index ("top\no1\nn2\no3\nn4\no5\nbot\n"), staged untouched!
	var freshPrev contract.GitDiscardPreview
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/discard-preview", contract.GitDiscardPreviewRequest{
		Scope: "files",
		Paths: []string{"lines.txt"},
	}, 200), &freshPrev); err != nil {
		t.Fatal(err)
	}
	var discardRes contract.GitDiscardResult
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/discard", contract.GitDiscardRequest{
		ConfirmationID:  freshPrev.ConfirmationID,
		ClientRequestID: "api-discard-1",
	}, 200), &discardRes); err != nil || discardRes.RecoveryOID == "" {
		t.Fatalf("discard failed: %+v (%v)", discardRes, err)
	}
	var discardOp contract.GitOperation
	if err := json.Unmarshal(env.doJSON("GET", gitURL+"/operations/"+discardRes.OperationID, nil, 200), &discardOp); err != nil {
		t.Fatal(err)
	}
	if discardOp.Kind != "discard" || discardOp.RecoveryOID != discardRes.RecoveryOID || discardOp.RecoveryCommand == "" {
		t.Fatalf("discard GitOperation missing recovery metadata: %+v", discardOp)
	}

	// 4. Smart Checkout clean + conflict preserving stashCommitOid over HTTP!
	env.gitOut(env.root, "reset", "--hard", "HEAD")
	env.gitOut(env.root, "checkout", "-b", "feat/sc-conflict")
	if err := os.WriteFile(linesFile, []byte("conflicting-branch-content\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env.gitOut(env.root, "add", "lines.txt")
	env.gitOut(env.root, "commit", "-m", "conflict on feat/sc-conflict")
	env.gitOut(env.root, "checkout", "main")
	env.gitOut(env.root, "branch", "feat/sc-clean", "HEAD")

	if err := os.WriteFile(linesFile, []byte("dirty-carry-me\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.root, "carry-untracked.txt"), []byte("untracked-carry\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var scCleanPrev contract.GitSmartCheckoutPreview
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/smart-checkout-preview", contract.GitSmartCheckoutPreviewRequest{
		Target: contract.SwitchTarget{Kind: "local", Branch: "feat/sc-clean"},
	}, 200), &scCleanPrev); err != nil || scCleanPrev.ConfirmationID == "" {
		t.Fatalf("smart-checkout-preview failed: %+v (%v)", scCleanPrev, err)
	}
	var scCleanRes contract.GitSmartCheckoutResult
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/smart-checkout", contract.GitSmartCheckoutRequest{
		ConfirmationID:  scCleanPrev.ConfirmationID,
		ClientRequestID: "api-sc-clean-1",
	}, 200), &scCleanRes); err != nil || scCleanRes.Outcome != "completed" || !scCleanRes.StashDropped {
		t.Fatalf("clean smart-checkout failed: %+v (%v)", scCleanRes, err)
	}

	// Now Smart Checkout into conflicting `feat/sc-conflict` -> preserves stashOid + enters 3-way conflict view!
	var scConfPrev contract.GitSmartCheckoutPreview
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/smart-checkout-preview", contract.GitSmartCheckoutPreviewRequest{
		Target: contract.SwitchTarget{Kind: "local", Branch: "feat/sc-conflict"},
	}, 200), &scConfPrev); err != nil {
		t.Fatal(err)
	}
	var scConfRes contract.GitSmartCheckoutResult
	if err := json.Unmarshal(env.doJSON("POST", wtURL+"/smart-checkout", contract.GitSmartCheckoutRequest{
		ConfirmationID:  scConfPrev.ConfirmationID,
		ClientRequestID: "api-sc-conf-1",
	}, 200), &scConfRes); err != nil {
		t.Fatal(err)
	}
	if scConfRes.Outcome != "conflict_needs_resolution" || !scConfRes.StashPreserved || scConfRes.StashDropped || scConfRes.StashOID == "" {
		t.Fatalf("conflicting smart-checkout did not preserve stash: %+v", scConfRes)
	}
	if !strings.Contains(scConfRes.InProgress.SourceLabel, "Smart Checkout") {
		t.Fatalf("expected Smart Checkout source label in InProgress: %+v", scConfRes.InProgress)
	}
	var confDetail contract.GitConflictFileDetail
	if err := json.Unmarshal(env.doJSON("GET", wtURL+"/conflict?path="+url.QueryEscape("lines.txt"), nil, 200), &confDetail); err != nil || !confDetail.TextMergeable {
		t.Fatalf("3-way conflict endpoint failed after Smart Checkout conflict: %+v (%v)", confDetail, err)
	}
}
