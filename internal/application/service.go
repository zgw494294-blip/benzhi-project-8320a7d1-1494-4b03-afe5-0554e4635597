package application

import (
	"corepreservation/internal/analysis"
	"corepreservation/internal/domain"
	"corepreservation/internal/store"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
)

type Service struct {
	st             *store.Store
	locks          sync.Map
	availableMu    sync.RWMutex
	availableCache map[string]domain.AvailabilityView
}

func New(st *store.Store) *Service {
	return &Service{st: st, availableCache: make(map[string]domain.AvailabilityView)}
}
func (s *Service) lock(id string) *sync.Mutex {
	v, _ := s.locks.LoadOrStore(id, &sync.Mutex{})
	return v.(*sync.Mutex)
}
func (s *Service) RegisterCore(c domain.CoreRecord) error {
	if e := domain.ValidateCore(c); e != nil {
		return e
	}
	l := s.lock(c.CoreID)
	l.Lock()
	defer l.Unlock()
	if _, e := s.st.Core(c.CoreID); e == nil {
		return domain.NewError(domain.ErrConflict, "CORE_EXISTS", "岩芯编号已存在", nil)
	}
	c.ProtectedIntervals = domain.NormalizeIntervals(c.ProtectedIntervals)
	c.Revision = 1
	now := time.Now().UTC()
	return s.st.SaveCoreVersion(c, domain.CoreVersion{Core: c, ChangeSummary: []domain.Change{}, CreatedAt: now})
}
func (s *Service) GetCore(id string) (domain.CoreRecord, error)         { return s.st.Core(id) }
func (s *Service) ListCores() []domain.CoreRecord                       { return s.st.Cores() }
func (s *Service) CoreVersions(id string) ([]domain.CoreVersion, error) { return s.st.CoreVersions(id) }

