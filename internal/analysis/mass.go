package analysis

func RemainingMass(before, sample int) int         { return before - sample }
func ReserveSatisfied(remaining, reserve int) bool { return remaining >= reserve }
