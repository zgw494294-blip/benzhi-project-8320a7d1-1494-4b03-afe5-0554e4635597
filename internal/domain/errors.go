package domain

import "fmt"

type ValidationError struct{ Field, Message string }

func (e ValidationError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Message) }
func Require(v string, field string) error {
	if v == "" {
		return ValidationError{field, "不能为空"}
	}
	return nil
}
