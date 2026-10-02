package followups

import (
	"context"
	"slices"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/google"
)

type Sender struct {
	From      string   `json:"from"`
	Signature string   `json:"signature,omitempty"`
	To        []string `json:"to"`
	Cc        []string `json:"cc"`
}

func (s *Service) Sender(ctx context.Context, actor auth.Actor, id string) (Sender, error) {
	f, err := s.crm.Get(ctx, actor, id)
	if err != nil {
		return Sender{}, err
	}
	last, sender, err := s.sender(ctx, actor, f)
	if err != nil {
		return Sender{}, err
	}
	original, _, err := s.original(ctx, last)
	if err != nil {
		return Sender{}, err
	}
	to, cc, err := s.recipients(ctx, actor.WorkspaceID, original)
	if err != nil {
		return Sender{}, err
	}
	mailbox, err := s.conns.Google(ctx, sender)
	if err != nil {
		return Sender{}, err
	}
	sig, err := mailbox.Signature(ctx, sender.Account)
	return Sender{From: sender.Account, Signature: sig.HTML, To: to, Cc: cc}, err
}

func (s *Service) recipients(ctx context.Context, workspaceID string, last google.Message) ([]string, []string, error) {
	own, err := s.addresses.InternalAddresses(ctx, workspaceID)
	if err != nil {
		return nil, nil, err
	}
	answer := last.ReplyTo
	if len(answer) == 0 {
		answer = []google.Address{last.From}
	}
	to, cc := []string{}, []string{}
	for i, field := range [][]google.Address{append(answer, last.To...), last.Cc} {
		for _, address := range field {
			if address.Email == "" || slices.Contains(own, address.Email) || slices.Contains(to, address.Email) || slices.Contains(cc, address.Email) {
				continue
			}
			if i == 0 {
				to = append(to, address.Email)
			} else {
				cc = append(cc, address.Email)
			}
		}
	}
	return to, cc, nil
}
