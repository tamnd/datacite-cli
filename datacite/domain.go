package datacite

import (
	"context"
	"fmt"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go registers the datacite kit Domain so a blank import in a multi-domain
// host (ant) enables the driver:
//
//	import _ "github.com/tamnd/datacite-cli/datacite"
//
// The Domain also builds the standalone datacite binary via cli.NewApp.
func init() { kit.Register(Domain{}) }

// Domain is the DataCite driver. It carries no state; the per-run client is
// built by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme and the identity the single-site binary inherits.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "datacite",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "datacite",
			Short:  "Browse the DataCite DOI registry",
			Long: `A command line for the DataCite DOI registry.

datacite reads public metadata from api.datacite.org over plain HTTPS, shapes
it into clean records, and prints output that pipes into the rest of your tools.
No API key, nothing to run alongside it.`,
			Site: Host,
			Repo: "https://github.com/tamnd/datacite-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{Name: "search", Group: "read", List: true,
		Summary: "Search DOIs by keyword",
		Args:    []kit.Arg{{Name: "query", Help: "search query"}}}, searchDOIs)

	kit.Handle(app, kit.OpMeta{Name: "doi", Group: "read", Single: true,
		Summary: "Fetch a single DOI record",
		Args:    []kit.Arg{{Name: "id", Help: "DOI identifier (e.g. 10.1234/example)"}}}, getDOI)

	kit.Handle(app, kit.OpMeta{Name: "funders", Group: "read", List: true,
		Summary: "List DataCite funders"}, listFunders)

	kit.Handle(app, kit.OpMeta{Name: "members", Group: "read", List: true,
		Summary: "List DataCite members"}, listMembers)
}

// newClient builds a Client from the resolved kit Config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClient(c), nil
}

// --- input structs ---

type searchInput struct {
	Query  string  `kit:"arg" help:"search query"`
	Page   int     `kit:"flag" help:"page number (default 1)"`
	Size   int     `kit:"flag" help:"results per page (default 25)"`
	Client *Client `kit:"inject"`
}

type doiInput struct {
	ID     string  `kit:"arg" help:"DOI identifier (e.g. 10.1234/example)"`
	Client *Client `kit:"inject"`
}

type fundersInput struct {
	Page   int     `kit:"flag" help:"page number (default 1)"`
	Size   int     `kit:"flag" help:"results per page (default 25)"`
	Client *Client `kit:"inject"`
}

type membersInput struct {
	Page   int     `kit:"flag" help:"page number (default 1)"`
	Size   int     `kit:"flag" help:"results per page (default 25)"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func searchDOIs(ctx context.Context, in searchInput, emit func(DOI) error) error {
	dois, _, err := in.Client.SearchDOIs(ctx, in.Query, in.Page, in.Size)
	if err != nil {
		return err
	}
	for _, d := range dois {
		if err := emit(d); err != nil {
			return err
		}
	}
	return nil
}

func getDOI(ctx context.Context, in doiInput, emit func(*DOI) error) error {
	d, err := in.Client.GetDOI(ctx, in.ID)
	if err != nil {
		return err
	}
	return emit(d)
}

func listFunders(ctx context.Context, in fundersInput, emit func(Funder) error) error {
	funders, err := in.Client.ListFunders(ctx, in.Page, in.Size)
	if err != nil {
		return err
	}
	for _, f := range funders {
		if err := emit(f); err != nil {
			return err
		}
	}
	return nil
}

func listMembers(ctx context.Context, in membersInput, emit func(Member) error) error {
	members, err := in.Client.ListMembers(ctx, in.Page, in.Size)
	if err != nil {
		return err
	}
	for _, m := range members {
		if err := emit(m); err != nil {
			return err
		}
	}
	return nil
}

// --- Resolver ---

// Classify turns any accepted input into the canonical (uriType, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	if input == "" {
		return "", "", errs.Usage("datacite: empty input")
	}
	// A DOI starts with "10." by convention
	if len(input) > 3 && input[:3] == "10." {
		return "doi", input, nil
	}
	return "", "", errs.Usage("datacite: unrecognized reference: %q", input)
}

// Locate returns the canonical URL for a (uriType, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "doi":
		return fmt.Sprintf("https://doi.org/%s", id), nil
	default:
		return "", errs.Usage("datacite has no resource type %q", uriType)
	}
}
