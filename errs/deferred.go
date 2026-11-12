package errs

import "time"

type Deferred struct {
	Reason  string
	RetryAt time.Time
}

func (e *Deferred) Error() string {
	return e.Reason
}
