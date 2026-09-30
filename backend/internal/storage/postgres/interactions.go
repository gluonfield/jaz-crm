package postgres

import (
	"context"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	intdb "github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/generated/interactions"
	"github.com/jackc/pgx/v5"
)

func toHandle(r intdb.Handle) storage.Handle                { return storage.Handle(r) }
func toInteraction(r intdb.Interaction) storage.Interaction { return storage.Interaction(r) }
func toPart(r intdb.InteractionPartsRow) storage.Part       { return storage.Part(r) }
func toLink(r intdb.InteractionLinksRow) storage.Link       { return storage.Link(r) }
func toActivity(r intdb.RecordActivityRow) storage.Activity { return storage.Activity(r) }
func toHandleRecord(r intdb.HandlesOnRecordsRow) storage.HandleRecord {
	return storage.HandleRecord(r)
}
func toUnassessed(r intdb.UnassessedHandlesRow) storage.UnassessedHandle {
	return storage.UnassessedHandle(r)
}
func toUnfetched(r intdb.UnfetchedPartsRow) storage.UnfetchedPart { return storage.UnfetchedPart(r) }
func toSummary(r intdb.ListHandlesRow) storage.HandleSummary {
	return storage.HandleSummary{Handle: toHandle(r.Handle), Interactions: r.Interactions, LastSeen: r.LastSeen}
}
func toParticipant(r intdb.InteractionParticipantsRow) storage.Participant {
	return storage.Participant{InteractionID: r.InteractionID, Role: r.Role, Handle: toHandle(r.Handle)}
}

func (s *Store) UpsertHandle(ctx context.Context, h storage.NewHandle) (storage.Handle, error) {
	return one(toHandle)(s.in.UpsertHandle(ctx, intdb.UpsertHandleParams(h)))
}

func (s *Store) SkipPendingHandle(ctx context.Context, id, reason string) error {
	return mapError(s.in.SkipPendingHandle(ctx, intdb.SkipPendingHandleParams{ID: id, Reason: reason}))
}

func (s *Store) SetTriage(ctx context.Context, v storage.Verdict) error {
	return affected(s.in.SetTriage(ctx, intdb.SetTriageParams(v)))
}

func (s *Store) Handles(ctx context.Context, workspaceID string, ids []string) ([]storage.Handle, error) {
	return many(toHandle)(s.in.GetHandles(ctx, intdb.GetHandlesParams{WorkspaceID: workspaceID, IDs: ids}))
}

func (s *Store) HandlesByValue(ctx context.Context, workspaceID string, values []string) ([]storage.Handle, error) {
	return many(toHandle)(s.in.HandlesByValue(ctx, intdb.HandlesByValueParams{WorkspaceID: workspaceID, Values: values}))
}

func (s *Store) HandlesByDomain(ctx context.Context, workspaceID, domain string) ([]storage.Handle, error) {
	return many(toHandle)(s.in.HandlesByDomain(ctx, intdb.HandlesByDomainParams{WorkspaceID: workspaceID, Value: domain}))
}

func (s *Store) ListHandles(ctx context.Context, q storage.HandleQuery) ([]storage.HandleSummary, error) {
	return many(toSummary)(s.in.ListHandles(ctx, intdb.ListHandlesParams(q)))
}

func (s *Store) MarkInternal(ctx context.Context, workspaceID string, addresses, domains []string) error {
	return mapError(s.in.MarkInternal(ctx, intdb.MarkInternalParams{WorkspaceID: workspaceID, Addresses: addresses, Domains: domains}))
}

func (s *Store) EngagedHandles(ctx context.Context, workspaceID string, maxSize int32) ([]string, error) {
	ids, err := s.in.EngagedHandles(ctx, intdb.EngagedHandlesParams{WorkspaceID: workspaceID, MaxSize: maxSize})
	return ids, mapError(err)
}

