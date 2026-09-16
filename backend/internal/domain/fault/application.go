package fault

// Error describes a use-case failure without depending on HTTP.
type Error struct {
	Kind, Code string
	Cause      error
}

func (e *Error) Error() string { return e.Cause.Error() }
func (e *Error) Unwrap() error { return e.Cause }
