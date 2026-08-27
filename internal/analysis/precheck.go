package analysis

import (
	"corepreservation/internal/domain"
	"fmt"
	"sort"
	"strings"
	"time"
)

const PrecheckRuleVersion = "precheck-v2"

type Finding = domain.PrecheckFinding
type PrecheckResult = domain.PrecheckSnapshot

func ActiveSummaries(candidate domain.SamplingCase, cases []domain.SamplingCase) []domain.ActiveCaseSummary {
	out := make([]domain.ActiveCaseSummary, 0)
	for _, c := range cases {
		if c.CoreID == candidate.CoreID && c.CaseID != candidate.CaseID && !domain.IsTerminal(c.Status) {
			out = append(out, domain.ActiveCaseSummary{CaseID: c.CaseID, Status: c.Status, Revision: c.Revision, Segments: domain.SortSegments(c.RequestedSegments), EstimatedMassMg: c.EstimatedMassMg})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CaseID < out[j].CaseID })
	return out
}
func Precheck(core domain.CoreRecord, candidate domain.SamplingCase, cases []domain.SamplingCase) PrecheckResult {
	core.ProtectedIntervals = domain.NormalizeIntervals(core.ProtectedIntervals)
	segs := domain.SortSegments(candidate.RequestedSegments)
	active := ActiveSummaries(candidate, cases)
	fingerprint := domain.Digest(struct {
		CaseID, CoreID             string
		CaseRevision, CoreRevision int
		Core                       domain.CoreRecord
		Segments                   []domain.Segment
		Estimated                  int
		Purpose, Method, Rule      string
		Active                     []domain.ActiveCaseSummary
	}{candidate.CaseID, core.CoreID, candidate.Revision, core.Revision, core, segs, candidate.EstimatedMassMg, strings.TrimSpace(candidate.Purpose), strings.TrimSpace(candidate.Method), PrecheckRuleVersion, active})
	r := domain.PrecheckSnapshot{InputFingerprint: fingerprint, CaseID: candidate.CaseID, CaseRevision: candidate.Revision, CoreID: core.CoreID, CoreRevision: core.Revision, Core: core, RequestedSegments: segs, EstimatedMassMg: candidate.EstimatedMassMg, Purpose: strings.TrimSpace(candidate.Purpose), Method: strings.TrimSpace(candidate.Method), ActiveCases: active, RuleVersion: PrecheckRuleVersion, Findings: []domain.PrecheckFinding{}, RemainingMassMg: core.AvailableMassMg - candidate.EstimatedMassMg, Pass: true, CreatedAt: time.Now().UTC()}
	add := func(code, msg string, seg *domain.Segment, related string) {
		r.Pass = false
		var cp *domain.Segment
		if seg != nil {
			x := *seg
			cp = &x
		}
		r.Findings = append(r.Findings, domain.PrecheckFinding{Code: code, Message: msg, Segment: cp, RelatedCaseID: related})
	}
	if strings.TrimSpace(candidate.Purpose) == "" {
		add("PURPOSE_REQUIRED", "必须填写研究目的", nil, "")
	}
	if strings.TrimSpace(candidate.Method) == "" {
		add("METHOD_REQUIRED", "必须填写操作方法", nil, "")
	}
	if candidate.EstimatedMassMg <= 0 {
		add("ESTIMATED_MASS_INVALID", "预计切割质量必须为正数", nil, "")
	}
	if e := domain.ValidateSegments(segs); e != nil {
		add("SEGMENTS_INVALID", e.Error(), nil, "")
	}
	for _, seg := range segs {
		x := seg
		if seg.Start < core.DepthStartMm || seg.End > core.DepthEndMm || !seg.Valid() {
			add("OUT_OF_BOUNDS", "目标区段超出岩芯范围", &x, "")
			continue
		}
		for _, p := range core.ProtectedIntervals {
			if p.Overlap(domain.Interval{Start: seg.Start, End: seg.End}) {
				add("PROTECTED_OVERLAP", "目标区段命中禁止切割区段", &x, "")
			}
		}
		for _, a := range active {
			for _, other := range a.Segments {
				if seg.Start < other.End && other.Start < seg.End {
					add("CASE_OVERLAP", "与未终结取样案重叠", &x, a.CaseID)
				}
			}
		}
	}
	if r.RemainingMassMg < core.MinimumReserveMassMg {
		add("RESERVE_LOW", fmt.Sprintf("预计余样 %d 低于最低保留量 %d", r.RemainingMassMg, core.MinimumReserveMassMg), nil, "")
	}
	sort.Slice(r.Findings, func(i, j int) bool {
		a, b := r.Findings[i], r.Findings[j]
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.RelatedCaseID != b.RelatedCaseID {
			return a.RelatedCaseID < b.RelatedCaseID
		}
		as, ae, bs, be := 0, 0, 0, 0
		if a.Segment != nil {
			as, ae = a.Segment.Start, a.Segment.End
		}
		if b.Segment != nil {
			bs, be = b.Segment.Start, b.Segment.End
		}
		if as != bs {
			return as < bs
		}
		if ae != be {
			return ae < be
		}
		return a.Message < b.Message
	})
	r.Digest = domain.Digest(struct {
		Fingerprint string
		Findings    []domain.PrecheckFinding
		Remaining   int
		Pass        bool
	}{r.InputFingerprint, r.Findings, r.RemainingMassMg, r.Pass})
	return r
}
