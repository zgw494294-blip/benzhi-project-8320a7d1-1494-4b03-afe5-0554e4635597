package submit_history_gap

import (
	"corepreservation/internal/application"
	"corepreservation/internal/domain"
	"corepreservation/internal/store"
	"testing"
)

func TestSubmitPersistsVersionedStatusTransition(t *testing.T) {
	st, err := store.New("")
	if err != nil {
		t.Fatal(err)
	}
	app := application.New(st)
	if err := app.RegisterCore(domain.CoreRecord{CoreID: "history-core", DepthStartMm: 0, DepthEndMm: 100, InitialMassMg: 1000, AvailableMassMg: 1000, MinimumReserveMassMg: 100}); err != nil {
		t.Fatal(err)
	}
	c, err := app.CreateCase(domain.SamplingCase{CaseID: "history-case", CoreID: "history-core", Purpose: "研究", Method: "锯切", RequestedSegments: []domain.Segment{{Start: 20, End: 30}}, EstimatedMassMg: 100}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Precheck(c.CaseID); err != nil {
		t.Fatal(err)
	}
	submitted, err := app.SubmitCase(c.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	versions, err := app.CaseVersions(c.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range versions {
		if version.Case.CaseID == submitted.CaseID && version.Case.Status == domain.Submitted {
			return
		}
	}
	t.Fatalf("submitted transition missing from case history: revision=%d versions=%d", submitted.Revision, len(versions))
}
