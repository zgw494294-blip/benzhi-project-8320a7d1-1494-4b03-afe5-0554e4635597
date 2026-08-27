package analysis

import (
	"corepreservation/internal/domain"
	"time"
)

func ExecutionDigest(e domain.CutExecution) string {
	return domain.Digest(struct {
		ExecutionID, CaseID, AuthorizationDigest, RequestDigest string
		ActualSegments                                          []domain.Segment
		MassBeforeMg, SampleMassMg, MassAfterMg                 int
		ContainerCode, Operator, Witness, IdempotencyKey        string
		ExecutedAt                                              time.Time
	}{e.ExecutionID, e.CaseID, e.AuthorizationDigest, e.RequestDigest, domain.SortSegments(e.ActualSegments), e.MassBeforeMg, e.SampleMassMg, e.MassAfterMg, e.ContainerCode, e.Operator, e.Witness, e.IdempotencyKey, e.ExecutedAt})
}