func proposedCore(current domain.CoreRecord, cmd CoreRevisionCommand) (domain.CoreRecord, []domain.Change, error) {
	next := current
	next.BoxID = strings.TrimSpace(cmd.BoxID)
	next.MinimumReserveMassMg = cmd.MinimumReserveMassMg
	next.ProtectedIntervals = domain.NormalizeIntervals(cmd.ProtectedIntervals)
	if e := domain.ValidateCore(next); e != nil {
		return next, nil, e
	}
	var ch []domain.Change
	if current.BoxID != next.BoxID {
		ch = append(ch, domain.Change{Field: "boxId", Before: current.BoxID, After: next.BoxID})
	}
	if current.MinimumReserveMassMg != next.MinimumReserveMassMg {
		ch = append(ch, domain.Change{Field: "minimumReserveMassMg", Before: current.MinimumReserveMassMg, After: next.MinimumReserveMassMg})
	}
	if !reflect.DeepEqual(domain.NormalizeIntervals(current.ProtectedIntervals), next.ProtectedIntervals) {
		ch = append(ch, domain.Change{Field: "protectedIntervals", Before: domain.NormalizeIntervals(current.ProtectedIntervals), After: next.ProtectedIntervals})
	}
	return next, ch, nil
}
func (s *Service) coreImpact(id string, cmd CoreRevisionCommand) (domain.CoreRevisionImpact, error) {
	core, e := s.st.Core(id)
	if e != nil {
		return domain.CoreRevisionImpact{}, e
	}
	next, changes, e := proposedCore(core, cmd)
	if e != nil {
		return domain.CoreRevisionImpact{}, e
	}
	impact := domain.CoreRevisionImpact{CoreID: id, ExpectedRevision: cmd.ExpectedRevision, CurrentRevision: core.Revision, Proposed: next, ChangeSummary: changes, AffectedCases: []domain.CoreCaseImpact{}, BlockingReasons: []domain.Violation{}}
	if cmd.ExpectedRevision != core.Revision {
		impact.BlockingReasons = append(impact.BlockingReasons, domain.Violation{Code: "REVISION_CONFLICT", Message: "expectedRevision 与当前岩芯修订不一致"})
	}
	if next.MinimumReserveMassMg > core.AvailableMassMg {
		impact.BlockingReasons = append(impact.BlockingReasons, domain.Violation{Code: "CURRENT_RESERVE_LOW", Message: "当前余样低于新最低保留质量"})
	}
	for _, c := range s.st.Cases() {
		if c.CoreID != id {
			continue
		}
		ci := domain.CoreCaseImpact{CaseID: c.CaseID, Status: c.Status, Revision: c.Revision, ConflictSegments: []domain.Segment{}}
		segments := c.RequestedSegments
		if c.Status == domain.Frozen && c.Execution != nil {
			segments = c.Execution.ActualSegments
		}
		for _, seg := range segments {
			for _, p := range next.ProtectedIntervals {
				if p.Overlap(domain.Interval{Start: seg.Start, End: seg.End}) {
					ci.ConflictSegments = append(ci.ConflictSegments, seg)
					break
				}
			}
		}
		if !domain.IsTerminal(c.Status) && core.AvailableMassMg-c.EstimatedMassMg < next.MinimumReserveMassMg {
			ci.MassShortfallMg = next.MinimumReserveMassMg - (core.AvailableMassMg - c.EstimatedMassMg)
		}
		if len(ci.ConflictSegments) > 0 || ci.MassShortfallMg > 0 {
			ci.ConflictSegments = domain.SortSegments(ci.ConflictSegments)
			impact.AffectedCases = append(impact.AffectedCases, ci)
		}
		if c.Status == domain.Frozen && len(ci.ConflictSegments) > 0 {
			impact.BlockingReasons = append(impact.BlockingReasons, domain.Violation{Code: "FROZEN_CUT_OVERLAP", Message: "保护区覆盖已冻结实际切割区段", Segment: &ci.ConflictSegments[0]})
		}
	}
	sort.Slice(impact.AffectedCases, func(i, j int) bool { return impact.AffectedCases[i].CaseID < impact.AffectedCases[j].CaseID })
	sort.Slice(impact.BlockingReasons, func(i, j int) bool { return impact.BlockingReasons[i].Code < impact.BlockingReasons[j].Code })
	return impact, nil
}
func (s *Service) PreviewCoreRevision(id string, cmd CoreRevisionCommand) (domain.CoreRevisionImpact, error) {
	l := s.lock(id)
	l.Lock()
	defer l.Unlock()
	return s.coreImpact(id, cmd)
}
func (s *Service) ReviseCore(id string, cmd CoreRevisionCommand) (domain.CoreVersion, error) {
	v, _, e := s.ReviseCoreDetailed(id, cmd)
	return v, e
}
func (s *Service) ReviseCoreDetailed(id string, cmd CoreRevisionCommand) (domain.CoreVersion, domain.CoreRevisionImpact, error) {
	l := s.lock(id)
	l.Lock()
	defer l.Unlock()
	impact, e := s.coreImpact(id, cmd)
	if e != nil {
		return domain.CoreVersion{}, domain.CoreRevisionImpact{}, e
	}
	if impact.CurrentRevision != cmd.ExpectedRevision {
		return domain.CoreVersion{}, impact, domain.NewError(domain.ErrConflict, "REVISION_CONFLICT", "岩芯修订冲突", map[string]int{"currentRevision": impact.CurrentRevision})
	}
	if len(impact.BlockingReasons) > 0 {
		return domain.CoreVersion{}, impact, domain.NewError(domain.ErrInvalid, "CORE_REVISION_BLOCKED", "岩芯约束修订被阻止", impact.BlockingReasons)
	}
	next := impact.Proposed
	next.Revision++
	v := domain.CoreVersion{Core: next, ChangeSummary: impact.ChangeSummary, RevisionNote: strings.TrimSpace(cmd.RevisionNote), CreatedAt: time.Now().UTC()}
	if e = s.st.SaveCoreVersion(next, v); e != nil {
		return domain.CoreVersion{}, impact, e
	}
	return v, impact, nil
}

