// Package errs defines errors shared by domain services and their callers.
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
