package dns

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jdillenberger/arastack/pkg/clients"
)

func TestAdGuard_ListEntries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/control/rewrite/list" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]adguardRewrite{
			{Domain: "app.local", Answer: "192.168.1.1"},
			{Domain: "blog.home.lan", Answer: "192.168.1.2"},
		})
	}))
	defer srv.Close()

	client := clients.NewBaseClient(srv.URL, 5*time.Second)
	p := NewAdGuardProvider(srv.URL, &client)

	entries, err := p.ListEntries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Domain != "app.local" || entries[0].Answer != "192.168.1.1" {
		t.Errorf("unexpected entry: %+v", entries[0])
	}
}

func TestAdGuard_AddEntry(t *testing.T) {
	var received adguardRewrite
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/control/rewrite/add" || r.Method != "POST" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := clients.NewBaseClient(srv.URL, 5*time.Second)
	p := NewAdGuardProvider(srv.URL, &client)

	err := p.AddEntry(context.Background(), Entry{Domain: "new.local", Answer: "10.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if received.Domain != "new.local" || received.Answer != "10.0.0.1" {
		t.Errorf("unexpected body: %+v", received)
	}
}

func TestAdGuard_RemoveEntry(t *testing.T) {
	var received adguardRewrite
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/control/rewrite/delete" || r.Method != "POST" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := clients.NewBaseClient(srv.URL, 5*time.Second)
	p := NewAdGuardProvider(srv.URL, &client)

	err := p.RemoveEntry(context.Background(), Entry{Domain: "old.local", Answer: "10.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if received.Domain != "old.local" || received.Answer != "10.0.0.1" {
		t.Errorf("unexpected body: %+v", received)
	}
}

func TestAdGuard_BasicAuth(t *testing.T) {
	var gotUser, gotPass string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPass, _ = r.BasicAuth()
		_ = json.NewEncoder(w).Encode([]adguardRewrite{})
	}))
	defer srv.Close()

	client := clients.NewBaseClient(srv.URL, 5*time.Second)
	client.SetBasicAuth("admin", "secret")
	p := NewAdGuardProvider(srv.URL, &client)

	_, _ = p.ListEntries(context.Background())
	if gotUser != "admin" || gotPass != "secret" {
		t.Errorf("expected admin/secret, got %s/%s", gotUser, gotPass)
	}
}
