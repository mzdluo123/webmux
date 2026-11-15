package historysource

import "strconv"

// ParseError contains only a byte position and a safe error category, never the
// failing native record or a driver error that could include a source body.
type ParseError struct {
	Offset int64
	Cause  error
}

func (e *ParseError) Error() string {
	return "native history failure at byte " + strconv.FormatInt(e.Offset, 10) + ": " + e.Cause.Error()
}

func (e *ParseError) Unwrap() error { return e.Cause }
