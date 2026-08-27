package application

import (
	"corepreservation/internal/analysis"
	"corepreservation/internal/domain"
	"fmt"
	"sort"
	"strings"
	"time"
)

func (s *Service) Authorize(id string) (domain.SamplingCase, error) {
	c, e := s.st.Case(id)
	if e != nil {
		return c, e
	}
	l := s.lock(c.CoreID)
	l.Lock()
	defer l.Unlock()
	c, e = s.st.Case(id)
	if e != nil {
		return c, e
	}
	if e = s.findingsClosedForCurrent(c); e != nil {
		return c, e
	}
	if c.Status != domain.Submitted {
		return c, domain.NewError(domain.ErrState, "AUTHORIZE_STATE_INVALID", "仅 Submitted 案卷可授权", nil)
	}
	if e = s.requireCurrentPrecheck(c); e != nil {
		return c, e
	}
	m := domain.AuthorizationManifest{CaseID: c.CaseID, CaseRevision: c.Revision, PrecheckDigest: c.PrecheckDigest, RequestedSegments: domain.SortSegments(c.RequestedSegments), EstimatedMassMg: c.EstimatedMassMg, DepthToleranceMm: 2, Method: c.Method, AuthorizedAt: time.Now().UTC()}
	m.AuthorizationDigest = analysis.AuthorizationDigest(m)
	c.AuthorizationDigest = m.AuthorizationDigest
	c.Status = domain.Authorized
	c.UpdatedAt = m.AuthorizedAt
	if e = s.st.SaveAuthorization(c, m); e != nil {
		return c, e
	}
	return c, nil
}
func (s *Service) Authorization(id string) (domain.AuthorizationView, error) {
	c, e := s.st.Case(id)
	if e != nil {
		return domain.AuthorizationView{}, e
	}
	m, e := s.st.Authorization(id)
	if e != nil {
		return domain.AuthorizationView{}, e
	}
	v := domain.AuthorizationView{Manifest: m, Valid: true, InvalidReasons: []string{}}
	if c.Revision != m.CaseRevision {
		v.Valid = false
		v.InvalidReasons = append(v.InvalidReasons, "caseRevision")
	}
	if c.AuthorizationDigest != m.AuthorizationDigest {
		v.Valid = false
		v.InvalidReasons = append(v.InvalidReasons, "authorizationDigest")
	}
	if c.Status != domain.Authorized {
		v.Valid = false
		v.InvalidReasons = append(v.InvalidReasons, "caseStatus")
	}
	return v, nil
}

