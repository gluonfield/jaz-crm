package postgres_test

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/mcpapi"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	"github.com/lithammer/shortuuid/v4"
)

func TestPreviouslyUploadedPageImagesRemainReadable(t *testing.T) {
	ctx := t.Context()
	db, dsn := legacy(t, 44)
	store, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	people := workspaces.NewService(store, workspaces.Config{})
	owner, err := people.Provision(ctx, "owner@jaz.test")
	if err != nil {
		t.Fatal(err)
	}
	actor := auth.Actor{UserID: owner.ID, WorkspaceID: owner.WorkspaceID}
	svc := records.NewService(store)
	page, _, err := svc.Upsert(ctx, actor, records.SourceUser, records.Write{Object: records.Pages, Set: map[string][]string{"name": {"Design"}}})
	if err != nil {
		t.Fatal(err)
	}
	id := shortuuid.New()
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewNRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO page_images(id,record_id,png) VALUES($1,$2,$3)", id, page.ID, data.Bytes()); err != nil {
		t.Fatal(err)
	}
	icon := "image:" + id
	if _, _, err := svc.Upsert(ctx, actor, records.SourceUser, records.Write{Object: records.Pages, RecordID: page.ID, Set: map[string][]string{"icon": {icon}}}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.PageImage(ctx, id)
	if err != nil || !bytes.Equal(got, data.Bytes()) {
		t.Fatalf("existing image changed: %v", err)
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM page_images").Scan(&count); err != nil || count != 1 {
		t.Fatalf("saving a legacy image created a file: %d %v", count, err)
	}
	keys := auth.NewService(store, auth.Config{PublicURL: "https://crm.test"})
	api := mcpapi.NewHandler(mcpapi.Services{Records: svc, Workspaces: people}, keys, log.New(io.Discard)).API
	link := "image:https://crm.test/page-icons/" + id
	for _, operation := range []string{`"values":{"icon":"` + link + `"}`, `"remove":{"icon":["` + link + `"]}`} {
		body := `{"object":"pages","record_id":"` + page.ID + `","expect":{"icon":"` + link + `"},` + operation + `}`
		req := httptest.NewRequest(http.MethodPost, "/api/tools/upsert_record", strings.NewReader(body)).WithContext(auth.WithActor(ctx, actor))
		req.SetPathValue("tool", "upsert_record")
		out := httptest.NewRecorder()
		api.ServeHTTP(out, req)
		if out.Code != http.StatusOK {
			t.Fatalf("legacy image URL cannot be saved or removed: %d %s", out.Code, out.Body.String())
		}
	}
	cleared, err := svc.Get(ctx, actor, page.ID)
	if err != nil || slices.ContainsFunc(cleared.Fields, func(f records.Field) bool { return f.Attribute == "icon" && len(f.Values) != 0 }) {
		t.Fatalf("legacy image URL removal did not clear its marker: %+v %v", cleared, err)
	}
	other, err := workspaces.NewService(store, workspaces.Config{}).Provision(ctx, "other@jaz.test")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Upsert(ctx, auth.Actor{UserID: other.ID, WorkspaceID: other.WorkspaceID}, records.SourceUser, records.Write{Object: records.Pages, Set: map[string][]string{"icon": {icon}}}); err == nil {
		t.Fatal("another workspace reused the stored image identifier")
	}
	if err := svc.Delete(ctx, actor, page.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PageImage(ctx, id); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("deleted page retains its image: %v", err)
	}
}
