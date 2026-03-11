package portcheck

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/jdillenberger/arastack/pkg/aradeployconfig"
)

func TestUsedPorts(t *testing.T) {
	dir := t.TempDir()

	// App with two port values
	app1Dir := filepath.Join(dir, "myapp")
	if err := os.Mkdir(app1Dir, 0o750); err != nil {
		t.Fatal(err)
	}
	state1 := `name: myapp
values:
  web_port: "8080"
  admin_port: "9090"
  not_a_port: "hello"
`
	if err := os.WriteFile(filepath.Join(app1Dir, aradeployconfig.StateFileName), []byte(state1), 0o600); err != nil {
		t.Fatal(err)
	}

	// App with one port value
	app2Dir := filepath.Join(dir, "other")
	if err := os.Mkdir(app2Dir, 0o750); err != nil {
		t.Fatal(err)
	}
	state2 := `name: other
values:
  web_port: "3000"
`
	if err := os.WriteFile(filepath.Join(app2Dir, aradeployconfig.StateFileName), []byte(state2), 0o600); err != nil {
		t.Fatal(err)
	}

	// Directory without state file (should be skipped)
	if err := os.Mkdir(filepath.Join(dir, "nostate"), 0o750); err != nil {
		t.Fatal(err)
	}

	used, err := UsedPorts(dir)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		port    int
		wantApp string
	}{
		{8080, "myapp"},
		{9090, "myapp"},
		{3000, "other"},
	}
	for _, tt := range tests {
		if got := used[tt.port]; got != tt.wantApp {
			t.Errorf("UsedPorts[%d] = %q, want %q", tt.port, got, tt.wantApp)
		}
	}

	if len(used) != 3 {
		t.Errorf("UsedPorts returned %d entries, want 3", len(used))
	}
}

func TestUsedPorts_MalformedYAML(t *testing.T) {
	dir := t.TempDir()

	// Valid app
	goodDir := filepath.Join(dir, "good")
	if err := os.Mkdir(goodDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goodDir, aradeployconfig.StateFileName), []byte(`name: good
values:
  web_port: "3000"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	// Malformed YAML — should be skipped without error
	badDir := filepath.Join(dir, "bad")
	if err := os.Mkdir(badDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(badDir, aradeployconfig.StateFileName), []byte(`{{not valid yaml`), 0o600); err != nil {
		t.Fatal(err)
	}

	used, err := UsedPorts(dir)
	if err != nil {
		t.Fatalf("UsedPorts should not fail on malformed YAML: %v", err)
	}
	if len(used) != 1 {
		t.Errorf("expected 1 port from good app, got %d: %v", len(used), used)
	}
	if used[3000] != "good" {
		t.Errorf("expected port 3000 -> good, got %q", used[3000])
	}
}

func TestUsedPorts_NonNumericAndInvalidPorts(t *testing.T) {
	dir := t.TempDir()

	appDir := filepath.Join(dir, "app")
	if err := os.Mkdir(appDir, 0o750); err != nil {
		t.Fatal(err)
	}
	state := `name: app
values:
  web_port: "abc"
  api_port: "0"
  db_port: "-1"
  good_port: "5432"
`
	if err := os.WriteFile(filepath.Join(appDir, aradeployconfig.StateFileName), []byte(state), 0o600); err != nil {
		t.Fatal(err)
	}

	used, err := UsedPorts(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Only good_port (5432) should be picked up; abc, 0, -1 are skipped
	if len(used) != 1 {
		t.Errorf("expected 1 port, got %d: %v", len(used), used)
	}
	if used[5432] != "app" {
		t.Errorf("expected port 5432 -> app, got %q", used[5432])
	}
}

func TestUsedPorts_EmptyNameFallback(t *testing.T) {
	dir := t.TempDir()

	appDir := filepath.Join(dir, "dirname-app")
	if err := os.Mkdir(appDir, 0o750); err != nil {
		t.Fatal(err)
	}
	// State file with empty name — should fall back to directory name
	state := `values:
  web_port: "4000"
`
	if err := os.WriteFile(filepath.Join(appDir, aradeployconfig.StateFileName), []byte(state), 0o600); err != nil {
		t.Fatal(err)
	}

	used, err := UsedPorts(dir)
	if err != nil {
		t.Fatal(err)
	}
	if used[4000] != "dirname-app" {
		t.Errorf("expected port 4000 -> dirname-app, got %q", used[4000])
	}
}

func TestUsedPorts_NonexistentDir(t *testing.T) {
	used, err := UsedPorts("/tmp/nonexistent-portcheck-test-dir")
	if err != nil {
		t.Fatal(err)
	}
	if len(used) != 0 {
		t.Errorf("expected empty map, got %v", used)
	}
}

func TestIsPortFree(t *testing.T) {
	// Bind a port on localhost
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()

	boundPort := ln.Addr().(*net.TCPAddr).Port

	if IsPortFree(boundPort) {
		t.Errorf("IsPortFree(%d) = true, want false (port is bound)", boundPort)
	}

	// Find a free port by letting the OS assign one, then closing it
	ln2, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	freePort := ln2.Addr().(*net.TCPAddr).Port
	_ = ln2.Close()

	if !IsPortFree(freePort) {
		t.Errorf("IsPortFree(%d) = false, want true (port was freed)", freePort)
	}
}

func TestNextFreePort(t *testing.T) {
	tests := []struct {
		preferred int
		used      map[int]string
		want      int
	}{
		{8080, nil, 8080},
		{8080, map[int]string{}, 8080},
		{8080, map[int]string{8080: "app1"}, 8081},
		{8080, map[int]string{8080: "a", 8081: "b", 8082: "c"}, 8083},
		{0, nil, 8080},
	}
	for _, tt := range tests {
		got := NextFreePort(tt.preferred, tt.used)
		if got != tt.want {
			t.Errorf("NextFreePort(%d, %v) = %d, want %d", tt.preferred, tt.used, got, tt.want)
		}
	}
}

func TestValidatePort(t *testing.T) {
	used := map[int]string{8080: "myapp", 3000: "other"}

	tests := []struct {
		port       int
		currentApp string
		wantErr    bool
	}{
		{8080, "myapp", false},
		{8080, "newapp", true},
		{3000, "other", false},
		{3000, "newapp", true},
		{9090, "newapp", false},
		{0, "x", true},
		{70000, "x", true},
	}
	for _, tt := range tests {
		err := ValidatePort(tt.port, used, tt.currentApp)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidatePort(%d, used, %q) err=%v, wantErr=%v",
				tt.port, tt.currentApp, err, tt.wantErr)
		}
	}
}

func TestValidatePort_ErrorMessages(t *testing.T) {
	used := map[int]string{8080: "myapp"}

	err := ValidatePort(8080, used, "other")
	if err == nil {
		t.Fatal("expected error")
	}
	want := fmt.Sprintf("port %d is already used by %q", 8080, "myapp")
	if err.Error() != want {
		t.Errorf("error message = %q, want %q", err.Error(), want)
	}
}