func (s *Store) HandlesOnRecords(ctx context.Context, workspaceID string) ([]storage.HandleRecord, error) {
	return many(toHandleRecord)(s.in.HandlesOnRecords(ctx, workspaceID))
}

func (s *Store) KeptWithoutPerson(ctx context.Context, workspaceID string) ([]storage.Handle, error) {
	return many(toHandle)(s.in.KeptWithoutPerson(ctx, workspaceID))
}

func (s *Store) UnassessedHandles(ctx context.Context, workspaceID string, limit int32) ([]storage.UnassessedHandle, error) {
	return many(toUnassessed)(s.in.UnassessedHandles(ctx, intdb.UnassessedHandlesParams{WorkspaceID: workspaceID, Limit: limit}))
}

func (s *Store) EmailThreadByMessageIDs(ctx context.Context, workspaceID string, messageIDs []string) (string, error) {
	id, err := s.in.EmailInteractionByMessageIDs(ctx, intdb.EmailInteractionByMessageIDsParams{WorkspaceID: workspaceID, MessageIDs: messageIDs})
	return id, mapError(err)
}

func (s *Store) UpsertEmailThread(ctx context.Context, t storage.EmailThread) (string, error) {
	id, err := s.in.UpsertEmailThread(ctx, intdb.UpsertEmailThreadParams(t))
	return id, mapError(err)
}

func (s *Store) ExtendEmailThread(ctx context.Context, id string, at time.Time, title string) error {
	return mapError(s.in.ExtendEmailThread(ctx, intdb.ExtendEmailThreadParams{ID: id, At: at, Title: title}))
}

func (s *Store) UpsertInteraction(ctx context.Context, i storage.NewInteraction) (storage.Interaction, error) {
	return one(toInteraction)(s.in.UpsertInteraction(ctx, intdb.UpsertInteractionParams(i)))
}

func (s *Store) ClearParticipants(ctx context.Context, interactionID string) error {
	return mapError(s.in.ClearParticipants(ctx, interactionID))
}

func (s *Store) AddParticipant(ctx context.Context, interactionID, handleID, role string) error {
	return mapError(s.in.AddParticipant(ctx, intdb.AddParticipantParams{InteractionID: interactionID, HandleID: handleID, Role: role}))
}

func (s *Store) UpsertPart(ctx context.Context, p storage.NewPart) error {
	return mapError(s.in.UpsertPart(ctx, intdb.UpsertPartParams(p)))
}

func (s *Store) inTx(ctx context.Context, fn func(q *intdb.Queries) error) error {
	return mapError(pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(s.in.WithTx(tx))
	}))
}

func (s *Store) Relink(ctx context.Context, interactionIDs []string) error {
	return s.inTx(ctx, func(q *intdb.Queries) error {
		return relink(ctx, q, interactionIDs)
	})
}

func relink(ctx context.Context, q *intdb.Queries, ids []string) error {
	if err := q.DeleteSyncLinks(ctx, ids); err != nil {
		return err
	}
	if err := q.InsertSyncLinks(ctx, ids); err != nil {
		return err
	}
	return q.ClearUnlinkedContent(ctx, ids)
}

func (s *Store) InteractionsOfHandles(ctx context.Context, handleIDs []string) ([]string, error) {
	ids, err := s.in.InteractionsOfHandles(ctx, handleIDs)
	return ids, mapError(err)
}

func (s *Store) AddLink(ctx context.Context, interactionID, recordID, source string) error {
	return mapError(s.in.AddLink(ctx, intdb.AddLinkParams{InteractionID: interactionID, RecordID: recordID, Source: source}))
}

func (s *Store) DeleteLink(ctx context.Context, interactionID, recordID string) error {
	return s.inTx(ctx, func(q *intdb.Queries) error {
		if err := affected(q.DeleteLink(ctx, intdb.DeleteLinkParams{InteractionID: interactionID, RecordID: recordID})); err != nil {
			return err
		}
		return q.ClearUnlinkedContent(ctx, []string{interactionID})
	})
}

