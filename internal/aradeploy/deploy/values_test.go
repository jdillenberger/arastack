package deploy

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jdillenberger/arastack/internal/aradeploy/template"
)

func TestRunValidator_Timezone(t *testing.T) {
	tests := []struct {
		value   string
		wantErr bool
	}{
		{"UTC", false},
		{"Europe/Berlin", false},
		{"America/New_York", false},
		{"Asia/Tokyo", false},
		{"US/Eastern", false},
		{"Local", true},
		{"local", true},
		{"Invalid/Zone", true},
		{"NotATimezone", true},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			err := runValidator("timezone", tt.value, "timezone")
			if (err != nil) != tt.wantErr {
				t.Errorf("runValidator(timezone, %q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestHostTimezone(t *testing.T) {
	tz := hostTimezone()
	if tz == "" {
		t.Fatal("hostTimezone() returned empty string")
	}
	if _, err := time.LoadLocation(tz); err != nil {
		t.Errorf("hostTimezone() returned invalid timezone %q: %v", tz, err)
	}
}

func TestSaveAndLoadSecrets(t *testing.T) {
	dir := t.TempDir()

	values := map[string]string{
		"db_password":   "secret123",
		"admin_password": "admin456",
		"web_port":      "8080",
	}
	autoGen := map[string]bool{
		"db_password":   true,
		"admin_password": true,
	}

	if err := SaveSecrets(dir, values, autoGen); err != nil {
		t.Fatalf("SaveSecrets() error = %v", err)
	}

	// Verify file exists with correct permissions
	info, err := os.Stat(filepath.Join(dir, SecretsFileName))
	if err != nil {
		t.Fatalf("secrets file not found: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("secrets file permissions = %o, want 600", info.Mode().Perm())
	}

	loaded := loadSavedSecrets(dir)
	if loaded == nil {
		t.Fatal("loadSavedSecrets() returned nil")
	}
	if loaded["db_password"] != "secret123" {
		t.Errorf("db_password = %q, want %q", loaded["db_password"], "secret123")
	}
	if loaded["admin_password"] != "admin456" {
		t.Errorf("admin_password = %q, want %q", loaded["admin_password"], "admin456")
	}
	if _, ok := loaded["web_port"]; ok {
		t.Error("web_port should not be saved (not auto-generated)")
	}
}

func TestLoadSavedSecrets_Missing(t *testing.T) {
	dir := t.TempDir()
	loaded := loadSavedSecrets(dir)
	if loaded != nil {
		t.Errorf("loadSavedSecrets() = %v, want nil for missing file", loaded)
	}
}

func TestSaveSecretsFromState(t *testing.T) {
	dir := t.TempDir()

	meta := &template.AppMeta{
		Values: []template.Value{
			{Name: "db_password", AutoGen: "password"},
			{Name: "web_port"},
		},
	}
	info := &DeployedApp{
		Values: map[string]string{
			"db_password": "fromstate",
			"web_port":    "8080",
		},
	}

	if err := SaveSecretsFromState(dir, info, meta); err != nil {
		t.Fatalf("SaveSecretsFromState() error = %v", err)
	}

	loaded := loadSavedSecrets(dir)
	if loaded["db_password"] != "fromstate" {
		t.Errorf("db_password = %q, want %q", loaded["db_password"], "fromstate")
	}
	if _, ok := loaded["web_port"]; ok {
		t.Error("web_port should not be saved (no auto_gen)")
	}
}
