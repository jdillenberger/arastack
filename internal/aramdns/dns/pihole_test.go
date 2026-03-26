package dns

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jdillenberger/arastack/pkg/clients"
)

func newPiHoleTestServer(t *testing.T) (*httptest.Server, *PiHoleProvider) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth" && r.Method == "POST":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]string{"sid": "test-session-id"},
			})
		case r.URL.Path == "/api/dns/custom" && r.Method == "GET":
			if r.Header.Get("X-FTL-SID") != "test-session-id" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(piholeListResponse{
				Records: []piholeCustomDNSRecord{
					{Domain: "app.local", IP: "192.168.1.1"},
				},
			})
		case r.URL.Path == "/api/dns/custom" && r.Method == "POST":
			if r.Header.Get("X-FTL-SID") != "test-session-id" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			var body piholeCustomDNS
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("failed to decode POST body: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if body.Domain == "" || body.IP == "" {
				t.Errorf("POST body missing domain or ip: %+v", body)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusCreated)
		case r.Method == "DELETE":
			if r.Header.Get("X-FTL-SID") != "test-session-id" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	client := clients.NewBaseClient(srv.URL, 5*time.Second)
	p := NewPiHoleProvider(srv.URL, &client, "test-password")
	return srv, p
}

func TestPiHole_ListEntries(t *testing.T) {
	srv, p := newPiHoleTestServer(t)
	defer srv.Close()

	entries, err := p.ListEntries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Domain != "app.local" || entries[0].Answer != "192.168.1.1" {
		t.Errorf("unexpected entry: %+v", entries[0])
	}
}

func TestPiHole_AddEntry(t *testing.T) {
	srv, p := newPiHoleTestServer(t)
	defer srv.Close()

	err := p.AddEntry(context.Background(), Entry{Domain: "new.local", Answer: "10.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPiHole_RemoveEntry(t *testing.T) {
	var deletePath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth" && r.Method == "POST":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]string{"sid": "test-session-id"},
			})
		case r.Method == "DELETE":
			if r.Header.Get("X-FTL-SID") != "test-session-id" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			deletePath = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := clients.NewBaseClient(srv.URL, 5*time.Second)
	p := NewPiHoleProvider(srv.URL, &client, "test-password")

	err := p.RemoveEntry(context.Background(), Entry{Domain: "old.local", Answer: "10.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if deletePath != "/api/dns/custom/old.local/10.0.0.1" {
		t.Errorf("unexpected DELETE path: %s", deletePath)
	}
}

func TestPiHole_ReauthOnFailure(t *testing.T) {
	authCount := 0
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth" && r.Method == "POST":
			authCount++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]string{"sid": fmt.Sprintf("session-%d", authCount)},
			})
		case r.URL.Path == "/api/dns/custom" && r.Method == "GET":
			requestCount++
			if requestCount == 1 {
				// First request fails (simulating expired session).
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(piholeListResponse{Records: nil})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := clients.NewBaseClient(srv.URL, 5*time.Second)
	p := NewPiHoleProvider(srv.URL, &client, "test-password")

	entries, err := p.ListEntries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
	// Should have authenticated twice: initial + re-auth after 401.
	if authCount != 2 {
		t.Errorf("expected 2 auth calls, got %d", authCount)
	}
}

func TestPiHole_NoRetryOnNon401(t *testing.T) {
	authCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth" && r.Method == "POST":
			authCount++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]string{"sid": fmt.Sprintf("session-%d", authCount)},
			})
		case r.URL.Path == "/api/dns/custom" && r.Method == "GET":
			// Return 500 (not 401) — should NOT trigger re-auth.
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("internal error"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := clients.NewBaseClient(srv.URL, 5*time.Second)
	p := NewPiHoleProvider(srv.URL, &client, "test-password")

	_, err := p.ListEntries(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	// Should have authenticated only once (initial), not re-authed on 500.
	if authCount != 1 {
		t.Errorf("expected 1 auth call (no retry on 500), got %d", authCount)
	}
}

func TestPiHole_AuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/auth" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid password"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := clients.NewBaseClient(srv.URL, 5*time.Second)
	p := NewPiHoleProvider(srv.URL, &client, "wrong-password")

	_, err := p.ListEntries(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid password")
	}
}
