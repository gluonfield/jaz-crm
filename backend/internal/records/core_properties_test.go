package records_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/records"
)

func TestCorePropertiesWorkWithoutCustomSchema(t *testing.T) {
	svc, actor, _ := setup(t)
	for attribute, option := range map[string]string{"industry": "Manufacturing", "hq_country": "United Kingdom"} {
		if _, err := svc.AddOption(t.Context(), actor, "companies", attribute, option); err != nil {
			t.Fatal(err)
		}
	}
	companyValues := set("name", "Acme", "website", "https://acme.test", "links", "https://linkedin.com/company/acme",
		"industry", "Manufacturing", "hq_city", "Cambridge", "hq_state", "Cambridgeshire", "hq_country", "United Kingdom",
		"employee_count", "42", "owner", "a@jaz.test")
	company, _ := upsert(t, svc, actor, records.SourceUser, records.Write{Object: "companies", Set: companyValues})
	personValues := set("name", "Ada", "company", company.ID, "links", "https://linkedin.com/in/ada",
		"owner", "a@jaz.test")
	person, _ := upsert(t, svc, actor, records.SourceUser, records.Write{Object: "people", Set: personValues})
	dealValues := set("name", "Factory project", "company", company.ID, "people", person.ID, "owner", "a@jaz.test",
		"value", "1200", "expected_close_date", "2026-11-30")
	deal, _ := upsert(t, svc, actor, records.SourceUser, records.Write{Object: "deals", Set: dealValues})
	for _, record := range []struct {
		record records.Record
		want   map[string][]string
	}{{company, companyValues}, {person, personValues}, {deal, dealValues}} {
		got, err := svc.Get(t.Context(), actor, record.record.ID)
		if err != nil {
			t.Fatal(err)
		}
		for attribute, want := range record.want {
			if !slices.Equal(values(got, attribute), want) {
				t.Fatalf("%s.%s round trip: got %v want %v", got.Object, attribute, values(got, attribute), want)
			}
			for _, action := range []string{"archive", "delete"} {
				if err := svc.EditAttribute(t.Context(), actor, got.Object, attribute, action, ""); err == nil || !strings.Contains(err.Error(), "built in") {
					t.Fatalf("%s allowed %s.%s removal: %v", action, got.Object, attribute, err)
				}
			}
		}
	}
	if _, _, err := svc.Upsert(t.Context(), actor, records.SourceUser, records.Write{Object: "people", RecordID: person.ID, Set: set("links", "javascript://example.test/run")}); err == nil {
		t.Fatal("a built-in profile accepted an executable URL")
	}
	if _, _, err := svc.Upsert(t.Context(), actor, records.SourceUser, records.Write{Object: "people", RecordID: person.ID, Set: set("owner", "outside@jaz.test")}); err == nil {
		t.Fatal("a built-in owner accepted someone outside the workspace")
	}
	for _, value := range []string{"NaN", "Inf", "-Inf"} {
		if _, _, err := svc.Upsert(t.Context(), actor, records.SourceUser, records.Write{Object: "deals", RecordID: deal.ID, Set: set("value", value)}); err == nil {
			t.Fatalf("a deal accepted a non-finite amount: %s", value)
		}
	}
}
