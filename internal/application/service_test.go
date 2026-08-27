package application

import (
	"corepreservation/internal/domain"
	"corepreservation/internal/store"
	"testing"
)

func TestWorkflowIdempotency(t *testing.T) {
	st, _ := store.New("")
	a := New(st)
	_ = a.RegisterCore(domain.CoreRecord{CoreID: "c", DepthStartMm: 0, DepthEndMm: 100, InitialMassMg: 1000, AvailableMassMg: 1000, MinimumReserveMassMg: 100})
	c, e := a.CreateCase(domain.SamplingCase{CoreID: "c", Purpose: "研究", Method: "锯切", RequestedSegments: []domain.Segment{{Start: 20, End: 30}}, EstimatedMassMg: 100}, 0)
	if e != nil {
		t.Fatal(e)
	}
	r, e := a.Precheck(c.CaseID)
	if e != nil || !r.Pass {
		t.Fatal(e, r)
	}
	_, _ = a.SubmitCase(c.CaseID)
	c, _ = a.Authorize(c.CaseID)
	x, e := a.Execute(c.CaseID, "k", c.AuthorizationDigest, []domain.Segment{{Start: 20, End: 30}}, 1000, 100, 900, "box", "op", "wit")
	if e != nil {
		t.Fatal(e)
	}
	y, e := a.Execute(c.CaseID, "k", c.AuthorizationDigest, x.ActualSegments, 1000, 100, 900, "box", "op", "wit")
	if e != nil || x.ExecutionID != y.ExecutionID {
		t.Fatal("idempotency")
	}
}
