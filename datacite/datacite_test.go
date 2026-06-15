package datacite

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestClient(baseURL string) *Client {
	cfg := DefaultConfig()
	cfg.BaseURL = baseURL
	cfg.Rate = 0 // no pacing in tests
	return NewClient(cfg)
}

func TestSearchDOIs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dois" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = w.Write([]byte(`{
			"data": [{
				"id": "10.test/abc",
				"attributes": {
					"doi": "10.test/abc",
					"titles": [{"title": "Test Paper"}],
					"creators": [{"name": "Smith J"}, {"name": "Doe A"}],
					"publicationYear": 2023,
					"publisher": "Test Pub",
					"types": {"resourceTypeGeneral": "JournalArticle"},
					"descriptions": [{"description": "A test paper."}],
					"url": "https://doi.org/10.test/abc"
				}
			}],
			"meta": {"total": 100, "totalPages": 4, "page": 1}
		}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	dois, total, err := c.SearchDOIs(context.Background(), "test", 1, 25)
	if err != nil {
		t.Fatalf("SearchDOIs: %v", err)
	}
	if total != 100 {
		t.Errorf("total = %d, want 100", total)
	}
	if len(dois) != 1 {
		t.Fatalf("len(dois) = %d, want 1", len(dois))
	}
	d := dois[0]
	if d.DOI != "10.test/abc" {
		t.Errorf("DOI = %q, want 10.test/abc", d.DOI)
	}
	if d.Title != "Test Paper" {
		t.Errorf("Title = %q, want Test Paper", d.Title)
	}
	if d.Creators != "Smith J, Doe A" {
		t.Errorf("Creators = %q, want 'Smith J, Doe A'", d.Creators)
	}
	if d.Year != "2023" {
		t.Errorf("Year = %q, want 2023", d.Year)
	}
	if d.Publisher != "Test Pub" {
		t.Errorf("Publisher = %q, want Test Pub", d.Publisher)
	}
	if d.Type != "JournalArticle" {
		t.Errorf("Type = %q, want JournalArticle", d.Type)
	}
	if d.Description != "A test paper." {
		t.Errorf("Description = %q, want 'A test paper.'", d.Description)
	}
}

func TestGetDOI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dois/10.1234%2Fexample" && r.URL.Path != "/dois/10.1234/example" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = w.Write([]byte(`{
			"data": {
				"id": "10.1234/example",
				"attributes": {
					"doi": "10.1234/example",
					"titles": [{"title": "Example Dataset"}],
					"creators": [{"name": "Jones B"}],
					"publicationYear": 2022,
					"publisher": "Example Publisher",
					"types": {"resourceTypeGeneral": "Dataset"},
					"descriptions": [{"description": "An example dataset."}],
					"url": "https://doi.org/10.1234/example"
				}
			}
		}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	d, err := c.GetDOI(context.Background(), "10.1234/example")
	if err != nil {
		t.Fatalf("GetDOI: %v", err)
	}
	if d.DOI != "10.1234/example" {
		t.Errorf("DOI = %q, want 10.1234/example", d.DOI)
	}
	if d.Title != "Example Dataset" {
		t.Errorf("Title = %q, want Example Dataset", d.Title)
	}
	if d.Type != "Dataset" {
		t.Errorf("Type = %q, want Dataset", d.Type)
	}
}

func TestListFunders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/funders" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = w.Write([]byte(`{
			"data": [{
				"id": "funder1",
				"attributes": {
					"name": "Test Funder",
					"region": "Americas",
					"country": {"name": "US"}
				}
			}],
			"meta": {"total": 50, "totalPages": 2, "page": 1}
		}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	funders, err := c.ListFunders(context.Background(), 1, 25)
	if err != nil {
		t.Fatalf("ListFunders: %v", err)
	}
	if len(funders) != 1 {
		t.Fatalf("len(funders) = %d, want 1", len(funders))
	}
	f := funders[0]
	if f.ID != "funder1" {
		t.Errorf("ID = %q, want funder1", f.ID)
	}
	if f.Name != "Test Funder" {
		t.Errorf("Name = %q, want Test Funder", f.Name)
	}
	if f.Region != "Americas" {
		t.Errorf("Region = %q, want Americas", f.Region)
	}
	if f.Country != "US" {
		t.Errorf("Country = %q, want US", f.Country)
	}
}

func TestListMembers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/members" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = w.Write([]byte(`{
			"data": [{
				"id": "member1",
				"attributes": {
					"name": "Test University",
					"region": "EMEA",
					"country": "DE",
					"memberType": "full_member"
				}
			}],
			"meta": {"total": 200, "totalPages": 8, "page": 1}
		}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	members, err := c.ListMembers(context.Background(), 1, 25)
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("len(members) = %d, want 1", len(members))
	}
	m := members[0]
	if m.ID != "member1" {
		t.Errorf("ID = %q, want member1", m.ID)
	}
	if m.Name != "Test University" {
		t.Errorf("Name = %q, want Test University", m.Name)
	}
	if m.MemberType != "full_member" {
		t.Errorf("MemberType = %q, want full_member", m.MemberType)
	}
}

func TestRetryOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		// Return a valid search response on the third hit
		resp := map[string]any{
			"data": []map[string]any{{
				"id": "10.retry/test",
				"attributes": map[string]any{
					"doi":    "10.retry/test",
					"titles": []map[string]any{{"title": "Retry Test"}},
				},
			}},
			"meta": map[string]any{"total": 1, "totalPages": 1, "page": 1},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 5
	c := NewClient(cfg)

	start := time.Now()
	dois, _, err := c.SearchDOIs(context.Background(), "test", 1, 10)
	if err != nil {
		t.Fatalf("SearchDOIs after retries: %v", err)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if len(dois) != 1 {
		t.Errorf("len(dois) = %d, want 1", len(dois))
	}
	// Should have backed off at least once (500ms)
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}
