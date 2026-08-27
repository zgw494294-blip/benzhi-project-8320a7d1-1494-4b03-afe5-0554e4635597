package domain

import "sort"

func NormalizeIntervals(in []Interval) []Interval {
	out := append([]Interval(nil), in...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Start == out[j].Start {
			return out[i].End < out[j].End
		}
		return out[i].Start < out[j].Start
	})
	if len(out) == 0 {
		return []Interval{}
	}
	merged := []Interval{out[0]}
	for _, current := range out[1:] {
		last := &merged[len(merged)-1]
		if current.Start <= last.End {
			if current.End > last.End {
				last.End = current.End
			}
			continue
		}
		merged = append(merged, current)
	}
	return merged
}

func NormalizeCase(c SamplingCase) SamplingCase {
	c.RequestedSegments = SortSegments(c.RequestedSegments)
	sort.Slice(c.Findings, func(i, j int) bool { return c.Findings[i].FindingID < c.Findings[j].FindingID })
	return c
}
func SegmentDigest(s []Segment) string { return Digest(SortSegments(s)) }
