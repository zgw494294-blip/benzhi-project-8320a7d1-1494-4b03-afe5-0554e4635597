package rejectedverificationrestart

import (
	"corepreservation/internal/application"
	"corepreservation/internal/domain"
	"corepreservation/internal/store"
	"testing"
)

func TestRejectedVerificationIdempotencySurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	st, err := store.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	app := application.New(st)
	core := domain.CoreRecord{
		CoreID:               "core-restart",
		CatalogCode:          "CAT-RESTART",
		BoxID:                "BOX-1",
		DepthStartMm:         0,
		DepthEndMm:           100,
		InitialMassMg:        1000,
		AvailableMassMg:      1000,
		MinimumReserveMassMg: 100,
	}
	if err := app.RegisterCore(core); err != nil {
		t.Fatal(err)
	}
	c, err := app.CreateCase(domain.SamplingCase{
		CaseID:            "case-restart",
		CoreID:            core.CoreID,
		Purpose:           "重启幂等性研究",
		Method:            "低速锯切",
		RequestedSegments: []domain.Segment{{Start: 20, End: 30}},
		EstimatedMassMg:   100,
	}, 1)
	if err != nil {
		t.Fatal(err)
	}
	precheck, err := app.Precheck(c.CaseID)
	if err != nil || !precheck.Pass {
		t.Fatalf("precheck: err=%v result=%+v", err, precheck)
	}
	if _, err = app.SubmitCase(c.CaseID); err != nil {
		t.Fatal(err)
	}
	c, err = app.Authorize(c.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = app.ExecuteReceipt(c.CaseID, "execute-once", c.AuthorizationDigest,
		[]domain.Segment{{Start: 20, End: 30}}, 1000, 100, 900,
		"SAMPLE-BOX", "operator", "witness"); err != nil {
		t.Fatal(err)
	}

	cmd := application.FreezeCommand{
		RemainingMassMg: 899,
		StorageLocation: "VAULT-A",
		Verifier:        "verifier",
		WitnessNote:     "质量计读数见证",
		VerificationKey: "rejected-once",
	}
	first, err := app.VerifyAndFreeze(c.CaseID, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if first.Verdict != "Rejected" {
		t.Fatalf("expected rejected verification, got %q", first.Verdict)
	}

	reopened, err := store.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	restarted := application.New(reopened)
	replayed, err := restarted.VerifyAndFreeze(c.CaseID, cmd)
	if err != nil {
		t.Fatal(err)
	}
	attempts, err := restarted.VerificationAttempts(c.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.AttemptID != first.AttemptID || len(attempts) != 1 {
		t.Fatalf("rejected verification retry was duplicated after restart: first=%s replayed=%s attempts=%d",
			first.AttemptID, replayed.AttemptID, len(attempts))
	}
}
