package records_test

import (
	"slices"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/records"
)

func TestCombinedFiltersUseCurrentValues(t *testing.T) {
	svc, a, b := setup(t)
	for _, tag := range []string{"Founder", "Manufacturing", "Archived"} {
		if _, err := svc.AddOption(ctx, a, "people", "tags", tag); err != nil {
			t.Fatal(err)
		}
	}
	company, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: "companies", Set: set("name", "Acme", "domains", "acme.test")})
	ada, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: "people", Set: set("name", "Ada Stone", "company", company.ID, "tags", "Founder", "tags", "Manufacturing", "tags", "Archived", "job_title", "CEO 100%", "email_addresses", "ada@acme.test")})
	upsert(t, svc, a, records.SourceUser, records.Write{Object: "people", RecordID: ada.ID, Remove: map[string][]string{"tags": {"Archived"}}})
	bob, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: "people", Set: set("name", "Bob Stone", "tags", "Founder", "job_title", "Engineer")})
	ann, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: "people", Set: set("name", "Ann")})
	upsert(t, svc, b, records.SourceUser, records.Write{Object: "people", Set: set("name", "Eve")})

	for _, test := range []struct {
		name    string
		filters []records.Filter
		want    []string
	}{
		{"two tags and company", []records.Filter{{Attribute: "tags", Operator: "is", Value: "founder"}, {Attribute: "tags", Operator: "is", Value: "Manufacturing"}, {Attribute: "company", Operator: "is", Value: "ACME.test"}}, []string{ada.ID}},
		{"exclude a tag across all values", []records.Filter{{Attribute: "tags", Operator: "is_not", Value: "Manufacturing"}}, []string{bob.ID, ann.ID}},
		{"historical tags do not match", []records.Filter{{Attribute: "tags", Operator: "is", Value: "Archived"}}, nil},
		{"text contains", []records.Filter{{Attribute: "job_title", Operator: "contains", Value: "ceo"}}, []string{ada.ID}},
		{"literal wildcard", []records.Filter{{Attribute: "job_title", Operator: "contains", Value: "%"}}, []string{ada.ID}},
		{"negative contains", []records.Filter{{Attribute: "tags", Operator: "not_contains", Value: "found"}}, []string{ann.ID}},
		{"empty tags", []records.Filter{{Attribute: "tags", Operator: "is_empty"}}, []string{ann.ID}},
		{"nonempty tags", []records.Filter{{Attribute: "tags", Operator: "is_not_empty"}}, []string{ada.ID, bob.ID}},
		{"empty company", []records.Filter{{Attribute: "company", Operator: "is_empty"}}, []string{bob.ID, ann.ID}},
		{"unknown company", []records.Filter{{Attribute: "company", Operator: "is", Value: "unknown.test"}}, nil},
		{"exclude unknown company", []records.Filter{{Attribute: "company", Operator: "is_not", Value: "unknown.test"}}, []string{ada.ID, bob.ID, ann.ID}},
	} {
		t.Run(test.name, func(t *testing.T) {
			found, _, err := svc.Search(ctx, a, records.Search{Object: "people", Filters: test.filters})
			if err != nil {
				t.Fatal(err)
			}
			ids := []string{}
			for _, record := range found {
				ids = append(ids, record.ID)
			}
			slices.Sort(ids)
			slices.Sort(test.want)
			if !slices.Equal(ids, test.want) {
				t.Fatalf("got %v, want %v", ids, test.want)
			}
		})
	}
	for _, filter := range []records.Filter{
		{Attribute: "missing", Operator: "is_empty"},
		{Attribute: "tags", Operator: "any", Value: "Founder"},
		{Attribute: "name", Operator: "contains", Value: " "},
		{Attribute: "company", Operator: "contains", Value: "Acme"},
	} {
		if _, _, err := svc.Search(ctx, a, records.Search{Object: "people", Filters: []records.Filter{filter}}); err == nil {
			t.Errorf("invalid condition accepted: %+v", filter)
		}
	}
	found, _, err := svc.Search(ctx, a, records.Search{Object: "people", Query: "stone", Where: map[string]string{"tags": "Founder"}, Filters: []records.Filter{{Attribute: "company", Operator: "is_not_empty"}}})
	if err != nil || len(found) != 1 || found[0].ID != ada.ID {
		t.Fatalf("query, legacy where and conditions must combine: %v %v", found, err)
	}
}

