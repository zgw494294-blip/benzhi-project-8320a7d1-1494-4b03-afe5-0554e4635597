package cache_stale_availability

import (
	"corepreservation/internal/application"
	"corepreservation/internal/domain"
	"corepreservation/internal/store"
	"testing"
)

func TestAvailabilityRefreshesAfterCaseCreation(t *testing.T) {
	st, err := store.New("")
	if err != nil {
		t.Fatal(err)
	}
	app := application.New(st)
	core := domain.CoreRecord{
		CoreID:               "cache-core",
		DepthStartMm:         0,
		DepthEndMm:           100,
		InitialMassMg:        1000,
		AvailableMassMg:      1000,
		MinimumReserveMassMg: 100,
	}
	if err := app.RegisterCore(core); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Available(core.CoreID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.CreateCase(domain.SamplingCase{
		CaseID:            "cache-case",
		CoreID:            core.CoreID,
		Purpose:           "区段研究",
		Method:            "低速切割",
		RequestedSegments: []domain.Segment{{Start: 20, End: 30}},
		EstimatedMassMg:   100,
	}, 0); err != nil {
		t.Fatal(err)
	}
	view, err := app.Available(core.CoreID)
	if err != nil {
		t.Fatal(err)
	}
	if view.ActiveEstimatedOccupancyMassMg != 100 {
		t.Fatalf("availability did not refresh after case creation: got active mass %d", view.ActiveEstimatedOccupancyMassMg)
	}
}
