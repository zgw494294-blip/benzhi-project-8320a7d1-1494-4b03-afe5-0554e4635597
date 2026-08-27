package analysis

import "corepreservation/internal/domain"

func TotalLength(s []domain.Segment) int {
	n := 0
	for _, x := range s {
		n += x.End - x.Start
	}
	return n
}
func ActualWithin(auth, actual []domain.Segment, tolerance int) bool {
	if len(actual) == 0 {
		return false
	}
	for _, a := range actual {
		ok := false
		for _, x := range auth {
			if a.Start >= x.Start-tolerance && a.End <= x.End+tolerance {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}
func Available(core domain.CoreRecord, frozen []domain.SamplingCase) []domain.Segment {
	used := append([]domain.Segment(nil), func() []domain.Segment {
		var z []domain.Segment
		for _, c := range frozen {
			z = append(z, c.RequestedSegments...)
		}
		return z
	}()...)
	used = domain.SortSegments(used)
	cur := core.DepthStartMm
	out := []domain.Segment{}
	for _, u := range used {
		if u.Start > cur {
			out = append(out, domain.Segment{Start: cur, End: u.Start})
		}
		if u.End > cur {
			cur = u.End
		}
	}
	if cur < core.DepthEndMm {
		out = append(out, domain.Segment{Start: cur, End: core.DepthEndMm})
	}
	return out
}
func ValidateMass(before, sample, after, reserve int) bool {
	return before >= 0 && sample >= 0 && after >= 0 && before-sample == after && after >= reserve
}
