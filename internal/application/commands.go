package application

import "corepreservation/internal/domain"

type CreateCoreCommand struct{ Core domain.CoreRecord }
type CreateCaseCommand struct {
	Case             domain.SamplingCase
	ExpectedRevision int
}
type ExecuteCommand struct {
	CaseID, IdempotencyKey, AuthorizationDigest string
	ActualSegments                              []domain.Segment
	MassBeforeMg, SampleMassMg, MassAfterMg     int
	ContainerCode, Operator, Witness            string
}

type CoreRevisionCommand struct {
	ExpectedRevision     int
	BoxID                string
	MinimumReserveMassMg int
	ProtectedIntervals   []domain.Interval
	RevisionNote         string
}
type CaseRevisionCommand struct {
	ExpectedRevision  int
	Purpose           string
	Method            string
	RequestedSegments []domain.Segment
	EstimatedMassMg   int
	RevisionNote      string
	Actor             string
}
type ReviewIssue struct {
	Code       string
	Message    string
	SegmentRef *domain.Segment
}
type FreezeCommand struct {
	RemainingMassMg int
	StorageLocation string
	Verifier        string
	WitnessNote     string
	VerificationKey string
}