func (s *Service) ExecuteReceipt(id, key, auth string, actual []domain.Segment, before, sample, after int, container, operator, witness string) (domain.ExecutionReceipt, error) {
	key = strings.TrimSpace(key)
	auth = strings.TrimSpace(auth)
	container = strings.TrimSpace(container)
	operator = strings.TrimSpace(operator)
	witness = strings.TrimSpace(witness)
	if key == "" {
		return domain.ExecutionReceipt{}, domain.NewError(domain.ErrInvalid, "IDEMPOTENCY_KEY_REQUIRED", "idempotencyKey 不能为空", nil)
	}
	actual = domain.SortSegments(actual)
	requestDigest := analysis.ExecutionRequestDigest(auth, actual, before, sample, after, container, operator, witness)
	c, e := s.st.Case(id)
	if e != nil {
		return domain.ExecutionReceipt{}, e
	}
	l := s.lock(c.CoreID)
	l.Lock()
	defer l.Unlock()
	if old, ok := s.st.ExecutionReceipt(id, key); ok {
		if old.RequestDigest != requestDigest {
			return domain.ExecutionReceipt{}, domain.NewError(domain.ErrConflict, "IDEMPOTENCY_PAYLOAD_CONFLICT", "相同幂等键的执行载荷不同", map[string]string{"firstRequestDigest": old.RequestDigest})
		}
		old.Replayed = true
		return old, nil
	}
	c, e = s.st.Case(id)
	if e != nil {
		return domain.ExecutionReceipt{}, e
	}
	if c.Status != domain.Authorized {
		return domain.ExecutionReceipt{}, domain.NewError(domain.ErrState, "EXECUTE_STATE_INVALID", "案卷当前不可执行", nil)
	}
	m, e := s.st.Authorization(id)
	if e != nil {
		return domain.ExecutionReceipt{}, e
	}
	if auth != m.AuthorizationDigest || auth != c.AuthorizationDigest {
		return domain.ExecutionReceipt{}, domain.NewError(domain.ErrInvalid, "AUTHORIZATION_DIGEST_MISMATCH", "授权摘要不匹配", nil)
	}
	if c.Revision != m.CaseRevision {
		return domain.ExecutionReceipt{}, domain.NewError(domain.ErrState, "AUTHORIZATION_INVALID", "授权清单引用的案卷版本已变化", nil)
	}
	if e = domain.ValidateSegments(actual); e != nil {
		return domain.ExecutionReceipt{}, e
	}
	core, e := s.st.Core(c.CoreID)
	if e != nil {
		return domain.ExecutionReceipt{}, e
	}
	violations := analysis.ExecutionViolations(m, core, actual, before, sample, after, container, operator, witness)
	if len(violations) > 0 {
		return domain.ExecutionReceipt{}, domain.NewError(domain.ErrInvalid, "EXECUTION_LIMIT_VIOLATION", "执行数据超出授权或质量约束", violations)
	}
	now := time.Now().UTC()
	x := domain.CutExecution{ExecutionID: fmt.Sprintf("exec-%d", now.UnixNano()), CaseID: id, AuthorizationDigest: auth, RequestDigest: requestDigest, ActualSegments: actual, MassBeforeMg: before, SampleMassMg: sample, MassAfterMg: after, ContainerCode: container, Operator: operator, Witness: witness, ExecutedAt: now, IdempotencyKey: key}
	c.Execution = &x
	c.Status = domain.Executed
	c.Revision++
	c.UpdatedAt = now
	r := domain.ExecutionReceipt{CaseID: id, IdempotencyKey: key, RequestDigest: requestDigest, ExecutionID: x.ExecutionID, Status: domain.Executed, Execution: x, CreatedAt: now}
	v := domain.CaseVersion{Case: c, ChangeSummary: []domain.Change{{Field: "status", Before: domain.Authorized, After: domain.Executed}}, RevisionNote: "记录切割执行", Actor: x.Operator, CreatedAt: now}
	if e = s.st.CommitExecution(c, v, x, r); e != nil {
		return domain.ExecutionReceipt{}, e
	}
	return r, nil
}
func (s *Service) Execute(id, key, auth string, actual []domain.Segment, before, sample, after int, container, operator, witness string) (domain.CutExecution, error) {
	r, e := s.ExecuteReceipt(id, key, auth, actual, before, sample, after, container, operator, witness)
	return r.Execution, e
}
func (s *Service) ExecutionReceipt(id, key string) (domain.ExecutionReceipt, error) {
	r, ok := s.st.ExecutionReceipt(id, key)
	if !ok {
		return r, domain.ErrNotFound
	}
	return r, nil
}

