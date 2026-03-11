package peersecret

import (
	"os"
	"path/filepath"
	"testing"
)

type mockAuthSetter struct {
	secret string
	calls  int
}

func (m *mockAuthSetter) SetAuth(secret string) {
	m.secret = secret
	m.calls++
}

func TestReloadReadsPeerGroupSecret(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "peers.yaml")

	content := []byte("peer_group:\n  secret: test-secret-123\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}

	mock := &mockAuthSetter{}
	w := New(path, mock)
	w.reload()

	if mock.secret != "test-secret-123" {
		t.Errorf("expected secret %q, got %q", "test-secret-123", mock.secret)
	}
	if mock.calls != 1 {
		t.Errorf("expected 1 SetAuth call, got %d", mock.calls)
	}
}

func TestReloadSkipsWhenUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "peers.yaml")

	content := []byte("peer_group:\n  secret: same-secret\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}

	mock := &mockAuthSetter{}
	w := New(path, mock)
	w.reload()
	w.reload()

	if mock.calls != 1 {
		t.Errorf("expected 1 SetAuth call (no change), got %d", mock.calls)
	}
}

func TestReloadDetectsChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "peers.yaml")

	if err := os.WriteFile(path, []byte("peer_group:\n  secret: old\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	mock := &mockAuthSetter{}
	w := New(path, mock)
	w.reload()

	if err := os.WriteFile(path, []byte("peer_group:\n  secret: new\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	w.reload()

	if mock.secret != "new" {
		t.Errorf("expected secret %q, got %q", "new", mock.secret)
	}
	if mock.calls != 2 {
		t.Errorf("expected 2 SetAuth calls, got %d", mock.calls)
	}
}

func TestReloadHandlesMissingFile(t *testing.T) {
	mock := &mockAuthSetter{}
	w := New("/nonexistent/peers.yaml", mock)
	w.reload()

	if mock.calls != 0 {
		t.Errorf("expected 0 SetAuth calls for missing file, got %d", mock.calls)
	}
}

func TestReloadHandlesEmptySecret(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "peers.yaml")

	if err := os.WriteFile(path, []byte("peer_group:\n  name: test\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	mock := &mockAuthSetter{}
	w := New(path, mock)
	w.reload()

	if mock.calls != 0 {
		t.Errorf("expected 0 SetAuth calls for empty secret, got %d", mock.calls)
	}
}
