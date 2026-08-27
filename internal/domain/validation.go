package domain

func ValidMassTriplet(before, sample, after int) bool {
	return before >= 0 && sample >= 0 && after >= 0 && before-sample == after
}
func ValidDepthRange(start, end int) bool { return start >= 0 && end > start }
func Contains(outer, inner Interval) bool {
	return inner.Start >= outer.Start && inner.End <= outer.End
}
