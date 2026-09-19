package skritter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(h)
	t.Cleanup(server.Close)
	c := NewClient("test-token")
	c.baseURL = server.URL
	return c
}
func TestPaginationAndAuthorization(t *testing.T) {
	calls := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer test-token" || r.URL.Query().Get("bearer_token") != "" {
			t.Error("incorrect credential transport")
		}
		if r.URL.Query().Get("sort") != "custom" {
			t.Error("not requesting owned lists")
		}
		if calls == 1 {
			_, _ = w.Write([]byte(`{"VocabLists":[],"cursor":"next"}`))
		} else {
			if r.URL.Query().Get("cursor") != "next" {
				t.Error("missing cursor")
			}
			_, _ = w.Write([]byte(`{"VocabLists":[{"id":"1","name":"Polybius","lang":"zh"}]}`))
		}
	})
	lists, err := c.Lists(context.Background())
	if err != nil || len(lists) != 1 || calls != 2 {
		t.Fatalf("%v %v", lists, err)
	}
}
func TestMalformedAndRejectedResponses(t *testing.T) {
	for _, body := range []string{`{}`, `<html>login</html>`, `{"error":"secret test-token","statusCode":403}`, `{"VocabLists":null}`} {
		t.Run(body, func(t *testing.T) {
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) })
			_, err := c.Lists(context.Background())
			if err == nil || strings.Contains(err.Error(), "test-token") {
				t.Fatalf("unsafe or absent error: %v", err)
			}
		})
	}
}
func TestRedirectDoesNotFollowWrite(t *testing.T) {
	followed := false
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/elsewhere" {
			followed = true
		}
		http.Redirect(w, r, "/elsewhere", 302)
	})
	_, err := c.CreateList(context.Background())
	if err == nil || followed {
		t.Fatal("followed write redirect")
	}
}
func TestCreateRejectsListCollection(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"VocabLists":[],"statusCode":200}`))
	})
	if _, err := c.CreateList(context.Background()); err == nil {
		t.Fatal("GET-shaped result accepted as creation")
	}
}
func TestPutPreservesUnknownRowFields(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("bad write request")
		}
		var body struct {
			Rows []Row `json:"rows"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if string(body.Rows[0]["custom"]) != `true` {
			t.Error("unknown field lost")
		}
		_, _ = w.Write([]byte(`{"VocabListSection":{"id":"section"}}`))
	})
	row := newRow("old", "old")
	row["custom"] = json.RawMessage(`true`)
	if err := c.PutRows(context.Background(), "list", "section", []Row{row}); err != nil {
		t.Fatal(err)
	}
}

func TestMissingRowsCannotBeOverwritten(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"VocabListSection":{"id":"section"}}`))
	})
	if _, err := c.GetSection(context.Background(), "list", "section"); err == nil {
		t.Fatal("accepted an incomplete section")
	}
}
