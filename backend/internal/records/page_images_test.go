package records_test

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/records"
)

func TestPageImagesRejectInvalidAndOtherWorkspaceFiles(t *testing.T) {
	svc, a, b := setup(t)
	encode := func(width int) string {
		t.Helper()
		var data bytes.Buffer
		if err := png.Encode(&data, image.NewNRGBA(image.Rect(0, 0, width, 1))); err != nil {
			t.Fatal(err)
		}
		return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data.Bytes())
	}
	page, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: records.Pages, Set: set("name", "Design", "icon", encode(128))})
	icon := values(page, "icon")[0]
	for _, value := range []string{"data:image/svg+xml;base64,PHN2Zy8+", "data:image/png;base64,broken", "data:image/png;base64," + strings.Repeat("A", 100_001), encode(129), "image:missing"} {
		if _, _, err := svc.Upsert(ctx, a, records.SourceUser, records.Write{Object: records.Pages, RecordID: page.ID, Set: set("icon", value)}); err == nil {
			t.Errorf("invalid image accepted: %.80s", value)
		}
	}
	if _, _, err := svc.Upsert(ctx, b, records.SourceUser, records.Write{Object: records.Pages, Set: set("name", "Other workspace", "icon", icon)}); err == nil {
		t.Fatal("an image from another workspace was reused")
	}
	got, err := svc.Get(ctx, a, page.ID)
	if err != nil || values(got, "icon")[0] != icon {
		t.Fatalf("rejected uploads changed the saved icon: %+v %v", got, err)
	}
}
