package google

import (
	"cmp"
	"context"
	"fmt"
	"net/url"
	"time"
)

type Conference struct {
	Name  string
	Start time.Time `json:"startTime"`
	End   time.Time `json:"endTime"`
}

type Transcript struct {
	Name, State string
}

type Entry struct {
	Name, Participant, Text string
	Start                   time.Time `json:"startTime"`
	End                     time.Time `json:"endTime"`
}

type Participant struct {
	Name, DisplayName string
}

type meetPage struct {
	ConferenceRecords []Conference
	Transcripts       []Transcript
	TranscriptEntries []Entry
	Participants      []struct {
		Name                                   string
		SignedinUser, AnonymousUser, PhoneUser struct{ DisplayName string }
	}
	NextPageToken string
}

func (c *Client) Conferences(ctx context.Context, meetingCode string) ([]Conference, error) {
	page, err := c.meet(ctx, "conferenceRecords", url.Values{"filter": {fmt.Sprintf("space.meeting_code = %q", meetingCode)}})
	return page.ConferenceRecords, err
}

func (c *Client) Transcripts(ctx context.Context, conference string) ([]Transcript, error) {
	page, err := c.meet(ctx, conference+"/transcripts", url.Values{})
	return page.Transcripts, err
}

func (c *Client) Entries(ctx context.Context, transcript string) ([]Entry, error) {
	page, err := c.meet(ctx, transcript+"/entries", url.Values{"pageSize": {"100"}})
	return page.TranscriptEntries, err
}

func (c *Client) Participants(ctx context.Context, conference string) ([]Participant, error) {
	page, err := c.meet(ctx, conference+"/participants", url.Values{})
	var out []Participant
	for _, p := range page.Participants {
		out = append(out, Participant{Name: p.Name, DisplayName: cmp.Or(p.SignedinUser.DisplayName, p.AnonymousUser.DisplayName, p.PhoneUser.DisplayName)})
	}
	return out, err
}

// meet follows nextPageToken and returns every page merged; on error it returns the zero page.
func (c *Client) meet(ctx context.Context, path string, query url.Values) (meetPage, error) {
	var all meetPage
	for {
		var page meetPage
		if err := c.get(ctx, c.endpoints.Meet+"/v2/"+path, query, &page); err != nil {
			return meetPage{}, err
		}
		all.ConferenceRecords = append(all.ConferenceRecords, page.ConferenceRecords...)
		all.Transcripts = append(all.Transcripts, page.Transcripts...)
		all.TranscriptEntries = append(all.TranscriptEntries, page.TranscriptEntries...)
		all.Participants = append(all.Participants, page.Participants...)
		if page.NextPageToken == "" {
			return all, nil
		}
		query.Set("pageToken", page.NextPageToken)
	}
}
