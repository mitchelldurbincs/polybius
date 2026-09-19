// Package skritter adds Chinese vocabulary to a private Polybius list.
package skritter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const BaseURL = "https://legacy.skritter.com/api/v0"

type Client struct {
	token   string
	baseURL string
	http    *http.Client
}

func NewClient(token string) *Client {
	return &Client{token: strings.TrimSpace(token), baseURL: BaseURL, http: &http.Client{
		Timeout: 20 * time.Second,
		// A redirect can silently turn a write into a GET or forward credentials.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

type Row map[string]json.RawMessage

func (r Row) id(key string) string { var s string; _ = json.Unmarshal(r[key], &s); return s }
func newRow(simp, trad string) Row {
	s, _ := json.Marshal(simp)
	t, _ := json.Marshal(trad)
	return Row{"vocabId": s, "tradVocabId": t}
}

type Section struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name"`
	Rows    []Row  `json:"rows,omitempty"`
	Deleted bool   `json:"deleted,omitempty"`
}
type List struct {
	ID       string    `json:"id,omitempty"`
	Name     string    `json:"name"`
	Lang     string    `json:"lang"`
	Sections []Section `json:"sections"`
	Deleted  bool      `json:"deleted,omitempty"`
	Disabled bool      `json:"disabled,omitempty"`
}
type Vocab struct {
	ID          string            `json:"id"`
	Writing     string            `json:"writing"`
	Reading     string            `json:"reading"`
	Style       string            `json:"style"`
	Definitions map[string]string `json:"definitions"`
}

func (c *Client) request(ctx context.Context, method, path string, query url.Values, body, out any) error {
	if c.token == "" {
		return fmt.Errorf("Skritter is not connected; set SKRITTER_TOKEN or save ~/.polybius/skritter-token")
	}
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("Skritter request failed; check connection and retry")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Skritter returned HTTP %d", resp.StatusCode)
	}
	data, err = io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("could not read Skritter response")
	}
	var status struct {
		StatusCode int             `json:"statusCode"`
		Error      json.RawMessage `json:"error"`
	}
	if json.Unmarshal(data, &status) != nil {
		return fmt.Errorf("Skritter returned an invalid response")
	}
	if status.StatusCode >= 400 || (len(status.Error) > 0 && string(status.Error) != "null") {
		return fmt.Errorf("Skritter rejected the request (status %d)", status.StatusCode)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("unexpected Skritter response format")
	}
	return nil
}

func (c *Client) Lists(ctx context.Context) ([]List, error) {
	var lists []List
	seen := map[string]bool{}
	cursor := ""
	for {
		q := url.Values{"sort": {"custom"}, "lang": {"zh"}, "limit": {"100"}}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		var p struct {
			Lists  *[]List `json:"VocabLists"`
			Cursor string  `json:"cursor"`
		}
		if err := c.request(ctx, "GET", "/vocablists", q, nil, &p); err != nil {
			return nil, err
		}
		if p.Lists == nil {
			return nil, fmt.Errorf("Skritter did not return custom lists")
		}
		lists = append(lists, (*p.Lists)...)
		if p.Cursor == "" {
			return lists, nil
		}
		if seen[p.Cursor] {
			return nil, fmt.Errorf("Skritter repeated a list cursor")
		}
		seen[p.Cursor] = true
		cursor = p.Cursor
	}
}
func (c *Client) GetList(ctx context.Context, id string) (*List, error) {
	var p struct {
		List *List `json:"VocabList"`
	}
	if err := c.request(ctx, "GET", "/vocablists/"+url.PathEscape(id), nil, nil, &p); err != nil {
		return nil, err
	}
	if p.List == nil || p.List.ID != id {
		return nil, fmt.Errorf("Skritter list was not found")
	}
	for _, section := range p.List.Sections {
		if !section.Deleted && (section.ID == "" || section.Rows == nil) {
			return nil, fmt.Errorf("Skritter returned an incomplete list section")
		}
	}
	return p.List, nil
}
func (c *Client) CreateList(ctx context.Context) (*List, error) {
	var p struct {
		List *List `json:"VocabList"`
	}
	body := List{Name: "Polybius", Lang: "zh", Sections: []Section{{Name: "Words"}}}
	if err := c.request(ctx, "POST", "/vocablists", nil, body, &p); err != nil {
		return nil, err
	}
	if p.List == nil || p.List.ID == "" || p.List.Name != "Polybius" {
		return nil, fmt.Errorf("Skritter did not confirm list creation; retry to check for the list")
	}
	return c.GetList(ctx, p.List.ID)
}
func (c *Client) Search(ctx context.Context, word Word) ([]Vocab, error) {
	q := word.Writing
	if word.Reading != "" {
		q += "\t" + word.Reading
	}
	var p struct {
		Vocabs *[]Vocab `json:"Vocabs"`
		Cursor string   `json:"cursor"`
	}
	if err := c.request(ctx, "GET", "/vocabs", url.Values{"q": {q}, "lang": {"zh"}, "limit": {"100"}}, nil, &p); err != nil {
		return nil, err
	}
	if p.Vocabs == nil {
		return nil, fmt.Errorf("Skritter did not return vocabulary")
	}
	if p.Cursor != "" {
		return nil, fmt.Errorf("too many matches for %s; specify pinyin", word.Writing)
	}
	return *p.Vocabs, nil
}
func (c *Client) GetSection(ctx context.Context, listID, sectionID string) (*Section, error) {
	var p struct {
		Section *Section `json:"VocabListSection"`
	}
	if err := c.request(ctx, "GET", sectionPath(listID, sectionID), nil, nil, &p); err != nil {
		return nil, err
	}
	if p.Section == nil || p.Section.ID != sectionID || p.Section.Rows == nil {
		return nil, fmt.Errorf("Skritter section was missing or incomplete")
	}
	return p.Section, nil
}
func (c *Client) PutRows(ctx context.Context, listID, sectionID string, rows []Row) error {
	var p struct {
		Section *Section `json:"VocabListSection"`
	}
	if err := c.request(ctx, "PUT", sectionPath(listID, sectionID), nil, struct {
		Rows []Row `json:"rows"`
	}{rows}, &p); err != nil {
		return err
	}
	if p.Section == nil || p.Section.ID != sectionID {
		return fmt.Errorf("Skritter did not confirm the update; retry to check membership")
	}
	return nil
}
func sectionPath(listID, sectionID string) string {
	return "/vocablists/" + url.PathEscape(listID) + "/sections/" + url.PathEscape(sectionID)
}
