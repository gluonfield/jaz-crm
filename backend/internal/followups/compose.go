package followups

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/google"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

type Sender struct {
	From      string   `json:"from"`
	Subject   string   `json:"subject"`
	Reply     bool     `json:"reply"`
	Signature string   `json:"signature,omitempty"`
	To        []string `json:"to"`
	Cc        []string `json:"cc"`
}

func (s *Service) Sender(ctx context.Context, actor auth.Actor, id string) (Sender, error) {
	f, err := s.crm.Get(ctx, actor, id)
	if err != nil {
		return Sender{}, err
	}
	if f.Object != records.FollowUps {
		return Sender{}, errs.Invalidf("%s is not a follow-up", id)
	}
	out, err := s.compose(ctx, actor, f)
	return Sender{From: out.account, Subject: out.message.Subject, Reply: out.latest != nil, Signature: out.message.Signature.HTML, To: out.message.To, Cc: out.message.Cc}, err
}

func (s *Service) compose(ctx context.Context, actor auth.Actor, f records.Record) (outgoing, error) {
	last, sender, err := s.sender(ctx, actor, f)
	if err != nil {
		return outgoing{}, err
	}
	message := google.Outgoing{Body: value(f, "draft"), Subject: value(f, "subject"), To: values(f, "to"), Cc: values(f, "cc")}
	mailbox, err := s.conns.Google(ctx, sender)
	if err != nil {
		return outgoing{}, err
	}
	identity, err := mailbox.Identity(ctx, sender.Account)
	if err != nil {
		return outgoing{}, err
	}
	message.From = google.Address{Name: identity.Name, Email: sender.Account}
	message.ReplyTo = identity.ReplyTo
	message.Signature = identity.Signature
	if last.ProviderID != nil {
		original, holder, err := s.original(ctx, last)
		if err != nil {
			return outgoing{}, err
		}
		message.To, message.Cc, err = s.recipients(ctx, f, original, sender, holder)
		if err != nil {
			return outgoing{}, err
		}
		subject := original.Subject
		if !strings.HasPrefix(strings.ToLower(subject), "re:") {
			subject = "Re: " + subject
		}
		if message.Subject == "" {
			message.Subject = subject
		}
		message.InReplyTo = original.MessageID
		message.References = slices.Clone(original.References)
		if original.MessageID != "" {
			message.References = append(message.References, original.MessageID)
		}
		// Gmail requires matching subjects when a thread ID is supplied.
		if message.Subject == subject {
			message.ThreadID = original.ThreadID
			if sender.ID != holder.ID && original.MessageID != "" {
				message.ThreadID, err = mailbox.ThreadOf(ctx, original.MessageID)
				if err != nil && !errors.Is(err, google.ErrNotFound) {
					return outgoing{}, err
				}
			}
		}
	} else if len(message.To)+len(message.Cc) == 0 {
		for _, id := range refs(f, "person") {
			person, err := s.crm.Get(ctx, actor, id)
			if err != nil {
				return outgoing{}, err
			}
			message.To = append(message.To, values(person, "email_addresses")...)
		}
	}
	return outgoing{mailbox: mailbox, account: sender.Account, message: message, latest: last.At}, nil
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
