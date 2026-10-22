// Package errs marks errors that are the caller's mistake, which transports
// show as is; every other error is reported as an internal failure.
package errs

import "fmt"

type Invalid struct {
	Message string
}

func (e Invalid) Error() string {
	return e.Message
}

func Invalidf(format string, args ...any) error {
	return Invalid{Message: fmt.Sprintf(format, args...)}
}
