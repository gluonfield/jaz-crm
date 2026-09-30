package interactions

import "testing"

// Automated senders are recognised wherever the marker sits in the local
// part, without catching people whose names contain it.
func TestAutomatedSenders(t *testing.T) {
	for address, want := range map[string]bool{
		"noreply@shop.com":            true,
		"payments-noreply@google.com": true,
		"no-reply+x1@stripe.com":      true,
		"team.notifications@app.io":   true,
		"mailer-daemon@mx.cas.dev":    true,
		"ada@customer.io":             false,
		"noreplyada@customer.io":      false,
		"bounceback@studio.io":        false,
	} {
		if got := automated.MatchString(address); got != want {
			t.Errorf("%s: %v, want %v", address, got, want)
		}
	}
}
