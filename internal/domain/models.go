package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

type Interval struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

func (i Interval) Valid() bool             { return i.Start >= 0 && i.End > i.Start }
func (i Interval) Overlap(o Interval) bool { return i.Start < o.End && o.Start < i.End }

type Segment struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

func (s Segment) Valid() bool { return s.Start >= 0 && s.End > s.Start }

type Change struct {
	Field  string `json:"field"`
	Before any    `json:"before"`
	After  any    `json:"after"`
}
type CoreRecord struct {
	CoreID               string     `json:"coreId"`
	CatalogCode          string     `json:"catalogCode"`
	BoxID                string     `json:"boxId"`
	DepthStartMm         int        `json:"depthStartMm"`
	DepthEndMm           int        `json:"depthEndMm"`
	InitialMassMg        int        `json:"initialMassMg"`
	AvailableMassMg      int        `json:"availableMassMg"`
	MinimumReserveMassMg int        `json:"minimumReserveMassMg"`
	ProtectedIntervals   []Interval `json:"protectedIntervals"`
	Revision             int        `json:"revision"`
}
type CoreVersion struct {
	Core          CoreRecord `json:"core"`
	ChangeSummary []Change   `json:"changeSummary"`
	RevisionNote  string     `json:"revisionNote,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
}
type CoreCaseImpact struct {
	CaseID           string    `json:"caseId"`
	Status           string    `json:"status"`
	Revision         int       `json:"revision"`
	ConflictSegments []Segment `json:"conflictSegments"`
	MassShortfallMg  int       `json:"massShortfallMg"`
}
type CoreRevisionImpact struct {
	CoreID           string           `json:"coreId"`
	ExpectedRevision int              `json:"expectedRevision"`
	CurrentRevision  int              `json:"currentRevision"`
	Proposed         CoreRecord       `json:"proposed"`
	ChangeSummary    []Change         `json:"changeSummary"`
	AffectedCases    []CoreCaseImpact `json:"affectedCases"`
	BlockingReasons  []Violation      `json:"blockingReasons"`
}

type SamplingCase struct {
	CaseID              string                `json:"caseId"`
	CoreID              string                `json:"coreId"`
	Purpose             string                `json:"purpose"`
	Method              string                `json:"method"`
	RequestedSegments   []Segment             `json:"requestedSegments"`
	EstimatedMassMg     int                   `json:"estimatedMassMg"`
	Status              string                `json:"status"`
	Revision            int                   `json:"revision"`
	PrecheckDigest      string                `json:"precheckDigest"`
	AuthorizationDigest string                `json:"authorizationDigest"`
	CreatedAt           time.Time             `json:"createdAt"`
	UpdatedAt           time.Time             `json:"updatedAt"`
	Findings            []ReviewFinding       `json:"findings"`
	Execution           *CutExecution         `json:"execution,omitempty"`
	Credential          *ProvenanceCredential `json:"credential,omitempty"`
}
type CaseVersion struct {
	Case          SamplingCase `json:"case"`
	ChangeSummary []Change     `json:"changeSummary"`
	RevisionNote  string       `json:"revisionNote,omitempty"`
	Actor         string       `json:"actor,omitempty"`
	CreatedAt     time.Time    `json:"createdAt"`
}
type ReviewFinding struct {
	FindingID      string    `json:"findingId"`
	CaseID         string    `json:"caseId"`
	Code           string    `json:"code"`
	Message        string    `json:"message"`
	SegmentRef     *Segment  `json:"segmentRef,omitempty"`
	Status         string    `json:"status"`
	OpenedRevision int       `json:"openedRevision"`
	ClosedRevision int       `json:"closedRevision,omitempty"`
	ClosureNote    string    `json:"closureNote,omitempty"`
	OpenedAt       time.Time `json:"openedAt"`
	ClosedAt       time.Time `json:"closedAt,omitempty"`
}
type FindingEvent struct {
	EventID      string    `json:"eventId"`
	FindingID    string    `json:"findingId"`
	CaseID       string    `json:"caseId"`
	Type         string    `json:"type"`
	CaseRevision int       `json:"caseRevision"`
	Code         string    `json:"code,omitempty"`
	Message      string    `json:"message,omitempty"`
	SegmentRef   *Segment  `json:"segmentRef,omitempty"`
	ClosureNote  string    `json:"closureNote,omitempty"`
	OccurredAt   time.Time `json:"occurredAt"`
}
type ActiveCaseSummary struct {
	CaseID          string    `json:"caseId"`
	Status          string    `json:"status"`
	Revision        int       `json:"revision"`
	Segments        []Segment `json:"segments"`
	EstimatedMassMg int       `json:"estimatedMassMg"`
}
type PrecheckFinding struct {
	Code          string   `json:"code"`
	Message       string   `json:"message"`
	Segment       *Segment `json:"segment,omitempty"`
	RelatedCaseID string   `json:"relatedCaseId,omitempty"`
}
type PrecheckSnapshot struct {
	Digest            string              `json:"digest"`
	InputFingerprint  string              `json:"inputFingerprint"`
	CaseID            string              `json:"caseId"`
	CaseRevision      int                 `json:"caseRevision"`
	CoreID            string              `json:"coreId"`
	CoreRevision      int                 `json:"coreRevision"`
	Core              CoreRecord          `json:"core"`
	RequestedSegments []Segment           `json:"requestedSegments"`
	EstimatedMassMg   int                 `json:"estimatedMassMg"`
	Purpose           string              `json:"purpose"`
	Method            string              `json:"method"`
	ActiveCases       []ActiveCaseSummary `json:"activeCases"`
	RuleVersion       string              `json:"ruleVersion"`
	Findings          []PrecheckFinding   `json:"findings"`
	RemainingMassMg   int                 `json:"remainingMassMg"`
	Pass              bool                `json:"pass"`
	CreatedAt         time.Time           `json:"createdAt"`
}
type SnapshotValidity struct {
	Valid         bool     `json:"valid"`
	ChangeSources []string `json:"changeSources"`
}
type AuthorizationManifest struct {
	CaseID              string    `json:"caseId"`
	CaseRevision        int       `json:"caseRevision"`
	PrecheckDigest      string    `json:"precheckDigest"`
	RequestedSegments   []Segment `json:"requestedSegments"`
	EstimatedMassMg     int       `json:"estimatedMassMg"`
	DepthToleranceMm    int       `json:"depthToleranceMm"`
	Method              string    `json:"method"`
	AuthorizedAt        time.Time `json:"authorizedAt"`
	AuthorizationDigest string    `json:"authorizationDigest"`
}
type AuthorizationView struct {
	Manifest       AuthorizationManifest `json:"manifest"`
	Valid          bool                  `json:"valid"`
	InvalidReasons []string              `json:"invalidReasons"`
}
type CutExecution struct {
	ExecutionID         string    `json:"executionId"`
	CaseID              string    `json:"caseId"`
	AuthorizationDigest string    `json:"authorizationDigest"`
	RequestDigest       string    `json:"requestDigest"`
	ActualSegments      []Segment `json:"actualSegments"`
	MassBeforeMg        int       `json:"massBeforeMg"`
	SampleMassMg        int       `json:"sampleMassMg"`
	MassAfterMg         int       `json:"massAfterMg"`
	ContainerCode       string    `json:"containerCode"`
	Operator            string    `json:"operator"`
	Witness             string    `json:"witness"`
	ExecutedAt          time.Time `json:"executedAt"`
	IdempotencyKey      string    `json:"idempotencyKey"`
}
type ExecutionReceipt struct {
	CaseID         string       `json:"caseId"`
	IdempotencyKey string       `json:"idempotencyKey"`
	RequestDigest  string       `json:"requestDigest"`
	ExecutionID    string       `json:"executionId"`
	Status         string       `json:"status"`
	Execution      CutExecution `json:"execution"`
	CreatedAt      time.Time    `json:"createdAt"`
	Replayed       bool         `json:"replayed"`
}
type VerificationAttempt struct {
	AttemptID          string      `json:"attemptId"`
	CaseID             string      `json:"caseId"`
	CaseRevision       int         `json:"caseRevision"`
	VerificationKey    string      `json:"verificationKey"`
	RequestDigest      string      `json:"requestDigest"`
	RemainingMassMg    int         `json:"remainingMassMg"`
	StorageLocation    string      `json:"storageLocation"`
	Verifier           string      `json:"verifier"`
	WitnessNote        string      `json:"witnessNote"`
	Verdict            string      `json:"verdict"`
	Reasons            []Violation `json:"reasons"`
	VerificationDigest string      `json:"verificationDigest"`
	CredentialID       string      `json:"credentialId,omitempty"`
	CreatedAt          time.Time   `json:"createdAt"`
}
type ProvenanceCredential struct {
	CredentialID       string `json:"credentialId"`
	CaseID             string `json:"caseId"`
	CoreID             string `json:"coreId"`
	FrozenRevision     int    `json:"frozenRevision"`
	SegmentDigest      string `json:"segmentDigest"`
	ExecutionDigest    string `json:"executionDigest"`
	VerificationDigest string `json:"verificationDigest"`
	RemainingMassMg    int    `json:"remainingMassMg"`
	StorageLocation    string `json:"storageLocation"`
	IssuedAt           string `json:"issuedAt"`
	Digest             string `json:"digest"`
}
type CredentialVerification struct {
	Valid      bool                 `json:"valid"`
	Mismatches []string             `json:"mismatches"`
	Credential ProvenanceCredential `json:"credential"`
}
type OccupancyReason struct {
	Type         string `json:"type"`
	CaseID       string `json:"caseId,omitempty"`
	Status       string `json:"status,omitempty"`
	Revision     int    `json:"revision,omitempty"`
	CredentialID string `json:"credentialId,omitempty"`
	Digest       string `json:"digest,omitempty"`
}
type AvailabilityPartition struct {
	Start   int               `json:"start"`
	End     int               `json:"end"`
	Kind    string            `json:"kind"`
	Reasons []OccupancyReason `json:"reasons"`
}
type AvailabilityView struct {
	CoreID                         string                  `json:"coreId"`
	CoreRevision                   int                     `json:"coreRevision"`
	Partitions                     []AvailabilityPartition `json:"partitions"`
	AvailableMassMg                int                     `json:"availableMassMg"`
	MinimumReserveMassMg           int                     `json:"minimumReserveMassMg"`
	ActiveEstimatedOccupancyMassMg int                     `json:"activeEstimatedOccupancyMassMg"`
	RequestableMassBudgetMg        int                     `json:"requestableMassBudgetMg"`
}
type Violation struct {
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Segment *Segment `json:"segment,omitempty"`
}

const (
	Draft      = "Draft"
	Returned   = "Returned"
	Submitted  = "Submitted"
	Authorized = "Authorized"
	Executed   = "Executed"
	Frozen     = "Frozen"
)

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("revision conflict")
var ErrInvalid = errors.New("invalid request")
var ErrState = errors.New("invalid state")

type BusinessError struct {
	Kind    error       `json:"-"`
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

func (e *BusinessError) Error() string { return e.Code + ": " + e.Message }
func (e *BusinessError) Unwrap() error { return e.Kind }
func NewError(kind error, code, message string, details any) error {
	return &BusinessError{Kind: kind, Code: code, Message: message, Details: details}
}

func ValidateCore(c CoreRecord) error {
	if strings.TrimSpace(c.CoreID) == "" || c.DepthEndMm <= c.DepthStartMm || c.InitialMassMg < 0 || c.AvailableMassMg < 0 || c.MinimumReserveMassMg < 0 || c.AvailableMassMg > c.InitialMassMg {
		return fmt.Errorf("%w: core fields", ErrInvalid)
	}
	for _, p := range c.ProtectedIntervals {
		if !p.Valid() || p.Start < c.DepthStartMm || p.End > c.DepthEndMm {
			return fmt.Errorf("%w: protected interval", ErrInvalid)
		}
	}
	return nil
}
func ValidateSegments(s []Segment) error {
	if len(s) == 0 {
		return fmt.Errorf("%w: empty segments", ErrInvalid)
	}
	sorted := SortSegments(s)
	for i, x := range sorted {
		if !x.Valid() {
			return fmt.Errorf("%w: segment", ErrInvalid)
		}
		if i > 0 && sorted[i-1].End > x.Start {
			return NewError(ErrInvalid, "CASE_SEGMENT_OVERLAP", "案内目标区段不得重复或重叠", []Segment{sorted[i-1], x})
		}
	}
	return nil
}
func ValidateCaseBody(c SamplingCase) error {
	if strings.TrimSpace(c.Purpose) == "" || strings.TrimSpace(c.Method) == "" {
		return NewError(ErrInvalid, "CASE_TEXT_REQUIRED", "研究目的和操作方法不能为空", nil)
	}
	if c.EstimatedMassMg <= 0 {
		return NewError(ErrInvalid, "ESTIMATED_MASS_INVALID", "预计切割质量必须为正数", nil)
	}
	return ValidateSegments(c.RequestedSegments)
}
func Canonical(v any) []byte { b, _ := json.Marshal(v); return b }
func Digest(v any) string    { h := sha256.Sum256(Canonical(v)); return hex.EncodeToString(h[:]) }
func SortSegments(s []Segment) []Segment {
	out := append([]Segment(nil), s...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Start == out[j].Start {
			return out[i].End < out[j].End
		}
		return out[i].Start < out[j].Start
	})
	return out
}
func (c *SamplingCase) HasOpenFindings() bool {
	for _, f := range c.Findings {
		if f.Status != "Closed" {
			return true
		}
	}
	return false
}
func (c *SamplingCase) ValidateTransition(next string) error {
	allowed := map[string]map[string]bool{Draft: {Submitted: true, Returned: true}, Returned: {Submitted: true}, Submitted: {Authorized: true, Returned: true}, Authorized: {Executed: true}, Executed: {Frozen: true}}
	if !allowed[c.Status][next] {
		return fmt.Errorf("%w: %s -> %s", ErrState, c.Status, next)
	}
	if next == Authorized && c.HasOpenFindings() {
		return fmt.Errorf("%w: open findings", ErrState)
	}
	return nil
}
