package application

import (
	"fmt"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/fault"
)

type Error = fault.Error

func Invalid(code string, err error) error  { return &Error{Kind: "invalid", Code: code, Cause: err} }
func Conflict(code string, err error) error { return &Error{Kind: "conflict", Code: code, Cause: err} }
func unavailable(code, message string) error {
	return &Error{Kind: "unavailable", Code: code, Cause: fmt.Errorf("%s", message)}
}
