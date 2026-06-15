package datacite

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

// These tests are offline: they exercise the URI driver's pure string functions
// and the host wiring (mint, body, resolve), which need no network. The client's
// HTTP behaviour is covered in datacite_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "datacite" {
		t.Errorf("Scheme = %q, want datacite", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "datacite" {
		t.Errorf("Identity.Binary = %q, want datacite", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		in  string
		typ string
		id  string
		ok  bool
	}{
		{"10.1234/example", "doi", "10.1234/example", true},
		{"10.5281/zenodo.1234567", "doi", "10.5281/zenodo.1234567", true},
		{"not-a-doi", "", "", false},
		{"", "", "", false},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if tc.ok {
			if err != nil || typ != tc.typ || id != tc.id {
				t.Errorf("Classify(%q) = (%q, %q, %v), want (%q, %q, nil)",
					tc.in, typ, id, err, tc.typ, tc.id)
			}
		} else {
			if err == nil {
				t.Errorf("Classify(%q) expected error, got (%q, %q, nil)", tc.in, typ, id)
			}
		}
	}
}

func TestLocate(t *testing.T) {
	got, err := Domain{}.Locate("doi", "10.1234/example")
	want := "https://doi.org/10.1234/example"
	if err != nil || got != want {
		t.Errorf("Locate = (%q, %v), want (%q, nil)", got, err, want)
	}

	_, err = Domain{}.Locate("page", "foo")
	if err == nil {
		t.Error("Locate(page) should return error for unknown type")
	}
}

// TestHostWiring mounts the driver in a kit Host and checks the round trip.
func TestHostWiring(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}

	d := &DOI{
		DOI:   "10.1234/example",
		Title: "Test Paper",
		URL:   "https://doi.org/10.1234/example",
	}
	u, err := h.Mint(d)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if want := "datacite://doi/10.1234/example"; u.String() != want {
		t.Errorf("Mint = %q, want %q", u.String(), want)
	}

	got, err := h.ResolveOn("datacite", "10.5678/test")
	if err != nil || got.String() != "datacite://doi/10.5678/test" {
		t.Errorf("ResolveOn = (%q, %v), want datacite://doi/10.5678/test", got.String(), err)
	}
}
