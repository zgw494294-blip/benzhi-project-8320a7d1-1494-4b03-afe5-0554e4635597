package analysis

import (
	"corepreservation/internal/domain"
	"reflect"
	"sort"
)

func AuthorizationDigest(m domain.AuthorizationManifest) string {
	m.AuthorizationDigest = ""
	m.RequestedSegments = domain.SortSegments(m.RequestedSegments)
	return domain.Digest(m)
}
func ExecutionRequestDigest(auth string, segs []domain.Segment, before, sample, after int, container, operator, witness string) string {
	return domain.Digest(struct {
		AuthorizationDigest                     string
		ActualSegments                          []domain.Segment
		MassBeforeMg, SampleMassMg, MassAfterMg int
		ContainerCode, Operator, Witness        string
	}{auth, domain.SortSegments(segs), before, sample, after, container, operator, witness})
}
func ExecutionViolations(m domain.AuthorizationManifest, core domain.CoreRecord, segs []domain.Segment, before, sample, after int, container, operator, witness string) []domain.Violation {
	var out []domain.Violation
	if len(segs) == 0 {
		out = append(out, domain.Violation{Code: "ACTUAL_SEGMENTS_REQUIRED", Message: "实际切割区段不能为空"})
	}
	for _, seg := range segs {
		x := seg
		ok := false
		if seg.Start < core.DepthStartMm || seg.End > core.DepthEndMm {
			out = append(out, domain.Violation{Code: "SEGMENT_OUTSIDE_CORE", Message: "实际区段超出岩芯初始深度范围", Segment: &x})
		}
		for _, p := range core.ProtectedIntervals {
			if p.Overlap(domain.Interval{Start: seg.Start, End: seg.End}) {
				out = append(out, domain.Violation{Code: "SEGMENT_OVERLAPS_PROTECTED", Message: "实际区段越过保护区边界", Segment: &x})
				break
			}
		}
		for _, a := range m.RequestedSegments {
			if seg.Start >= a.Start-m.DepthToleranceMm && seg.End <= a.End+m.DepthToleranceMm && seg.Valid() {
				ok = true
				break
			}
		}
		if !ok {
			out = append(out, domain.Violation{Code: "SEGMENT_OUTSIDE_AUTHORIZATION", Message: "实际区段超出授权深度容差", Segment: &x})
		}
	}
	if sample > m.EstimatedMassMg {
		out = append(out, domain.Violation{Code: "SAMPLE_MASS_EXCEEDS_AUTHORIZATION", Message: "样品质量超过授权上限"})
	}
	if before != core.AvailableMassMg {
		out = append(out, domain.Violation{Code: "MASS_BEFORE_MISMATCH", Message: "执行前质量与岩芯当前可用质量不一致"})
	}
	if !domain.ValidMassTriplet(before, sample, after) {
		out = append(out, domain.Violation{Code: "MASS_CONSERVATION_FAILED", Message: "执行质量不守恒"})
	}
	if after < core.MinimumReserveMassMg {
		out = append(out, domain.Violation{Code: "RESERVE_LOW", Message: "执行后余样低于最低保留质量"})
	}
	if container == "" {
		out = append(out, domain.Violation{Code: "CONTAINER_REQUIRED", Message: "容器编号不能为空"})
	}
	if operator == "" {
		out = append(out, domain.Violation{Code: "OPERATOR_REQUIRED", Message: "操作者不能为空"})
	}
	if witness == "" {
		out = append(out, domain.Violation{Code: "WITNESS_REQUIRED", Message: "见证人不能为空"})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Code != out[j].Code {
			return out[i].Code < out[j].Code
		}
		if out[i].Segment == nil {
			return true
		}
		if out[j].Segment == nil {
			return false
		}
		if out[i].Segment.Start != out[j].Segment.Start {
			return out[i].Segment.Start < out[j].Segment.Start
		}
		return out[i].Segment.End < out[j].Segment.End
	})
	return out
}
func VerificationRequestDigest(remaining int, location, verifier, witness string) string {
	return domain.Digest(struct {
		RemainingMassMg                        int
		StorageLocation, Verifier, WitnessNote string
	}{remaining, location, verifier, witness})
}
func VerificationReasons(e domain.CutExecution, core domain.CoreRecord, remaining int, location, verifier, witness string) []domain.Violation {
	var out []domain.Violation
	if !domain.ValidMassTriplet(e.MassBeforeMg, e.SampleMassMg, e.MassAfterMg) {
		out = append(out, domain.Violation{Code: "MASS_CONSERVATION_FAILED", Message: "执行记录质量不守恒"})
	}
	if remaining != e.MassAfterMg {
		out = append(out, domain.Violation{Code: "REMAINING_MASS_MISMATCH", Message: "实测余样与执行后质量不一致"})
	}
	if remaining < core.MinimumReserveMassMg {
		out = append(out, domain.Violation{Code: "RESERVE_LOW", Message: "实测余样低于最低保留质量"})
	}
	if location == "" {
		out = append(out, domain.Violation{Code: "STORAGE_LOCATION_REQUIRED", Message: "容器位置不能为空"})
	}
	if verifier == "" {
		out = append(out, domain.Violation{Code: "VERIFIER_REQUIRED", Message: "核验人不能为空"})
	}
	if witness == "" {
		out = append(out, domain.Violation{Code: "WITNESS_NOTE_REQUIRED", Message: "见证说明不能为空"})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}
func CredentialDigest(c domain.ProvenanceCredential) string { c.Digest = ""; return domain.Digest(c) }

func Availability(core domain.CoreRecord, cases []domain.SamplingCase) domain.AvailabilityView {
	points := []int{core.DepthStartMm, core.DepthEndMm}
	activeMass := 0
	for _, p := range core.ProtectedIntervals {
		points = append(points, p.Start, p.End)
	}
	for _, c := range cases {
		if c.CoreID != core.CoreID {
			continue
		}
		if c.Status == domain.Frozen && c.Execution != nil {
			for _, x := range c.Execution.ActualSegments {
				points = append(points, x.Start, x.End)
			}
		} else if !domain.IsTerminal(c.Status) {
			activeMass += c.EstimatedMassMg
			for _, x := range c.RequestedSegments {
				points = append(points, x.Start, x.End)
			}
		}
	}
	sort.Ints(points)
	uniq := points[:0]
	for _, p := range points {
		if p < core.DepthStartMm || p > core.DepthEndMm {
			continue
		}
		if len(uniq) == 0 || uniq[len(uniq)-1] != p {
			uniq = append(uniq, p)
		}
	}
	view := domain.AvailabilityView{CoreID: core.CoreID, CoreRevision: core.Revision, AvailableMassMg: core.AvailableMassMg, MinimumReserveMassMg: core.MinimumReserveMassMg, ActiveEstimatedOccupancyMassMg: activeMass}
	view.RequestableMassBudgetMg = core.AvailableMassMg - core.MinimumReserveMassMg - activeMass
	if view.RequestableMassBudgetMg < 0 {
		view.RequestableMassBudgetMg = 0
	}
	for i := 0; i+1 < len(uniq); i++ {
		a, b := uniq[i], uniq[i+1]
		if a == b {
			continue
		}
		reasons := []domain.OccupancyReason{}
		for _, p := range core.ProtectedIntervals {
			if a < p.End && p.Start < b {
				reasons = append(reasons, domain.OccupancyReason{Type: "Protected"})
			}
		}
		for _, c := range cases {
			if c.CoreID != core.CoreID {
				continue
			}
			if c.Status == domain.Frozen && c.Execution != nil {
				for _, x := range c.Execution.ActualSegments {
					if a < x.End && x.Start < b {
						r := domain.OccupancyReason{Type: "Frozen", CaseID: c.CaseID, Status: c.Status, Revision: c.Revision}
						if c.Credential != nil {
							r.CredentialID = c.Credential.CredentialID
							r.Digest = c.Credential.Digest
						}
						reasons = append(reasons, r)
						break
					}
				}
			} else if !domain.IsTerminal(c.Status) {
				for _, x := range c.RequestedSegments {
					if a < x.End && x.Start < b {
						reasons = append(reasons, domain.OccupancyReason{Type: "ActiveCase", CaseID: c.CaseID, Status: c.Status, Revision: c.Revision})
						break
					}
				}
			}
		}
		sort.Slice(reasons, func(i, j int) bool {
			if reasons[i].Type != reasons[j].Type {
				return reasons[i].Type < reasons[j].Type
			}
			return reasons[i].CaseID < reasons[j].CaseID
		})
		kind := "Available"
		if len(reasons) > 0 {
			kind = "Occupied"
		}
		part := domain.AvailabilityPartition{Start: a, End: b, Kind: kind, Reasons: reasons}
		if n := len(view.Partitions); n > 0 && view.Partitions[n-1].End == a && view.Partitions[n-1].Kind == kind && reflect.DeepEqual(view.Partitions[n-1].Reasons, reasons) {
			view.Partitions[n-1].End = b
		} else {
			view.Partitions = append(view.Partitions, part)
		}
	}
	return view
}
