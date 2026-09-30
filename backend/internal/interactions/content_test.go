package interactions

import "testing"

func TestReadable(t *testing.T) {
	for name, c := range map[string]struct{ kind, in, want string }{
		"teams invite": {"description", "Agenda:\r\n\r\n  *\r\nOverview of your area\r\n  *\r\nNext steps\r\n\r\n\r\n________________________________________________________________________________\r\n" +
			"Microsoft Teams meeting\r\nJoin: https://teams.microsoft.com/meet/123?p=abc\r\nMeeting ID: 123 456 789\r\nPasscode: xY12\r\n________________________________\r\n" +
			"Need help?<https://aka.ms/JoinTeamsMeeting> | System reference<https://teams.microsoft.com/l/meetup-join/19%3a>\r\nDial in by phone\r\n" +
			"+44 20 0000 0000,,111#<tel:+442000000000,,111#> United Kingdom, London\r\nFind a local number<https://dialin.example/x>\r\nPhone conference ID: 111#\r\n" +
			"For organisers: Meeting options<https://teams.microsoft.com/meetingOptions/?x=1> | Reset dial-in PIN<https://dialin.example/pin>\r\n________________________________________________________________________________\r\n",
			"Agenda:\n\n- Overview of your area\n- Next steps\n\nMicrosoft Teams meeting: https://teams.microsoft.com/meet/123?p=abc"},
		"reply chain": {"message", "Hi Ann,\n\nThanks, see our site<https://example.com/pricing> for prices.\nBest,\nBo [image: Logo]\n\nConfidentiality Notice: This message is for Ann only.\n\nOn Sep 25 2026, at 6:13 pm, Ann Lee <ann@example.com> wrote:\n> Hi Bo,\n>\n> Could you send prices?\n> Ann",
			"Hi Ann,\n\nThanks, see our site https://example.com/pricing for prices.\nBest,\nBo"},
		"wrapped wrote line": {"message", "Sounds good.\n\nOn Thu, Sep 24, 2026 at 9:00 AM Ann Lee\n<ann@example.com> wrote:\n> Shall we meet?", "Sounds good."},
		"outlook history": {"message", "Confirmed for Tuesday.\n\n________________________________\nFrom: Ann Lee <ann@example.com>\nSent: 24 September 2026 09:00\nTo: Bo\nSubject: Visit\n\nCan you come Tuesday?",
			"Confirmed for Tuesday."},
		"forwarded message": {"message", "FYI\n\n---------- Forwarded message ---------\nFrom: Ann Lee <ann@example.com>\nDate: Thu, Sep 24, 2026\nSubject: Visit\n\nCan you come Tuesday?",
			"FYI\n\n---------- Forwarded message ---------\nFrom: Ann Lee <ann@example.com>\nDate: Thu, Sep 24, 2026\nSubject: Visit\n\nCan you come Tuesday?"},
		"only quotes": {"message", "> an old reply\n> kept", "> an old reply\n> kept"},
		"google invite": {"message", "Intro call\nTuesday Jun 23, 2026 ⋅ 8:30am – 9am\n\nJoin with Google Meet\nhttps://meet.google.com/abc-defg-hij?hs=224\n\n\t\nJoin by phone\n(GB) +44 20 0000 0000\nPIN: 123456\n\nMore phone numbers\nhttps://tel.meet/abc-defg-hij?pin=1\n\n\nOrganizer\nAnn Lee\n\n~~//~~\nInvitation from Google Calendar: https://calendar.google.com/calendar/\n\nYou are receiving this email because you are subscribed to calendar  \nnotifications.",
			"Intro call\nTuesday Jun 23, 2026 ⋅ 8:30am – 9am\n\nGoogle Meet: https://meet.google.com/abc-defg-hij?hs=224\n\nOrganizer\nAnn Lee"},
		"soft wraps and bullets": {"message", "We would like to quote for the brackets on the drawing that you sent us  \nlast week, with these notes:\n• anodised\n• deburred\n\nSent from my iPhone",
			"We would like to quote for the brackets on the drawing that you sent us last week, with these notes:\n- anodised\n- deburred"},
		"note": {"note", "Call notes\n________\n> keep", "Call notes\n________\n> keep"},
	} {
		if got := readable(c.kind, c.in); got != c.want {
			t.Errorf("%s:\n%s\nwant:\n%s", name, got, c.want)
		}
	}
}
