package interactions

import (
	"cmp"
	"regexp"
	"slices"
	"strings"
)

// readable shows provider text without what no reader wants: quoted history,
// disclaimers, separator lines, dial-in blocks, inline image markers and
// Outlook's "label<url>" links. What
// remains is plain text with paragraphs, "- " items and bare links. Stored
// content stays as it was; notes and transcripts pass as they are.
func readable(kind, content string) string {
	if kind != "message" && kind != "description" {
		return content
	}
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, " ", " ")
	content = angleLink.ReplaceAllStringFunc(content, func(link string) string {
		if strings.HasPrefix(link, "<http") {
			return " " + strings.Trim(link, "<>")
		}
		return ""
	})
	content = inlineImage.ReplaceAllString(content, "")
	return tidyLines(dialIns(history(strings.Split(content, "\n"))))
}

var (
	angleLink   = regexp.MustCompile(`<(?:https?|mailto|tel):[^\s<>]+>`)
	inlineImage = regexp.MustCompile(`\[(cid|image):[^\]]*\]`)
)

var (
	wrote       = regexp.MustCompile(`^On\b.*\bwrote:$`)
	historyLine = regexp.MustCompile(`(?i)^(-+ ?original message ?-+|~~//~~)$|^(confidentiality notice|disclaimer|this (e-?mail|message) (and any|is intended|may contain|contains confidential))`)
	headerFrom  = regexp.MustCompile(`^From: \S`)
	headerSent  = regexp.MustCompile(`^(Sent|Date): \S`)
)

// history cuts a message where the history it quotes begins, unless only
// quotes would be left. A forwarded message keeps its headers and content.
func history(lines []string) []string {
	forwarded := slices.ContainsFunc(lines, func(l string) bool { return strings.Contains(l, "Forwarded message") })
	for i := range lines {
		line := strings.TrimSpace(lines[i])
		rest := lines[i+1 : min(i+5, len(lines))]
		cut := wrote.MatchString(line) || historyLine.MatchString(line) ||
			strings.HasPrefix(line, "On ") && len(rest) > 0 && strings.HasSuffix(strings.TrimSpace(rest[0]), "wrote:") ||
			!forwarded && headerFrom.MatchString(line) && slices.ContainsFunc(rest, func(l string) bool { return headerSent.MatchString(strings.TrimSpace(l)) }) ||
			strings.HasPrefix(line, ">") && !slices.ContainsFunc(lines[i:], unquoted)
		if cut && slices.ContainsFunc(lines[:i], unquoted) {
			return lines[:i]
		}
	}
	return lines
}

func unquoted(line string) bool {
	line = strings.TrimSpace(line)
	return line != "" && !strings.HasPrefix(line, ">")
}

var (
	dialInStart = regexp.MustCompile(`(?i)^(microsoft teams (meeting|need help)|join zoom meeting|join with google meet)`)
	dialInLine  = regexp.MustCompile(`(?i)^(join|meeting id|passcode|password|pin|phone conference id|dial|find a local number|find your local number|need help|for organi[sz]ers|more phone numbers|one tap mobile|or call|video id|tenant key|learn more|meeting options|reset dial-in pin|https?://\S+$)`)
	phoneLine   = regexp.MustCompile(`^(\([A-Z]{2}\) )?[+\d][\d\s().,#*+-]{6,}`)
	joinLink    = regexp.MustCompile(`https?://(teams\.microsoft\.com|[\w.-]*zoom\.us|meet\.google\.com)/\S+`)
	separator   = regexp.MustCompile(`^[_\-=*~—–]{5,}$`)
)

// dialIns replaces each conferencing block, from its heading through its
// dial-in lines, with one line naming the meeting and its join link.
func dialIns(lines []string) []string {
	var out []string
	for i := 0; i < len(lines); i++ {
		if !dialInStart.MatchString(strings.TrimSpace(lines[i])) {
			out = append(out, lines[i])
			continue
		}
		link := joinLink.FindString(lines[i])
		j := i + 1
		for ; j < len(lines); j++ {
			line := strings.TrimSpace(lines[j])
			if line != "" && !separator.MatchString(line) && !dialInLine.MatchString(line) && !phoneLine.MatchString(line) {
				break
			}
			link = cmp.Or(link, joinLink.FindString(line))
		}
		if link != "" {
			out = append(out, meetingName(link)+": "+link, "")
		}
		i = j - 1
	}
	return out
}

func meetingName(link string) string {
	switch {
	case strings.Contains(link, "teams.microsoft.com"):
		return "Microsoft Teams meeting"
	case strings.Contains(link, "zoom.us"):
		return "Zoom meeting"
	}
	return "Google Meet"
}

var (
	bulletMark = regexp.MustCompile(`^[*•·▪◦o-]$`)
	bulletItem = regexp.MustCompile(`^[*•·▪◦-]\s+`)
	sentFrom   = regexp.MustCompile(`(?i)^(sent from my \w+|get outlook for \w+)$`)
)

// softWrap is how long a line a mail client wraps with a trailing space.
const softWrap = 60

// tidyLines drops separators, rejoins soft-wrapped lines, turns bullets into
// "- " items and collapses blank runs.
func tidyLines(lines []string) string {
	var out []string
	bullet := false
	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		for len(raw) >= softWrap && strings.HasSuffix(raw, " ") && i+1 < len(lines) && strings.TrimSpace(lines[i+1]) != "" {
			i++
			raw = strings.TrimRight(raw, " ") + " " + strings.TrimSpace(lines[i])
		}
		line := strings.Join(strings.Fields(raw), " ")
		switch {
		case separator.MatchString(line) || sentFrom.MatchString(line):
			continue
		case bulletMark.MatchString(line):
			bullet = true
			continue
		case line == "":
			if !bullet && len(out) > 0 && out[len(out)-1] != "" {
				out = append(out, "")
			}
			continue
		case bullet:
			line = "- " + line
		case bulletItem.MatchString(line):
			line = "- " + bulletItem.ReplaceAllString(line, "")
		}
		bullet = false
		if strings.HasPrefix(line, "- ") && len(out) > 1 && out[len(out)-1] == "" && strings.HasPrefix(out[len(out)-2], "- ") {
			out = out[:len(out)-1]
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