func (s *Service) CreateCase(c domain.SamplingCase, expected int) (domain.SamplingCase, error) {
	if e := domain.ValidateCaseBody(c); e != nil {
		return c, e
	}
	core, e := s.st.Core(c.CoreID)
	if e != nil {
		return c, e
	}
	l := s.lock(c.CoreID)
	l.Lock()
	defer l.Unlock()
	core, e = s.st.Core(c.CoreID)
	if e != nil {
		return c, e
	}
	if expected > 0 && core.Revision != expected {
		return c, domain.NewError(domain.ErrConflict, "REVISION_CONFLICT", "岩芯修订冲突", map[string]int{"currentRevision": core.Revision})
	}
	if c.CaseID == "" {
		c.CaseID = fmt.Sprintf("case-%d", time.Now().UnixNano())
	}
	if _, e = s.st.Case(c.CaseID); e == nil {
		return c, domain.NewError(domain.ErrConflict, "CASE_EXISTS", "案卷编号已存在", nil)
	}
	now := time.Now().UTC()
	c.Purpose = strings.TrimSpace(c.Purpose)
	c.Method = strings.TrimSpace(c.Method)
	c.RequestedSegments = domain.SortSegments(c.RequestedSegments)
	c.Status = domain.Draft
	c.Revision = 1
	c.CreatedAt = now
	c.UpdatedAt = now
	c.Findings = []domain.ReviewFinding{}
	v := domain.CaseVersion{Case: c, ChangeSummary: []domain.Change{}, RevisionNote: "创建案卷", CreatedAt: now}
	return c, s.st.SaveCaseVersion(c, v)
}
func (s *Service) GetCase(id string) (domain.SamplingCase, error)       { return s.st.Case(id) }
func (s *Service) CaseVersions(id string) ([]domain.CaseVersion, error) { return s.st.CaseVersions(id) }
func (s *Service) ReviseCaseCommand(id string, cmd CaseRevisionCommand) (domain.CaseVersion, error) {
	c, e := s.st.Case(id)
	if e != nil {
		return domain.CaseVersion{}, e
	}
	l := s.lock(c.CoreID)
	l.Lock()
	defer l.Unlock()
	c, e = s.st.Case(id)
	if e != nil {
		return domain.CaseVersion{}, e
	}
	if !domain.CanEdit(c.Status) {
		return domain.CaseVersion{}, domain.NewError(domain.ErrState, "CASE_NOT_EDITABLE", "仅 Draft 或 Returned 案卷可修订", nil)
	}
	if c.Revision != cmd.ExpectedRevision {
		return domain.CaseVersion{}, domain.NewError(domain.ErrConflict, "REVISION_CONFLICT", "案卷修订冲突", map[string]int{"currentRevision": c.Revision})
	}
	if strings.TrimSpace(cmd.RevisionNote) == "" {
		return domain.CaseVersion{}, domain.NewError(domain.ErrInvalid, "REVISION_NOTE_REQUIRED", "修订说明不能为空", nil)
	}
	next := c
	next.Purpose = strings.TrimSpace(cmd.Purpose)
	next.Method = strings.TrimSpace(cmd.Method)
	next.RequestedSegments = domain.SortSegments(cmd.RequestedSegments)
	next.EstimatedMassMg = cmd.EstimatedMassMg
	if e = domain.ValidateCaseBody(next); e != nil {
		return domain.CaseVersion{}, e
	}
	changes := []domain.Change{}
	if c.Purpose != next.Purpose {
		changes = append(changes, domain.Change{Field: "purpose", Before: c.Purpose, After: next.Purpose})
	}
	if c.Method != next.Method {
		changes = append(changes, domain.Change{Field: "method", Before: c.Method, After: next.Method})
	}
	if !reflect.DeepEqual(domain.SortSegments(c.RequestedSegments), next.RequestedSegments) {
		changes = append(changes, domain.Change{Field: "requestedSegments", Before: domain.SortSegments(c.RequestedSegments), After: next.RequestedSegments})
	}
	if c.EstimatedMassMg != next.EstimatedMassMg {
		changes = append(changes, domain.Change{Field: "estimatedMassMg", Before: c.EstimatedMassMg, After: next.EstimatedMassMg})
	}
	next.PrecheckDigest = ""
	next.AuthorizationDigest = ""
	next.Revision++
	next.UpdatedAt = time.Now().UTC()
	v := domain.CaseVersion{Case: next, ChangeSummary: changes, RevisionNote: strings.TrimSpace(cmd.RevisionNote), Actor: strings.TrimSpace(cmd.Actor), CreatedAt: next.UpdatedAt}
	if e = s.st.SaveCaseVersion(next, v); e != nil {
		return domain.CaseVersion{}, e
	}
	return v, nil
}
func (s *Service) ReviseCase(id string, expected int, purpose, method string, seg []domain.Segment) (domain.SamplingCase, error) {
	old, e := s.st.Case(id)
	if e != nil {
		return old, e
	}
	v, e := s.ReviseCaseCommand(id, CaseRevisionCommand{ExpectedRevision: expected, Purpose: purpose, Method: method, RequestedSegments: seg, EstimatedMassMg: old.EstimatedMassMg, RevisionNote: "兼容修订"})
	return v.Case, e
}