func TestDateFilterComparisons(t *testing.T) {
	svc, a, _ := setup(t)
	early, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: "deals", Set: set("name", "Early", "next_follow_up_date", "2027-01-04")})
	same, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: "deals", Set: set("name", "Same", "next_follow_up_date", "2027-01-05")})
	late, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: "deals", Set: set("name", "Late", "next_follow_up_date", "2027-01-06")})
	upsert(t, svc, a, records.SourceUser, records.Write{Object: "deals", Set: set("name", "No date")})
	for _, test := range []struct {
		operator string
		want     []string
	}{
		{"before", []string{early.ID}},
		{"on_or_before", []string{early.ID, same.ID}},
		{"after", []string{late.ID}},
		{"on_or_after", []string{same.ID, late.ID}},
	} {
		t.Run(test.operator, func(t *testing.T) {
			found, _, err := svc.Search(ctx, a, records.Search{Object: "deals", Filters: []records.Filter{{Attribute: "next_follow_up_date", Operator: test.operator, Value: "2027-01-05"}}})
			if err != nil {
				t.Fatal(err)
			}
			var ids []string
			for _, record := range found {
				ids = append(ids, record.ID)
			}
			slices.Sort(ids)
			slices.Sort(test.want)
			if !slices.Equal(ids, test.want) {
				t.Fatalf("got %v, want %v", ids, test.want)
			}
		})
	}
	for _, filter := range []records.Filter{
		{Attribute: "name", Operator: "before", Value: "2027-01-05"},
		{Attribute: "value", Operator: "after", Value: "10"},
		{Attribute: "next_follow_up_date", Operator: "on_or_before", Value: "January"},
	} {
		if _, _, err := svc.Search(ctx, a, records.Search{Object: "deals", Filters: []records.Filter{filter}}); err == nil {
			t.Errorf("invalid date filter accepted: %+v", filter)
		}
	}
}

// A search pages through records in name order and counts every match.
func TestSearchPages(t *testing.T) {
	svc, a, _ := setup(t)
	for _, name := range []string{"Eve", "Ada", "Dan", "Cy", "Bo"} {
		upsert(t, svc, a, records.SourceUser, records.Write{Object: "people", Set: set("name", name)})
	}
	var names []string
	for offset := 0; offset < 6; offset += 2 {
		page, total, err := svc.Search(ctx, a, records.Search{Object: "people", Sort: "name", Offset: offset, Limit: 2})
		if err != nil || total != 5 && len(page) > 0 {
			t.Fatalf("page at %d: %d records of %d, %v", offset, len(page), total, err)
		}
		for _, r := range page {
			names = append(names, values(r, "name")[0])
		}
	}
	if !slices.Equal(names, []string{"Ada", "Bo", "Cy", "Dan", "Eve"}) {
		t.Fatalf("paged names: %v", names)
	}
}

// A name search finds the person named before newer records that only
// mention them.
func TestTextSearchRanksNameMatchesFirst(t *testing.T) {
	svc, a, _ := setup(t)
	jim, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: "people", Set: set("name", "Jim Mayer")})
	pat, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: "people", Set: set("name", "Pat Lee", "email_addresses", "pat@mayer.test")})
	var noted records.Record
	for _, name := range []string{"Roger Atkins", "Lisa Lang", "Laurie Harbour"} {
		noted, _ = upsert(t, svc, a, records.SourceUser, records.Write{Object: "people", Set: set("name", name, "context", "- Introduced by Jim Mayer")})
	}
	found, total, err := svc.Search(ctx, a, records.Search{Object: "people", Query: "mayer", Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, record := range found {
		ids = append(ids, record.ID)
	}
	if want := []string{jim.ID, pat.ID, noted.ID}; total != 5 || !slices.Equal(ids, want) {
		t.Fatalf("got %v of %d, want %v", ids, total, want)
	}
}
