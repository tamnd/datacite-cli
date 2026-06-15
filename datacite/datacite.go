// Package datacite is the library behind the datacite command line:
// the HTTP client, request shaping, and the typed data models for the
// DataCite DOI Registry API at api.datacite.org.
//
// The DataCite REST API is open for public read-only data: no API key, no auth
// required. This package wraps the API with a rate-limited client that the
// kit operations consume.
package datacite

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Host is the human-facing DataCite site.
const Host = "datacite.org"

// DefaultUserAgent identifies the client to DataCite honestly.
const DefaultUserAgent = "datacite-cli/0.1 (tamnd87@gmail.com)"

// Config holds constructor parameters for Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Timeout   time.Duration
	Retries   int
}

// DefaultConfig returns sensible defaults for api.datacite.org.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://api.datacite.org",
		UserAgent: DefaultUserAgent,
		Rate:      500 * time.Millisecond,
		Timeout:   15 * time.Second,
		Retries:   3,
	}
}

// Client is a rate-limited HTTP client for the DataCite API.
type Client struct {
	cfg     Config
	http    *http.Client
	mu      sync.Mutex
	lastReq time.Time
}

// NewClient returns a Client configured with cfg.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// --- output types ---

// DOI is a single DataCite DOI record.
type DOI struct {
	DOI         string `json:"doi" kit:"id"`
	Title       string `json:"title"`
	Creators    string `json:"creators"`
	Year        string `json:"year"`
	Publisher   string `json:"publisher"`
	Type        string `json:"type"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

// Funder is a single DataCite funder record.
type Funder struct {
	ID      string `json:"id" kit:"id"`
	Name    string `json:"name"`
	Region  string `json:"region"`
	Country string `json:"country"`
}

// Member is a single DataCite member record.
type Member struct {
	ID         string `json:"id" kit:"id"`
	Name       string `json:"name"`
	Region     string `json:"region"`
	Country    string `json:"country"`
	MemberType string `json:"member_type"`
}

// --- wire types ---

type wireResponse struct {
	Data  json.RawMessage `json:"data"`
	Meta  wireMeta        `json:"meta"`
}

type wireMeta struct {
	Total      int `json:"total"`
	TotalPages int `json:"totalPages"`
	Page       int `json:"page"`
}

type wireDOI struct {
	ID         string          `json:"id"`
	Attributes wireDOIAttributes `json:"attributes"`
}

type wireDOIAttributes struct {
	DOI             string        `json:"doi"`
	Titles          []wireTitle   `json:"titles"`
	Creators        []wireCreator `json:"creators"`
	PublicationYear int           `json:"publicationYear"`
	Publisher       string        `json:"publisher"`
	Types           wireTypes     `json:"types"`
	Descriptions    []wireDesc    `json:"descriptions"`
	URL             string        `json:"url"`
}

type wireTitle struct {
	Title string `json:"title"`
}

type wireCreator struct {
	Name string `json:"name"`
}

type wireDesc struct {
	Description string `json:"description"`
}

type wireTypes struct {
	ResourceTypeGeneral string `json:"resourceTypeGeneral"`
}

type wireFunder struct {
	ID         string             `json:"id"`
	Attributes wireFunderAttrs    `json:"attributes"`
}

type wireFunderAttrs struct {
	Name    string          `json:"name"`
	Region  string          `json:"region"`
	Country wireFunderCountry `json:"country"`
}

type wireFunderCountry struct {
	Name string `json:"name"`
}

type wireMember struct {
	ID         string           `json:"id"`
	Attributes wireMemberAttrs  `json:"attributes"`
}

type wireMemberAttrs struct {
	Name       string `json:"name"`
	Region     string `json:"region"`
	Country    string `json:"country"`
	MemberType string `json:"memberType"`
}

// --- converters ---

func toDOI(w wireDOI) DOI {
	attrs := w.Attributes
	doi := attrs.DOI
	if doi == "" {
		doi = w.ID
	}

	var title string
	if len(attrs.Titles) > 0 {
		title = attrs.Titles[0].Title
	}

	var creators []string
	for _, c := range attrs.Creators {
		if c.Name != "" {
			creators = append(creators, c.Name)
		}
	}

	var desc string
	if len(attrs.Descriptions) > 0 {
		desc = attrs.Descriptions[0].Description
	}

	year := ""
	if attrs.PublicationYear > 0 {
		year = fmt.Sprintf("%d", attrs.PublicationYear)
	}

	return DOI{
		DOI:         doi,
		Title:       title,
		Creators:    strings.Join(creators, ", "),
		Year:        year,
		Publisher:   attrs.Publisher,
		Type:        attrs.Types.ResourceTypeGeneral,
		Description: desc,
		URL:         attrs.URL,
	}
}

func toFunder(w wireFunder) Funder {
	return Funder{
		ID:      w.ID,
		Name:    w.Attributes.Name,
		Region:  w.Attributes.Region,
		Country: w.Attributes.Country.Name,
	}
}

func toMember(w wireMember) Member {
	return Member{
		ID:         w.ID,
		Name:       w.Attributes.Name,
		Region:     w.Attributes.Region,
		Country:    w.Attributes.Country,
		MemberType: w.Attributes.MemberType,
	}
}

// --- API methods ---

// SearchDOIs searches the DataCite DOI registry.
// Returns matching DOIs, the total count, and any error.
func (c *Client) SearchDOIs(ctx context.Context, query string, page, size int) ([]DOI, int, error) {
	if size <= 0 {
		size = 25
	}
	if page <= 0 {
		page = 1
	}
	q := url.Values{}
	q.Set("query", query)
	q.Set("page[size]", fmt.Sprintf("%d", size))
	q.Set("page[number]", fmt.Sprintf("%d", page))

	var resp wireResponse
	if err := c.getJSON(ctx, c.cfg.BaseURL+"/dois?"+q.Encode(), &resp); err != nil {
		return nil, 0, err
	}

	var raw []wireDOI
	if err := json.Unmarshal(resp.Data, &raw); err != nil {
		return nil, 0, fmt.Errorf("decode DOIs: %w", err)
	}

	out := make([]DOI, len(raw))
	for i, w := range raw {
		out[i] = toDOI(w)
	}
	return out, resp.Meta.Total, nil
}

// GetDOI fetches a single DOI record by its identifier (e.g. "10.1234/example").
func (c *Client) GetDOI(ctx context.Context, id string) (*DOI, error) {
	var resp struct {
		Data wireDOI `json:"data"`
	}
	if err := c.getJSON(ctx, c.cfg.BaseURL+"/dois/"+url.PathEscape(id), &resp); err != nil {
		return nil, err
	}
	d := toDOI(resp.Data)
	return &d, nil
}

// ListFunders fetches a page of DataCite funders.
func (c *Client) ListFunders(ctx context.Context, page, size int) ([]Funder, error) {
	if size <= 0 {
		size = 25
	}
	if page <= 0 {
		page = 1
	}
	q := url.Values{}
	q.Set("page[size]", fmt.Sprintf("%d", size))
	q.Set("page[number]", fmt.Sprintf("%d", page))

	var resp wireResponse
	if err := c.getJSON(ctx, c.cfg.BaseURL+"/funders?"+q.Encode(), &resp); err != nil {
		return nil, err
	}

	var raw []wireFunder
	if err := json.Unmarshal(resp.Data, &raw); err != nil {
		return nil, fmt.Errorf("decode funders: %w", err)
	}

	out := make([]Funder, len(raw))
	for i, w := range raw {
		out[i] = toFunder(w)
	}
	return out, nil
}

// ListMembers fetches a page of DataCite members.
func (c *Client) ListMembers(ctx context.Context, page, size int) ([]Member, error) {
	if size <= 0 {
		size = 25
	}
	if page <= 0 {
		page = 1
	}
	q := url.Values{}
	q.Set("page[size]", fmt.Sprintf("%d", size))
	q.Set("page[number]", fmt.Sprintf("%d", page))

	var resp wireResponse
	if err := c.getJSON(ctx, c.cfg.BaseURL+"/members?"+q.Encode(), &resp); err != nil {
		return nil, err
	}

	var raw []wireMember
	if err := json.Unmarshal(resp.Data, &raw); err != nil {
		return nil, fmt.Errorf("decode members: %w", err)
	}

	out := make([]Member, len(raw))
	for i, w := range raw {
		out[i] = toMember(w)
	}
	return out, nil
}

// --- HTTP helpers ---

func (c *Client) getJSON(ctx context.Context, rawURL string, v any) error {
	body, err := c.get(ctx, rawURL)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("decode %s: %w", rawURL, err)
	}
	return nil
}

func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			d := time.Duration(attempt) * 500 * time.Millisecond
			if d > 5*time.Second {
				d = 5 * time.Second
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(d):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "application/vnd.api+json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least Rate has passed since the last request.
func (c *Client) pace() {
	if c.cfg.Rate <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if wait := c.cfg.Rate - time.Since(c.lastReq); wait > 0 {
		time.Sleep(wait)
	}
	c.lastReq = time.Now()
}