func (s *Service) Precheck(id string) (analysis.PrecheckResult, error) {
	c, e := s.st.Case(id)
	if e != nil {
		return analysis.PrecheckResult{}, e
	}
	l := s.lock(c.CoreID)
	l.Lock()
	defer l.Unlock()
	c, e = s.st.Case(id)
	if e != nil {
		return analysis.PrecheckResult{}, e
	}
	if c.Status != domain.Draft && c.Status != domain.Returned {
		return analysis.PrecheckResult{}, domain.NewError(domain.ErrState, "PRECHECK_STATE_INVALID", "当前状态不能执行预检", nil)
	}
	core, e := s.st.Core(c.CoreID)
	if e != nil {
		return analysis.PrecheckResult{}, e
	}
	r := analysis.Precheck(core, c, s.st.Cases())
	if existing, lookupErr := s.st.Precheck(r.Digest); lookupErr == nil {
		r = existing
	}
	c.PrecheckDigest = r.Digest
	c.UpdatedAt = time.Now().UTC()
	if e = s.st.SavePrecheck(c, r); e != nil {
		return analysis.PrecheckResult{}, e
	}
	return r, nil
}
func (s *Service) snapshotValidity(p domain.PrecheckSnapshot) (domain.SnapshotValidity, error) {
	c, e := s.st.Case(p.CaseID)
	if e != nil {
		return domain.SnapshotValidity{}, e
	}
	core, e := s.st.Core(c.CoreID)
	if e != nil {
		return domain.SnapshotValidity{}, e
	}
	current := analysis.Precheck(core, c, s.st.Cases())
	var sources []string
	if c.Revision != p.CaseRevision {
		sources = append(sources, "caseRevision")
	}
	if core.Revision != p.CoreRevision {
		sources = append(sources, "coreRevision")
	}
	if !reflect.DeepEqual(current.ActiveCases, p.ActiveCases) {
		sources = append(sources, "activeCases")
	}
	if current.InputFingerprint != p.InputFingerprint && len(sources) == 0 {
		sources = append(sources, "caseInputs")
	}
	return domain.SnapshotValidity{Valid: len(sources) == 0, ChangeSources: sources}, nil
}
func (s *Service) GetPrecheck(caseID, digest string) (domain.PrecheckSnapshot, domain.SnapshotValidity, error) {
	p, e := s.st.Precheck(digest)
	if e != nil {
		return p, domain.SnapshotValidity{}, e
	}
	if p.CaseID != caseID {
		return domain.PrecheckSnapshot{}, domain.SnapshotValidity{}, domain.ErrNotFound
	}
	v, e := s.snapshotValidity(p)
	return p, v, e
}
func (s *Service) requireCurrentPrecheck(c domain.SamplingCase) error {
	if c.PrecheckDigest == "" {
		return domain.NewError(domain.ErrState, "PRECHECK_REQUIRED", "需要当前修订的预检快照", nil)
	}
	p, e := s.st.Precheck(c.PrecheckDigest)
	if e != nil {
		return e
	}
	v, e := s.snapshotValidity(p)
	if e != nil {
		return e
	}
	if !p.Pass {
		return domain.NewError(domain.ErrState, "PRECHECK_FAILED", "当前预检未通过", p.Findings)
	}
	if !v.Valid {
		return domain.NewError(domain.ErrState, "PRECHECK_STALE", "当前预检快照已失效", v.ChangeSources)
	}
	return nil
}

