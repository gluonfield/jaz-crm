package records_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
)

type countedRelations struct {
	storage.RecordStore
	calls   int
	parents int
}

func (s *countedRelations) RelatedRecords(ctx context.Context, workspaceID, attributeID string, ids []string, limit int32) ([]storage.RelatedRecord, error) {
	s.calls++
	s.parents += len(ids)
	return s.RecordStore.RelatedRecords(ctx, workspaceID, attributeID, ids, limit)
}

func TestRelatedRecordsBatchTheSelectedPage(t *testing.T) {
	store := postgrestest.New(t)
	user, err := workspaces.NewService(store, workspaces.Config{}).Provision(ctx, "batch@jaz.test")
	if err != nil {
		t.Fatal(err)
	}
	counted := &countedRelations{RecordStore: store}
	svc := records.NewService(counted)
	actor := auth.Actor{UserID: user.ID, WorkspaceID: user.WorkspaceID}
	for i := range 25 {
		company, _ := upsert(t, svc, actor, records.SourceUser, records.Write{Object: "companies", Set: set("name", fmt.Sprintf("Company %d", i))})
		upsert(t, svc, actor, records.SourceUser, records.Write{Object: "people", Set: set("name", "Person", "company", company.ID)})
	}
	page, _, err := svc.Search(ctx, actor, records.Search{Object: "companies", Limit: 10, Include: []records.Relation{{Object: "people", Attribute: "company", Limit: 4}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 10 || counted.calls != 1 || counted.parents != 10 {
		t.Fatalf("a page of 10 companies required %d relationship queries covering %d parents; returned %d", counted.calls, counted.parents, len(page))
	}
}
