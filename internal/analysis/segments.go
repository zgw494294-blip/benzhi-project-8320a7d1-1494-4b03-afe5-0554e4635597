package analysis

import "corepreservation/internal/domain"

func MergeSegments(in []domain.Segment) []domain.Segment {
	s := domain.SortSegments(in)
	if len(s) == 0 {
		return s
	}
	out := []domain.Segment{s[0]}
	for _, x := range s[1:] {
		last := &out[len(out)-1]
		if x.Start <= last.End {
			if x.End > last.End {
				last.End = x.End
			}
		} else {
			out = append(out, x)
		}
	}
	return out
}
func IntersectsAny(x domain.Segment, list []domain.Segment) bool {
	for _, y := range list {
		if x.Start < y.End && y.Start < x.End {
			return true
		}
	}
	return false
}
func Subtract(base domain.Segment, cuts []domain.Segment) []domain.Segment {
	cur := base.Start
	out := []domain.Segment{}
	for _, c := range MergeSegments(cuts) {
		if c.End <= cur || c.Start >= base.End {
			continue
		}
		if c.Start > cur {
			out = append(out, domain.Segment{Start: cur, End: min(c.Start, base.End)})
		}
		if c.End > cur {
			cur = c.End
		}
	}
	if cur < base.End {
		out = append(out, domain.Segment{Start: cur, End: base.End})
	}
	return out
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
