package analysis

import (
	"corepreservation/internal/domain"
	"testing"
)

func TestPrecheckProtectedAndReserve(t *testing.T) {
	c := domain.CoreRecord{CoreID: "c", DepthStartMm: 0, DepthEndMm: 100, AvailableMassMg: 500, MinimumReserveMassMg: 400, InitialMassMg: 500, ProtectedIntervals: []domain.Interval{{Start: 10, End: 20}}}
	x := domain.SamplingCase{CaseID: "x", RequestedSegments: []domain.Segment{{Start: 15, End: 25}}, EstimatedMassMg: 200, Method: "切割"}
	r := Precheck(c, x, nil)
	if r.Pass || len(r.Findings) < 2 {
		t.Fatalf("unexpected %+v", r)
	}
}
func TestMassAndTolerance(t *testing.T) {
	if !ValidateMass(100, 20, 80, 70) || ValidateMass(100, 20, 81, 70) {
		t.Fatal("mass")
	}
	if !ActualWithin([]domain.Segment{{Start: 10, End: 20}}, []domain.Segment{{Start: 9, End: 20}}, 1) {
		t.Fatal("tolerance")
	}
}
