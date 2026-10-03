package followups

import (
	"strings"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/errs"
)

func actionDate(value, timezone string) (string, error) {
	value = strings.TrimSpace(value)
	if _, err := time.Parse(time.DateOnly, value); err == nil {
		return value, nil
	}
	zone, err := time.LoadLocation(timezone)
	if err != nil {
		return "", err
	}
	const layout = "2006-01-02T15:04"
	at, err := time.ParseInLocation(layout, value, zone)
	if err != nil || at.Format(layout) != value {
		return "", errs.Invalidf("action_date must be a date or a valid local date/time in %s", timezone)
	}
	return at.Format(time.RFC3339), nil
}
