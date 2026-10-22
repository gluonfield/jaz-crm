package followups

import "regexp"

var (
	dashRange = regexp.MustCompile(`(\d)[ \t]*(?:—|–|--+)[ \t]*(\d)`)
	dashEdge  = regexp.MustCompile(`(?m)^[ \t]*(?:—|–|--+)[ \t]*|[ \t]*(?:—|–|--+)[ \t]*$`)
	dash      = regexp.MustCompile(`[ \t]*(?:—|–|--+)[ \t]*`)
)

// plain writes text as people type it, without the em dashes, en dashes and
// double hyphens models use as punctuation: a range such as 7–8 keeps a
// hyphen, a dash opening or closing a line, as in a "—Name" sign-off, goes,
// and any other becomes a comma. Hyphens and "- " bullets stay.
func plain(s string) string {
	s = dashRange.ReplaceAllString(s, "$1-$2")
	s = dashEdge.ReplaceAllString(s, "")
	return dash.ReplaceAllString(s, ", ")
}
