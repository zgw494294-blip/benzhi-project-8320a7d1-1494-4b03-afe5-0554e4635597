package application

import (
	"corepreservation/internal/domain"
	"corepreservation/internal/store"
	"errors"
	"sync"
	"testing"
)

func newScaleService(t *testing.T, dir string) *Service {
	t.Helper()
	st, err := store.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := New(st)
	err = a.RegisterCore(domain.CoreRecord{CoreID: "core-scale", CatalogCode: "CAT", BoxID: "B1", DepthStartMm: 0, DepthEndMm: 200, InitialMassMg: 1000, AvailableMassMg: 1000, MinimumReserveMassMg: 200, ProtectedIntervals: []domain.Interval{{Start: 0, End: 10}}})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func createScaleCase(t *testing.T, a *Service, start, end, mass int) domain.SamplingCase {
	t.Helper()
	c, err := a.CreateCase(domain.SamplingCase{CoreID: "core-scale", Purpose: "同位素研究", Method: "低速锯切", RequestedSegments: []domain.Segment{{Start: start, End: end}}, EstimatedMassMg: mass}, 0)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCoreRevisionImpactAndHistory(t *testing.T) {
	a := newScaleService(t, "")
	c := createScaleCase(t, a, 30, 50, 100)
	cmd := CoreRevisionCommand{ExpectedRevision: 1, BoxID: "B2", MinimumReserveMassMg: 250, ProtectedIntervals: []domain.Interval{{Start: 40, End: 45}, {Start: 45, End: 48}}, RevisionNote: "调整箱位与保护段"}
	impact, err := a.PreviewCoreRevision("core-scale", cmd)
	if err != nil {
		t.Fatal(err)
	}
	if len(impact.AffectedCases) != 1 || impact.AffectedCases[0].CaseID != c.CaseID || len(impact.AffectedCases[0].ConflictSegments) != 1 {
		t.Fatalf("unexpected impact: %+v", impact)
	}
	v, err := a.ReviseCore("core-scale", cmd)
	if err != nil {
		t.Fatal(err)
	}
	if v.Core.Revision != 2 || len(v.Core.ProtectedIntervals) != 1 {
		t.Fatalf("unexpected version: %+v", v)
	}
	_, err = a.ReviseCore("core-scale", cmd)
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	history, err := a.CoreVersions("core-scale")
	if err != nil || len(history) != 2 || history[0].Core.BoxID != "B1" || history[1].Core.BoxID != "B2" {
		t.Fatalf("unexpected history: %+v %v", history, err)
	}
}

func TestCaseRevisionFindingsAndStalePrecheck(t *testing.T) {
	a := newScaleService(t, "")
	first := createScaleCase(t, a, 30, 40, 80)
	second := createScaleCase(t, a, 60, 70, 70)
	p, err := a.Precheck(first.CaseID)
	if err != nil || !p.Pass {
		t.Fatalf("precheck: %+v %v", p, err)
	}
	repeated, err := a.Precheck(first.CaseID)
	if err != nil || repeated.Digest != p.Digest || !repeated.CreatedAt.Equal(p.CreatedAt) {
		t.Fatalf("precheck must be deterministic and immutable: %+v %v", repeated, err)
	}
	_, err = a.ReviseCaseCommand(second.CaseID, CaseRevisionCommand{ExpectedRevision: second.Revision, Purpose: second.Purpose, Method: second.Method, RequestedSegments: []domain.Segment{{Start: 80, End: 90}}, EstimatedMassMg: 70, RevisionNote: "调整目标段"})
	if err != nil {
		t.Fatal(err)
	}
	_, validity, err := a.GetPrecheck(first.CaseID, p.Digest)
	if err != nil || validity.Valid {
		t.Fatalf("snapshot should be stale: %+v %v", validity, err)
	}
	if _, err = a.SubmitCase(first.CaseID); !errors.Is(err, domain.ErrState) {
		t.Fatalf("stale submit should fail: %v", err)
	}
	returned, err := a.ReturnCaseIssues(first.CaseID, []ReviewIssue{{Code: "SEGMENT_NOTE", Message: "说明区段用途", SegmentRef: &domain.Segment{Start: 30, End: 40}}, {Code: "METHOD_NOTE", Message: "补充方法参数"}})
	if err != nil {
		t.Fatal(err)
	}
	v, err := a.ReviseCaseCommand(first.CaseID, CaseRevisionCommand{ExpectedRevision: returned.Revision, Purpose: first.Purpose, Method: "低速锯切并记录参数", RequestedSegments: []domain.Segment{{Start: 31, End: 40}}, EstimatedMassMg: 60, RevisionNote: "逐项完成整改"})
	if err != nil {
		t.Fatal(err)
	}
	if v.Case.PrecheckDigest != "" || len(v.ChangeSummary) < 3 {
		t.Fatalf("revision not complete: %+v", v)
	}
	for _, f := range v.Case.Findings {
		if _, err = a.CloseFindingWithRevision(first.CaseID, f.FindingID, "已按修订内容整改", v.Case.Revision); err != nil {
			t.Fatal(err)
		}
	}
	p, err = a.Precheck(first.CaseID)
	if err != nil || !p.Pass {
		t.Fatalf("recheck: %+v %v", p, err)
	}
	if _, err = a.SubmitCase(first.CaseID); err != nil {
		t.Fatal(err)
	}
	history, err := a.CaseVersions(first.CaseID)
	if err != nil || len(history) != 3 {
		t.Fatalf("unexpected history: %d %v", len(history), err)
	}
}

func TestExecutionIdempotencyAndVerificationLedger(t *testing.T) {
	dir := t.TempDir()
	a := newScaleService(t, dir)
	c := createScaleCase(t, a, 30, 40, 100)
	p, err := a.Precheck(c.CaseID)
	if err != nil || !p.Pass {
		t.Fatal(err)
	}
	if c, err = a.SubmitCase(c.CaseID); err != nil {
		t.Fatal(err)
	}
	if c, err = a.Authorize(c.CaseID); err != nil {
		t.Fatal(err)
	}
	const n = 20
	ids := make(chan string, n)
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := a.ExecuteReceipt(c.CaseID, "same-key", c.AuthorizationDigest, []domain.Segment{{Start: 30, End: 40}}, 1000, 100, 900, "C1", "实验员", "见证人")
			if e != nil {
				errs <- e
				return
			}
			ids <- r.ExecutionID
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
	first := ""
	for id := range ids {
		if first == "" {
			first = id
		} else if id != first {
			t.Fatalf("different execution ids: %s %s", first, id)
		}
	}
	st2, err := store.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	a2 := New(st2)
	replay, err := a2.ExecuteReceipt(c.CaseID, "same-key", c.AuthorizationDigest, []domain.Segment{{Start: 30, End: 40}}, 1000, 100, 900, "C1", "实验员", "见证人")
	if err != nil || replay.ExecutionID != first || !replay.Replayed {
		t.Fatalf("restart replay: %+v %v", replay, err)
	}
	if _, err = a2.ExecuteReceipt(c.CaseID, "same-key", c.AuthorizationDigest, []domain.Segment{{Start: 30, End: 40}}, 1000, 99, 901, "C1", "实验员", "见证人"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected payload conflict: %v", err)
	}
	rejected, err := a2.VerifyAndFreeze(c.CaseID, FreezeCommand{RemainingMassMg: 150, StorageLocation: "A-1", Verifier: "保管员", WitnessNote: "现场核对", VerificationKey: "verify-bad"})
	if err != nil || rejected.Verdict != "Rejected" || len(rejected.Reasons) != 2 {
		t.Fatalf("rejected verification: %+v %v", rejected, err)
	}
	current, _ := a2.GetCase(c.CaseID)
	if current.Status != domain.Executed || current.Credential != nil {
		t.Fatalf("rejection mutated case: %+v", current)
	}
	passed, err := a2.VerifyAndFreeze(c.CaseID, FreezeCommand{RemainingMassMg: 900, StorageLocation: "A-1", Verifier: "保管员", WitnessNote: "现场核对", VerificationKey: "verify-ok"})
	if err != nil || passed.Verdict != "Passed" || passed.CredentialID == "" {
		t.Fatalf("passed verification: %+v %v", passed, err)
	}
	check, err := a2.VerifyCredential(passed.CredentialID)
	if err != nil || !check.Valid {
		t.Fatalf("credential: %+v %v", check, err)
	}
	tampered := check.Credential
	tampered.SegmentDigest = "tampered"
	if err = st2.SaveCredential(tampered); err != nil {
		t.Fatal(err)
	}
	check, err = a2.VerifyCredential(passed.CredentialID)
	if err != nil || check.Valid || len(check.Mismatches) != 2 || check.Mismatches[0] != "digest" || check.Mismatches[1] != "segmentDigest" {
		t.Fatalf("tampered credential: %+v %v", check, err)
	}
	view, err := a2.Available("core-scale")
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Partitions) == 0 || view.Partitions[0].Start != 0 || view.Partitions[len(view.Partitions)-1].End != 200 {
		t.Fatalf("availability coverage: %+v", view)
	}
	_, err = a2.ReviseCore("core-scale", CoreRevisionCommand{ExpectedRevision: view.CoreRevision, BoxID: "B1", MinimumReserveMassMg: 200, ProtectedIntervals: []domain.Interval{{Start: 30, End: 40}}, RevisionNote: "不得覆盖冻结区段"})
	if !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("frozen cut must block core revision: %v", err)
	}
	core, _ := a2.GetCore("core-scale")
	if core.Revision != view.CoreRevision {
		t.Fatalf("blocked revision changed core: %+v", core)
	}
}
