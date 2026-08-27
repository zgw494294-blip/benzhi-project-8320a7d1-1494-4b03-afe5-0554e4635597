package domain

import "testing"

func TestTransitionAndDigest(t *testing.T) {
	c := SamplingCase{Status: Submitted}
	if e := c.ValidateTransition(Authorized); e != nil {
		t.Fatal(e)
	}
	if Digest("x") == Digest("y") {
		t.Fatal("digest collision")
	}
	if (Interval{Start: 0, End: 2}).Overlap(Interval{Start: 2, End: 3}) {
		t.Fatal("half open")
	}
}
