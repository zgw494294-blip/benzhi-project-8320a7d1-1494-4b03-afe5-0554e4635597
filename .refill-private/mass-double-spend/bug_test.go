package massdoublespend

import (
	"corepreservation/internal/application"
	"corepreservation/internal/domain"
	"corepreservation/internal/store"
	"testing"
)

func TestAuthorizedCasesCannotReuseMassBaseline(t *testing.T) {
	st, err := store.New("")
	if err != nil {
		t.Fatal(err)
	}
	app := application.New(st)
	if err = app.RegisterCore(domain.CoreRecord{CoreID: "core-mass", DepthStartMm: 0, DepthEndMm: 100, InitialMassMg: 1000, AvailableMassMg: 1000, MinimumReserveMassMg: 100}); err != nil {
		t.Fatal(err)
	}
	create := func(seg domain.Segment, mass int) domain.SamplingCase {
		c, e := app.CreateCase(domain.SamplingCase{CoreID: "core-mass", Purpose: "研究", Method: "锯切", RequestedSegments: []domain.Segment{seg}, EstimatedMassMg: mass}, 0)
		if e != nil {
			t.Fatal(e)
		}
		return c
	}
	c1 := create(domain.Segment{Start: 10, End: 20}, 100)
	c2 := create(domain.Segment{Start: 30, End: 40}, 200)
	for _, id := range []string{c1.CaseID, c2.CaseID} {
		p, e := app.Precheck(id)
		if e != nil || !p.Pass {
			t.Fatalf("precheck %s: %v %+v", id, e, p)
		}
		if _, e = app.SubmitCase(id); e != nil {
			t.Fatal(e)
		}
		if _, e = app.Authorize(id); e != nil {
			t.Fatal(e)
		}
	}
	c1, err = app.GetCase(c1.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	c2, err = app.GetCase(c2.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errs := make(chan error, 2)
	go func() {
		<-start
		_, e := app.Execute(c1.CaseID, "exec-1", c1.AuthorizationDigest, c1.RequestedSegments, 1000, 100, 900, "box-1", "操作员", "见证人")
		errs <- e
	}()
	go func() {
		<-start
		_, e := app.Execute(c2.CaseID, "exec-2", c2.AuthorizationDigest, c2.RequestedSegments, 1000, 200, 800, "box-2", "操作员", "见证人")
		errs <- e
	}()
	close(start)
	for i := 0; i < 2; i++ {
		if err = <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if _, err = app.VerifyAndFreeze(c1.CaseID, application.FreezeCommand{RemainingMassMg: 900, StorageLocation: "A-1", Verifier: "核验员", WitnessNote: "见证", VerificationKey: "freeze-1"}); err != nil {
		t.Fatal(err)
	}
	if _, err = app.VerifyAndFreeze(c2.CaseID, application.FreezeCommand{RemainingMassMg: 800, StorageLocation: "A-2", Verifier: "核验员", WitnessNote: "见证", VerificationKey: "freeze-2"}); err != nil {
		t.Fatal(err)
	}
	core, err := app.GetCore("core-mass")
	if err != nil {
		t.Fatal(err)
	}
	if core.AvailableMassMg != 700 {
		t.Fatalf("two executions reused the same mass baseline; final available mass is %d, want 700", core.AvailableMassMg)
	}
}
