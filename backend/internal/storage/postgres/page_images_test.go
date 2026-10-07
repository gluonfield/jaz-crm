package postgres_test

import (
	"errors"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	"github.com/lithammer/shortuuid/v4"
)

func TestPageImageRollsBackWithFailedRecordWrite(t *testing.T) {
	ctx := t.Context()
	store := postgrestest.New(t)
	owner, err := workspaces.NewService(store, workspaces.Config{}).Provision(ctx, "owner@jaz.test")
	if err != nil {
		t.Fatal(err)
	}
	svc := records.NewService(store)
	actor := auth.Actor{UserID: owner.ID, WorkspaceID: owner.WorkspaceID}
	page, _, err := svc.Upsert(ctx, actor, records.SourceUser, records.Write{Object: records.Pages, Set: map[string][]string{"name": {"Design"}}})
	if err != nil {
		t.Fatal(err)
	}
	held, err := store.Records(ctx, actor.WorkspaceID, []string{page.ID})
	if err != nil {
		t.Fatal(err)
	}
	imageID := shortuuid.New()
	text := "image:" + imageID
	_, err = store.WriteRecord(ctx, actor.WorkspaceID, held[0].ObjectID, page.ID, func([]storage.RecordValue) (storage.ValueChanges, error) {
		return storage.ValueChanges{
			Images: []storage.PageImage{{ID: imageID, PNG: []byte("thumbnail")}},
			Insert: []storage.NewRecordValue{{AttributeID: "missing", Text: &text, Source: "user"}},
		}, nil
	})
	if err == nil {
		t.Fatal("a record value with a missing attribute was inserted")
	}
	if _, err := store.PageImage(ctx, imageID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("image outlived the failed transaction: %v", err)
	}
}