func (s *Service) ReturnCaseIssues(id string, issues []ReviewIssue) (domain.SamplingCase, error) {
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
	if c.Status != domain.Submitted && c.Status != domain.Draft {
		return c, domain.NewError(domain.ErrState, "REVIEW_STATE_INVALID", "当前状态不能退回复核问题", nil)
	}
	if len(issues) == 0 {
		return c, domain.NewError(domain.ErrInvalid, "FINDINGS_REQUIRED", "至少提交一项复核问题", nil)
	}
	seen := map[string]bool{}
	for _, f := range c.Findings {
		if f.Status == "Open" && f.OpenedRevision == c.Revision {
			seen[f.Code+"\x00"+f.Message+fmt.Sprint(f.SegmentRef)] = true
		}
	}
	for _, x := range issues {
		if strings.TrimSpace(x.Code) == "" || strings.TrimSpace(x.Message) == "" {
			return c, domain.NewError(domain.ErrInvalid, "FINDING_INVALID", "问题 code 和 message 不能为空", nil)
		}
		if x.SegmentRef != nil && !x.SegmentRef.Valid() {
			return c, domain.NewError(domain.ErrInvalid, "SEGMENT_REF_INVALID", "问题定位区段不合法", nil)
		}
		if x.SegmentRef != nil {
			located := false
			for _, target := range c.RequestedSegments {
				if x.SegmentRef.Start >= target.Start && x.SegmentRef.End <= target.End {
					located = true
					break
				}
			}
			if !located {
				return c, domain.NewError(domain.ErrInvalid, "SEGMENT_REF_OUTSIDE_CASE", "问题定位区段必须属于当前案卷目标区段", x.SegmentRef)
			}
		}
		k := x.Code + "\x00" + x.Message + fmt.Sprint(x.SegmentRef)
		if seen[k] {
			return c, domain.NewError(domain.ErrConflict, "DUPLICATE_OPEN_FINDING", "同一案卷版本存在重复开放问题", nil)
		}
		seen[k] = true
	}
	openedRevision := c.Revision
	previousStatus := c.Status
	now := time.Now().UTC()
	c.Revision++
	c.Status = domain.Returned
	c.PrecheckDigest = ""
	c.AuthorizationDigest = ""
	c.UpdatedAt = now
	events := []domain.FindingEvent{}
	for i, x := range issues {
		idn := fmt.Sprintf("finding-%d-%d", now.UnixNano(), i)
		f := domain.ReviewFinding{FindingID: idn, CaseID: id, Code: strings.TrimSpace(x.Code), Message: strings.TrimSpace(x.Message), SegmentRef: x.SegmentRef, Status: "Open", OpenedRevision: openedRevision, OpenedAt: now}
		c.Findings = append(c.Findings, f)
		events = append(events, domain.FindingEvent{EventID: "open-" + idn, FindingID: idn, CaseID: id, Type: "Opened", CaseRevision: openedRevision, Code: f.Code, Message: f.Message, SegmentRef: f.SegmentRef, OccurredAt: now})
	}
	v := domain.CaseVersion{Case: c, ChangeSummary: []domain.Change{{Field: "status", Before: previousStatus, After: domain.Returned}}, RevisionNote: "复核退回", CreatedAt: now}
	return c, s.st.SaveFindingChanges(c, v, events)
}
func (s *Service) ReturnCase(id, code, msg string) (domain.SamplingCase, error) {
	return s.ReturnCaseIssues(id, []ReviewIssue{{Code: code, Message: msg}})
}
func (s *Service) CloseFindingWithRevision(id, fid, note string, caseRevision int) (domain.SamplingCase, error) {
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
	if c.Status != domain.Returned {
		return c, domain.NewError(domain.ErrState, "FINDING_CLOSE_STATE_INVALID", "仅 Returned 状态可关闭问题", nil)
	}
	if strings.TrimSpace(note) == "" {
		return c, domain.NewError(domain.ErrInvalid, "CLOSURE_NOTE_REQUIRED", "闭环说明不能为空", nil)
	}
	if caseRevision != c.Revision {
		return c, domain.NewError(domain.ErrConflict, "CLOSURE_REVISION_STALE", "闭环证据必须绑定当前案卷修订", map[string]int{"currentRevision": c.Revision})
	}
	idx := -1
	for i := range c.Findings {
		if c.Findings[i].FindingID == fid {
			idx = i
			break
		}
	}
	if idx < 0 {
		return c, domain.NewError(domain.ErrNotFound, "FINDING_NOT_FOUND", "复核问题不存在", nil)
	}
	if c.Findings[idx].Status == "Closed" {
		return c, domain.NewError(domain.ErrConflict, "FINDING_ALREADY_CLOSED", "复核问题已关闭", nil)
	}
	now := time.Now().UTC()
	c.Findings[idx].Status = "Closed"
	c.Findings[idx].ClosureNote = strings.TrimSpace(note)
	c.Findings[idx].ClosedRevision = caseRevision
	c.Findings[idx].ClosedAt = now
	c.UpdatedAt = now
	event := domain.FindingEvent{EventID: fmt.Sprintf("close-%d", now.UnixNano()), FindingID: fid, CaseID: id, Type: "Closed", CaseRevision: caseRevision, ClosureNote: strings.TrimSpace(note), OccurredAt: now}
	return c, s.st.SaveFindingEvents(c, []domain.FindingEvent{event})
}
func (s *Service) CloseFinding(id, fid, note string) (domain.SamplingCase, error) {
	c, e := s.st.Case(id)
	if e != nil {
		return c, e
	}
	return s.CloseFindingWithRevision(id, fid, note, c.Revision)
}
func (s *Service) Findings(id, status string, revision int) ([]domain.ReviewFinding, []domain.FindingEvent, error) {
	c, e := s.st.Case(id)
	if e != nil {
		return nil, nil, e
	}
	out := []domain.ReviewFinding{}
	for _, f := range c.Findings {
		if status != "" && f.Status != status {
			continue
		}
		if revision > 0 && f.OpenedRevision != revision && f.ClosedRevision != revision {
			continue
		}
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].OpenedRevision != out[j].OpenedRevision {
			return out[i].OpenedRevision < out[j].OpenedRevision
		}
		return out[i].FindingID < out[j].FindingID
	})
	events := s.st.FindingEvents(id)
	if revision > 0 {
		filtered := events[:0]
		for _, event := range events {
			if event.CaseRevision == revision {
				filtered = append(filtered, event)
			}
		}
		events = filtered
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].CaseRevision != events[j].CaseRevision {
			return events[i].CaseRevision < events[j].CaseRevision
		}
		return events[i].EventID < events[j].EventID
	})
	return out, events, nil
}
func (s *Service) findingsClosedForCurrent(c domain.SamplingCase) error {
	var ids []string
	for _, f := range c.Findings {
		if f.Status != "Closed" || f.ClosedRevision != c.Revision {
			ids = append(ids, f.FindingID)
		}
	}
	if len(ids) > 0 {
		return domain.NewError(domain.ErrState, "FINDINGS_NOT_CLOSED", "存在未闭环或未绑定当前修订的问题", ids)
	}
	return nil
}
func (s *Service) SubmitCase(id string) (domain.SamplingCase, error) {
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
	if !domain.CanEdit(c.Status) {
		return c, domain.NewError(domain.ErrState, "SUBMIT_STATE_INVALID", "当前状态不能提交", nil)
	}
	if e = s.findingsClosedForCurrent(c); e != nil {
		return c, e
	}
	if e = s.requireCurrentPrecheck(c); e != nil {
		return c, e
	}
	c.Status = domain.Submitted
	c.UpdatedAt = time.Now().UTC()
	return c, s.st.SaveCase(c)
}