func verificationDigest(a domain.VerificationAttempt) string {
	return domain.Digest(struct {
		CaseID                                          string
		CaseRevision                                    int
		VerificationKey, RequestDigest                  string
		RemainingMassMg                                 int
		StorageLocation, Verifier, WitnessNote, Verdict string
		Reasons                                         []domain.Violation
		CreatedAt                                       time.Time
	}{a.CaseID, a.CaseRevision, a.VerificationKey, a.RequestDigest, a.RemainingMassMg, a.StorageLocation, a.Verifier, a.WitnessNote, a.Verdict, a.Reasons, a.CreatedAt})
}
func (s *Service) VerifyAndFreeze(id string, cmd FreezeCommand) (domain.VerificationAttempt, error) {
	cmd.VerificationKey = strings.TrimSpace(cmd.VerificationKey)
	cmd.StorageLocation = strings.TrimSpace(cmd.StorageLocation)
	cmd.Verifier = strings.TrimSpace(cmd.Verifier)
	cmd.WitnessNote = strings.TrimSpace(cmd.WitnessNote)
	if cmd.VerificationKey == "" {
		return domain.VerificationAttempt{}, domain.NewError(domain.ErrInvalid, "VERIFICATION_KEY_REQUIRED", "verificationKey 不能为空", nil)
	}
	requestDigest := analysis.VerificationRequestDigest(cmd.RemainingMassMg, strings.TrimSpace(cmd.StorageLocation), strings.TrimSpace(cmd.Verifier), strings.TrimSpace(cmd.WitnessNote))
	c, e := s.st.Case(id)
	if e != nil {
		return domain.VerificationAttempt{}, e
	}
	l := s.lock(c.CoreID)
	l.Lock()
	defer l.Unlock()
	if old, ok := s.st.VerificationByKey(id, cmd.VerificationKey); ok {
		if old.RequestDigest != requestDigest {
			return domain.VerificationAttempt{}, domain.NewError(domain.ErrConflict, "VERIFICATION_PAYLOAD_CONFLICT", "相同 verificationKey 的核验载荷不同", map[string]string{"firstRequestDigest": old.RequestDigest})
		}
		return old, nil
	}
	c, e = s.st.Case(id)
	if e != nil {
		return domain.VerificationAttempt{}, e
	}
	if c.Status != domain.Executed || c.Execution == nil {
		return domain.VerificationAttempt{}, domain.NewError(domain.ErrState, "FREEZE_STATE_INVALID", "仅 Executed 案卷可核验余样", nil)
	}
	core, e := s.st.Core(c.CoreID)
	if e != nil {
		return domain.VerificationAttempt{}, e
	}
	now := time.Now().UTC()
	reasons := analysis.VerificationReasons(*c.Execution, core, cmd.RemainingMassMg, strings.TrimSpace(cmd.StorageLocation), strings.TrimSpace(cmd.Verifier), strings.TrimSpace(cmd.WitnessNote))
	a := domain.VerificationAttempt{AttemptID: fmt.Sprintf("verify-%d", now.UnixNano()), CaseID: id, CaseRevision: c.Revision, VerificationKey: cmd.VerificationKey, RequestDigest: requestDigest, RemainingMassMg: cmd.RemainingMassMg, StorageLocation: strings.TrimSpace(cmd.StorageLocation), Verifier: strings.TrimSpace(cmd.Verifier), WitnessNote: strings.TrimSpace(cmd.WitnessNote), Verdict: "Rejected", Reasons: reasons, CreatedAt: now}
	if len(reasons) > 0 {
		a.VerificationDigest = verificationDigest(a)
		if e = s.st.SaveRejectedVerification(a); e != nil {
			return domain.VerificationAttempt{}, e
		}
		return a, nil
	}
	a.Verdict = "Passed"
	a.Reasons = []domain.Violation{}
	a.VerificationDigest = verificationDigest(a)
	segmentDigest := domain.SegmentDigest(c.Execution.ActualSegments)
	executionDigest := analysis.ExecutionDigest(*c.Execution)
	c.Status = domain.Frozen
	c.Revision++
	c.UpdatedAt = now
	cred := domain.ProvenanceCredential{CredentialID: fmt.Sprintf("cred-%d", now.UnixNano()), CaseID: id, CoreID: c.CoreID, FrozenRevision: c.Revision, SegmentDigest: segmentDigest, ExecutionDigest: executionDigest, VerificationDigest: a.VerificationDigest, RemainingMassMg: cmd.RemainingMassMg, StorageLocation: a.StorageLocation, IssuedAt: now.Format(time.RFC3339Nano)}
	cred.Digest = analysis.CredentialDigest(cred)
	a.CredentialID = cred.CredentialID
	c.Credential = &cred
	core.AvailableMassMg = cmd.RemainingMassMg
	core.Revision++
	cv := domain.CoreVersion{Core: core, ChangeSummary: []domain.Change{{Field: "availableMassMg", Before: c.Execution.MassBeforeMg, After: cmd.RemainingMassMg}}, RevisionNote: "余样核验通过并冻结", CreatedAt: now}
	v := domain.CaseVersion{Case: c, ChangeSummary: []domain.Change{{Field: "status", Before: domain.Executed, After: domain.Frozen}}, RevisionNote: "余样核验通过并签发凭据", Actor: a.Verifier, CreatedAt: now}
	if e = s.st.CommitFreeze(c, v, core, cv, a, cred); e != nil {
		return domain.VerificationAttempt{}, e
	}
	return a, nil
}
func (s *Service) Freeze(id string, remaining int, location string) (domain.ProvenanceCredential, error) {
	a, e := s.VerifyAndFreeze(id, FreezeCommand{RemainingMassMg: remaining, StorageLocation: location, Verifier: "兼容核验人", WitnessNote: "兼容见证说明", VerificationKey: fmt.Sprintf("legacy-%d", time.Now().UnixNano())})
	if e != nil {
		return domain.ProvenanceCredential{}, e
	}
	if a.Verdict != "Passed" {
		return domain.ProvenanceCredential{}, domain.NewError(domain.ErrInvalid, "VERIFICATION_REJECTED", "余样核验未通过", a.Reasons)
	}
	return s.st.Credential(a.CredentialID)
}
func (s *Service) VerificationAttempts(id string) ([]domain.VerificationAttempt, error) {
	if _, e := s.st.Case(id); e != nil {
		return nil, e
	}
	out := s.st.VerificationAttempts(id)
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].AttemptID < out[j].AttemptID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}
func (s *Service) VerificationAttempt(id, ref string) (domain.VerificationAttempt, error) {
	list, e := s.VerificationAttempts(id)
	if e != nil {
		return domain.VerificationAttempt{}, e
	}
	for _, a := range list {
		if a.AttemptID == ref || a.VerificationKey == ref {
			return a, nil
		}
	}
	return domain.VerificationAttempt{}, domain.ErrNotFound
}
func (s *Service) Available(coreID string) (domain.AvailabilityView, error) {
	core, e := s.st.Core(coreID)
	if e != nil {
		return domain.AvailabilityView{}, e
	}
	return analysis.Availability(core, s.st.Cases()), nil
}
func (s *Service) VerifyCredential(id string) (domain.CredentialVerification, error) {
	cred, e := s.st.Credential(id)
	if e != nil {
		return domain.CredentialVerification{}, e
	}
	result := domain.CredentialVerification{Valid: true, Mismatches: []string{}, Credential: cred}
	c, e := s.st.Case(cred.CaseID)
	if e != nil {
		result.Mismatches = append(result.Mismatches, "caseId")
	} else {
		if c.CoreID != cred.CoreID {
			result.Mismatches = append(result.Mismatches, "coreId")
		}
		if c.Revision != cred.FrozenRevision {
			result.Mismatches = append(result.Mismatches, "frozenRevision")
		}
		if c.Credential == nil || c.Credential.CredentialID != cred.CredentialID {
			result.Mismatches = append(result.Mismatches, "credentialId")
		} else if c.Credential.VerificationDigest != cred.VerificationDigest {
			result.Mismatches = append(result.Mismatches, "verificationDigest")
		}
		x, e2 := s.st.Execution(c.CaseID)
		if e2 != nil {
			result.Mismatches = append(result.Mismatches, "execution")
		} else {
			if domain.SegmentDigest(x.ActualSegments) != cred.SegmentDigest {
				result.Mismatches = append(result.Mismatches, "segmentDigest")
			}
			if analysis.ExecutionDigest(x) != cred.ExecutionDigest {
				result.Mismatches = append(result.Mismatches, "executionDigest")
			}
		}
	}
	if analysis.CredentialDigest(cred) != cred.Digest {
		result.Mismatches = append(result.Mismatches, "digest")
	}
	sort.Strings(result.Mismatches)
	result.Valid = len(result.Mismatches) == 0
	return result, nil
}
