package clients

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDeleteJSON_WithoutBody(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		if r.Header.Get("Content-Type") != "" {
			t.Error("DELETE without body should not set Content-Type")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewBaseClient(srv.URL, 5*time.Second)
	err := c.DeleteJSON(context.Background(), "/test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != "DELETE" {
		t.Errorf("expected DELETE, got %s", gotMethod)
	}
}

func TestDeleteJSON_WithBody(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("DELETE with body should set Content-Type")
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewBaseClient(srv.URL, 5*time.Second)
	err := c.DeleteJSON(context.Background(), "/test", map[string]string{"key": "val"})
	if err != nil {
		t.Fatal(err)
	}
	if gotBody["key"] != "val" {
		t.Errorf("expected key=val, got %v", gotBody)
	}
}

func TestDeleteJSON_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
	}))
	defer srv.Close()

	c := NewBaseClient(srv.URL, 5*time.Second)
	err := c.DeleteJSON(context.Background(), "/missing", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 in error, got: %s", err)
	}
}

func TestPostJSONWithResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		_ = json.NewEncoder(w).Encode(map[string]string{"echo": body["input"]})
	}))
	defer srv.Close()

	c := NewBaseClient(srv.URL, 5*time.Second)
	var result map[string]string
	err := c.PostJSONWithResult(context.Background(), "/echo", map[string]string{"input": "hello"}, &result)
	if err != nil {
		t.Fatal(err)
	}
	if result["echo"] != "hello" {
		t.Errorf("expected echo=hello, got %v", result)
	}
}

func TestPostJSONWithResult_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad request"))
	}))
	defer srv.Close()

	c := NewBaseClient(srv.URL, 5*time.Second)
	err := c.PostJSONWithResult(context.Background(), "/fail", map[string]string{}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("expected 400 in error, got: %s", err)
	}
}

func TestSetHeader(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Custom")
		_ = json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer srv.Close()

	c := NewBaseClient(srv.URL, 5*time.Second)
	c.SetHeader("X-Custom", "test-value")
	_ = c.GetJSON(context.Background(), "/test", nil)
	if gotHeader != "test-value" {
		t.Errorf("expected X-Custom=test-value, got %s", gotHeader)
	}
}

func TestSetBasicAuth(t *testing.T) {
	var gotUser, gotPass string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPass, _ = r.BasicAuth()
		_ = json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer srv.Close()

	c := NewBaseClient(srv.URL, 5*time.Second)
	c.SetBasicAuth("admin", "secret")
	_ = c.GetJSON(context.Background(), "/test", nil)
	if gotUser != "admin" || gotPass != "secret" {
		t.Errorf("expected admin/secret, got %s/%s", gotUser, gotPass)
	}
}

func TestBearerAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer srv.Close()

	c := NewBaseClient(srv.URL, 5*time.Second)
	c.SetAuth("my-token")
	_ = c.GetJSON(context.Background(), "/test", nil)
	if gotAuth != "Bearer my-token" {
		t.Errorf("expected Bearer my-token, got %s", gotAuth)
	}
}

func TestBasicAuthOverridesBearer(t *testing.T) {
	var gotAuth string
	var gotUser string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotUser, _, _ = r.BasicAuth()
		_ = json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer srv.Close()

	c := NewBaseClient(srv.URL, 5*time.Second)
	c.SetAuth("my-token")
	c.SetBasicAuth("admin", "pass")
	_ = c.GetJSON(context.Background(), "/test", nil)
	// Basic auth should take precedence
	if gotUser != "admin" {
		t.Errorf("expected basic auth user admin, got %s", gotUser)
	}
	if strings.HasPrefix(gotAuth, "Bearer") {
		t.Error("basic auth should override bearer")
	}
}

func TestErrorBodyLimited(t *testing.T) {
	// Verify that error responses don't read unlimited data.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		// Write more than maxErrorBodySize (64KB)
		_, _ = w.Write([]byte(strings.Repeat("x", 100*1024)))
	}))
	defer srv.Close()

	c := NewBaseClient(srv.URL, 5*time.Second)
	err := c.GetJSON(context.Background(), "/big-error", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	// The error message should be truncated, not contain 100KB of data.
	if len(err.Error()) > maxErrorBodySize+200 {
		t.Errorf("error body not limited: got %d bytes", len(err.Error()))
	}
}

func TestPostJSONWithRetry_Success(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewBaseClient(srv.URL, 5*time.Second)
	err := c.PostJSONWithRetry(context.Background(), "/retry", map[string]string{}, 3)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestPostJSONWithRetry_Exhausted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewBaseClient(srv.URL, 5*time.Second)
	err := c.PostJSONWithRetry(context.Background(), "/fail", map[string]string{}, 1)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if !strings.Contains(err.Error(), "after 1 retries") {
		t.Errorf("expected retry exhaustion message, got: %s", err)
	}
}

func TestIsHTTPStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("unauthorized"))
	}))
	defer srv.Close()

	c := NewBaseClient(srv.URL, 5*time.Second)
	err := c.GetJSON(context.Background(), "/protected", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsHTTPStatus(err, http.StatusUnauthorized) {
		t.Errorf("expected IsHTTPStatus(err, 401) to be true")
	}
	if IsHTTPStatus(err, http.StatusNotFound) {
		t.Errorf("expected IsHTTPStatus(err, 404) to be false")
	}
}

func TestPostJSONWithRetry_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	c := NewBaseClient(srv.URL, 5*time.Second)
	err := c.PostJSONWithRetry(ctx, "/fail", map[string]string{}, 5)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
