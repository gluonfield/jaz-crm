package google

import (
	"context"
	"net/url"
	"slices"
	"strings"
)

// photo is a People API picture; Default marks Google's generated letter.
type photo struct {
	URL     string
	Default bool
}

// ContactPhotos reads a page of the account's other contacts, the people it
// has written to, mapping each address to its profile picture. Google's
// generated letter pictures are left out.
func (c *Client) ContactPhotos(ctx context.Context, pageToken string) (photos map[string]string, next string, err error) {
	var raw struct {
		OtherContacts []struct {
			EmailAddresses []struct{ Value string }
			Photos         []photo
		}
		NextPageToken string
	}
	q := url.Values{"readMask": {"emailAddresses,photos"}, "pageSize": {"1000"}, "pageToken": {pageToken}}
	if err := c.get(ctx, c.endpoints.People+"/v1/otherContacts", q, &raw); err != nil {
		return nil, "", err
	}
	photos = map[string]string{}
	for _, contact := range raw.OtherContacts {
		i := slices.IndexFunc(contact.Photos, func(p photo) bool { return !p.Default })
		if i < 0 {
			continue
		}
		for _, e := range contact.EmailAddresses {
			photos[strings.ToLower(e.Value)] = contact.Photos[i].URL
		}
	}
	return photos, raw.NextPageToken, nil
}
