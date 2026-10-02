package followups

import (
	"context"
	"slices"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/google"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
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
	original, holder, err := s.original(ctx, last)
	if err != nil {
		return Sender{}, err
	}
	to, cc, err := s.recipients(ctx, f, original, sender, holder)
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

func (s *Service) recipients(ctx context.Context, f records.Record, last google.Message, sender, holder storage.Connection) ([]string, []string, error) {
	to, cc := replyAll(last, slices.Concat([]string{sender.Account}, sender.Aliases))
	savedTo, savedCc := values(f, "to"), values(f, "cc")
	if len(savedTo)+len(savedCc) == 0 || sameSet(savedTo, to) && sameSet(savedCc, cc) {
		return to, cc, nil
	}
	holderTo, holderCc := replyAll(last, slices.Concat([]string{holder.Account}, holder.Aliases))
	if sameSet(savedTo, holderTo) && sameSet(savedCc, holderCc) {
		return to, cc, nil
	}
	// Earlier automatic drafts excluded every workspace address. Repair only
	// lists matching those defaults so custom recipients stay as reviewed.
	own, err := s.addresses.InternalAddresses(ctx, sender.WorkspaceID)
	if err != nil {
		return nil, nil, err
	}
	oldTo, oldCc := replyAll(last, own)
	if sameSet(savedTo, oldTo) && sameSet(savedCc, oldCc) {
		return to, cc, nil
	}
	return savedTo, savedCc, nil
}

func replyAll(last google.Message, own []string) ([]string, []string) {
	answer := last.ReplyTo
	if len(answer) == 0 {
		answer = []google.Address{last.From}
	}
	to, cc := []string{}, []string{}
	for i, field := range [][]google.Address{slices.Concat(answer, last.To), last.Cc} {
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
	return to, cc
}
