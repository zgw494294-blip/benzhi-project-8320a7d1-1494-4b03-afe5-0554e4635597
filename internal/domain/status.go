package domain

var TerminalStatuses = map[string]bool{Frozen: true}

func IsTerminal(status string) bool { return TerminalStatuses[status] }
func CanEdit(status string) bool    { return status == Draft || status == Returned }
func CanExecute(status string) bool { return status == Authorized }
