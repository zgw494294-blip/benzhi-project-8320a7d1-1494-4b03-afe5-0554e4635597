package credentialverificationscratchrace

import (
	"corepreservation/internal/analysis"
	"corepreservation/internal/application"
	"corepreservation/internal/domain"
	"corepreservation/internal/store"
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"testing"
)

func saveTamperedCredential(t *testing.T, st *store.Store, suffix, tamper string) string {
	t.Helper()
	caseID := "case-" + suffix
	execution := domain.CutExecution{
		ExecutionID:         "execution-" + suffix,
		CaseID:              caseID,
		AuthorizationDigest: "authorization-" + suffix,
		RequestDigest:       "request-" + suffix,
		ActualSegments:      []domain.Segment{{Start: 20, End: 30}},
		MassBeforeMg:        1000,
		SampleMassMg:        100,
		MassAfterMg:         900,
		ContainerCode:       "container-" + suffix,
		Operator:            "实验员",
		Witness:             "见证人",
		IdempotencyKey:      "execute-" + suffix,
	}
	credential := domain.ProvenanceCredential{
		CredentialID:       "credential-" + suffix,
		CaseID:             caseID,
		CoreID:             "core-" + suffix,
		FrozenRevision:     2,
		SegmentDigest:      domain.SegmentDigest(execution.ActualSegments),
		ExecutionDigest:    analysis.ExecutionDigest(execution),
		VerificationDigest: "verification-" + suffix,
		RemainingMassMg:    900,
		StorageLocation:    "A-1",
		IssuedAt:           "2026-08-27T00:00:00Z",
	}
	credential.Digest = analysis.CredentialDigest(credential)
	c := domain.SamplingCase{
		CaseID:     caseID,
		CoreID:     credential.CoreID,
		Status:     domain.Frozen,
		Revision:   credential.FrozenRevision,
		Execution:  &execution,
		Credential: &credential,
	}
	receipt := domain.ExecutionReceipt{
		CaseID:         caseID,
		IdempotencyKey: execution.IdempotencyKey,
		RequestDigest:  execution.RequestDigest,
		ExecutionID:    execution.ExecutionID,
		Status:         domain.Executed,
		Execution:      execution,
	}
	if err := st.CommitExecution(c, domain.CaseVersion{Case: c}, execution, receipt); err != nil {
		t.Fatal(err)
	}
	tampered := credential
	switch tamper {
	case "segment":
		tampered.SegmentDigest = "tampered-segment"
	case "execution":
		tampered.ExecutionDigest = "tampered-execution"
	default:
		t.Fatalf("unknown tamper mode %q", tamper)
	}
	if err := st.SaveCredential(tampered); err != nil {
		t.Fatal(err)
	}
	return credential.CredentialID
}

func TestConcurrentCredentialVerificationIsIsolated(t *testing.T) {
	previousProcs := runtime.GOMAXPROCS(2)
	defer runtime.GOMAXPROCS(previousProcs)

	st, err := store.New("")
	if err != nil {
		t.Fatal(err)
	}
	segmentCredential := saveTamperedCredential(t, st, "segment", "segment")
	executionCredential := saveTamperedCredential(t, st, "execution", "execution")
	service := application.New(st)

	start := make(chan struct{})
	errorsFound := make(chan error, 2)
	var ready sync.WaitGroup
	var done sync.WaitGroup
	for _, credentialID := range []string{segmentCredential, executionCredential} {
		credentialID := credentialID
		ready.Add(1)
		done.Add(1)
		go func() {
			defer done.Done()
			ready.Done()
			<-start
			for i := 0; i < 64; i++ {
				if _, verifyErr := service.VerifyCredential(credentialID); verifyErr != nil {
					errorsFound <- fmt.Errorf("verify %s: %w", credentialID, verifyErr)
					return
				}
			}
		}()
	}
	ready.Wait()
	close(start)
	done.Wait()
	close(errorsFound)
	for verifyErr := range errorsFound {
		t.Fatal(verifyErr)
	}

	first, err := service.VerifyCredential(segmentCredential)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.VerifyCredential(executionCredential)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(second.Mismatches, []string{"digest", "executionDigest"}) {
		t.Fatalf("unexpected second verification result: %v", second.Mismatches)
	}
	if !reflect.DeepEqual(first.Mismatches, []string{"digest", "segmentDigest"}) {
		t.Fatalf("first verification result was overwritten by a later request: %v", first.Mismatches)
	}
}
