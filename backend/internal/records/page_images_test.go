package records_test

import (
	"strings"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/records"
)

func TestPageIconsRejectUploadsAndUnsafeLinks(t *testing.T) {
	svc, a, _ := setup(t)
	icon := "image:https://example.com/logo.png"
	page, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: records.Pages, Set: set("name", "Design", "icon", icon)})
	for _, value := range []string{
		"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAAB",
		"data:image/svg+xml;base64,PHN2Zy8+",
		"data:image/png;base64," + strings.Repeat("A", 100_001),
		"image:javascript:alert(1)",
		"image:data:image/png;base64,aGVsbG8=",
		"image:file:///private/logo.png",
		"image:https://user:password@example.com/logo.png",
		"image:https://",
		"image:missing",
	} {
		if _, _, err := svc.Upsert(ctx, a, records.SourceUser, records.Write{Object: records.Pages, RecordID: page.ID, Set: set("icon", value)}); err == nil {
			t.Errorf("invalid image accepted: %.80s", value)
		}
	}
	got, err := svc.Get(ctx, a, page.ID)
	if err != nil || values(got, "icon")[0] != icon {
		t.Fatalf("rejected inputs changed the saved icon: %+v %v", got, err)
	}
}
