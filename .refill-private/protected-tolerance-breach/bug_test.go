package protectedtolerancebreach

import (
	"corepreservation/internal/application"
	"corepreservation/internal/domain"
	"corepreservation/internal/store"
	"testing"
)

func TestExecutionToleranceCannotEnterProtectedInterval(t *testing.T) {
	st, err := store.New("")
	if err != nil {
		t.Fatal(err)
	}
	app := application.New(st)
	if err = app.RegisterCore(domain.CoreRecord{CoreID: "core-protected", DepthStartMm: 0, DepthEndMm: 100, InitialMassMg: 1000, AvailableMassMg: 1000, MinimumReserveMassMg: 100, ProtectedIntervals: []domain.Interval{{Start: 30, End: 35}}}); err != nil {
		t.Fatal(err)
	}
	c, err := app.CreateCase(domain.SamplingCase{CoreID: "core-protected", Purpose: "研究", Method: "锯切", RequestedSegments: []domain.Segment{{Start: 20, End: 30}}, EstimatedMassMg: 100}, 0)
	if err != nil {
		t.Fatal(err)
	}
	p, err := app.Precheck(c.CaseID)
	if err != nil || !p.Pass {
		t.Fatalf("precheck: %v %+v", err, p)
	}
	if _, err = app.SubmitCase(c.CaseID); err != nil {
		t.Fatal(err)
	}
	c, err = app.Authorize(c.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	actual := []domain.Segment{{Start: 20, End: 32}}
	if _, err = app.Execute(c.CaseID, "protected-cut", c.AuthorizationDigest, actual, 1000, 100, 900, "box", "操作员", "见证人"); err == nil {
		t.Fatalf("execution entered protected interval through authorization tolerance")
	}
}