func (s *Store) SkipInteraction(ctx context.Context, workspaceID, id string) error {
	return s.inTx(ctx, func(q *intdb.Queries) error {
		if err := affected(q.SkipInteraction(ctx, intdb.SkipInteractionParams{WorkspaceID: workspaceID, ID: id})); err != nil {
			return err
		}
		if err := q.DeleteLinks(ctx, []string{id}); err != nil {
			return err
		}
		return q.ClearUnlinkedContent(ctx, []string{id})
	})
}

func (s *Store) UnfetchedParts(ctx context.Context, connectionID string, limit int32) ([]storage.UnfetchedPart, error) {
	return many(toUnfetched)(s.in.UnfetchedParts(ctx, intdb.UnfetchedPartsParams{ConnectionID: &connectionID, Limit: limit}))
}

func (s *Store) SetPartContent(ctx context.Context, id int64, content string) error {
	return mapError(s.in.SetPartContent(ctx, intdb.SetPartContentParams{ID: id, Content: &content}))
}

func (s *Store) Interactions(ctx context.Context, workspaceID string, ids []string) ([]storage.Interaction, error) {
	return many(toInteraction)(s.in.GetInteractions(ctx, intdb.GetInteractionsParams{WorkspaceID: workspaceID, IDs: ids}))
}

func (s *Store) Timeline(ctx context.Context, q storage.TimelineQuery) ([]storage.Interaction, error) {
	return many(toInteraction)(s.in.Timeline(ctx, intdb.TimelineParams(q)))
}

func (s *Store) SearchInteractions(ctx context.Context, workspaceID, query string, limit int32) ([]storage.Interaction, error) {
	return many(toInteraction)(s.in.SearchInteractions(ctx, intdb.SearchInteractionsParams{WorkspaceID: workspaceID, Query: query, Limit: limit}))
}

func (s *Store) Participants(ctx context.Context, interactionIDs []string) ([]storage.Participant, error) {
	return many(toParticipant)(s.in.InteractionParticipants(ctx, interactionIDs))
}

func (s *Store) Parts(ctx context.Context, interactionIDs []string) ([]storage.Part, error) {
	return many(toPart)(s.in.InteractionParts(ctx, interactionIDs))
}

func (s *Store) Links(ctx context.Context, interactionIDs []string) ([]storage.Link, error) {
	return many(toLink)(s.in.InteractionLinks(ctx, interactionIDs))
}

func (s *Store) RecordActivity(ctx context.Context, workspaceID string, recordIDs []string) ([]storage.Activity, error) {
	return many(toActivity)(s.in.RecordActivity(ctx, intdb.RecordActivityParams{WorkspaceID: workspaceID, RecordIDs: recordIDs}))
}

func (s *Store) SetDomainRule(ctx context.Context, workspaceID, domain, triage, reason string) error {
	return mapError(s.in.SetDomainRule(ctx, intdb.SetDomainRuleParams{WorkspaceID: workspaceID, Domain: domain, Triage: triage, Reason: reason}))
}

func (s *Store) DomainRules(ctx context.Context, workspaceID string) ([]storage.DomainRule, error) {
	return many(func(r intdb.DomainRule) storage.DomainRule { return storage.DomainRule(r) })(s.in.DomainRules(ctx, workspaceID))
}

func (s *Store) InteractionByExternalID(ctx context.Context, workspaceID, source, externalID string) (string, error) {
	id, err := s.in.InteractionByExternalID(ctx, intdb.InteractionByExternalIDParams{WorkspaceID: workspaceID, Source: source, ExternalID: externalID})
	return id, mapError(err)
}

func (s *Store) DueMeetings(ctx context.Context, connectionID string) ([]storage.Interaction, error) {
	return many(toInteraction)(s.in.DueMeetings(ctx, &connectionID))
}

func (s *Store) MarkTranscriptChecked(ctx context.Context, interactionID string) error {
	return mapError(s.in.MarkTranscriptChecked(ctx, interactionID))
}
