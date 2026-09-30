package records_test

import (
	"context"
	"slices"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
)

type stageRaceStore struct {
	storage.RecordStore
	beforeWrite func()
}

func (s *stageRaceStore) WriteRecord(ctx context.Context, workspace, object, id string, mutate storage.RecordMutation) (string, error) {
	if s.beforeWrite != nil {
		f := s.beforeWrite
		s.beforeWrite = nil
		f()
	}
	return s.RecordStore.WriteRecord(ctx, workspace, object, id, mutate)
}

func TestWriteWithOutdatedStages(t *testing.T) {
	store := postgrestest.New(t)
	user, err := workspaces.NewService(store, workspaces.Config{}).Provision(ctx, "stages@jaz.test")
	if err != nil {
		t.Fatal(err)
	}
	a := auth.Actor{UserID: user.ID, WorkspaceID: user.WorkspaceID}
	svc := records.NewService(store)
	race := &stageRaceStore{RecordStore: store}
	writer := records.NewService(race)
	rename := func(from, to string) {
		if err := svc.EditStage(ctx, a, "deals", "stage", records.StageEdit{Action: "rename", Stage: from, Name: to}); err != nil {
			t.Fatal(err)
		}
	}
	race.beforeWrite = func() { rename("Lead", "Qualified") }
	if _, _, err := writer.Upsert(ctx, a, records.SourceUser, records.Write{Object: "deals", Set: set("name", "Stale", "stage", "Lead")}); err == nil {
		t.Fatal("stored a stage removed after validation")
	}
	if found, err := svc.Search(ctx, a, records.Search{Object: "deals"}); err != nil || len(found) != 0 {
		t.Fatalf("failed write left records: %v %v", found, err)
	}
	race.beforeWrite = func() { rename("Qualified", "Prospect") }
	deal, _ := upsert(t, writer, a, records.SourceUser, records.Write{Object: "deals", Set: set("name", "Default")})
	if got := values(deal, "stage"); !slices.Equal(got, []string{"Prospect"}) {
		t.Fatalf("default stage did not refresh: %v", got)
	}
}
