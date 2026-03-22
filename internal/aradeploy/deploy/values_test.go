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

	secrets := map[string]string{
		"db_password":    "secret123",
		"admin_password": "admin456",
	}

	if err := saveSecrets(dir, secrets); err != nil {
		t.Fatalf("saveSecrets() error = %v", err)
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
}

func TestSaveSecrets_Empty(t *testing.T) {
	dir := t.TempDir()
	if err := saveSecrets(dir, nil); err != nil {
		t.Fatalf("saveSecrets(nil) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, SecretsFileName)); !os.IsNotExist(err) {
		t.Error("secrets file should not be created for empty secrets")
	}
}

func TestLoadSavedSecrets_Missing(t *testing.T) {
	dir := t.TempDir()
	loaded := loadSavedSecrets(dir)
	if loaded != nil {
		t.Errorf("loadSavedSecrets() = %v, want nil for missing file", loaded)
	}
}

func TestCollectAutoGenSecrets(t *testing.T) {
	meta := &template.AppMeta{
		Values: []template.Value{
			{Name: "db_password", AutoGen: "password"},
			{Name: "api_key", AutoGen: "uuid"},
			{Name: "web_port"},
		},
	}
	values := map[string]string{
		"db_password": "fromstate",
		"api_key":     "some-uuid",
		"web_port":    "8080",
	}

	secrets := collectAutoGenSecrets(meta, values)
	if secrets["db_password"] != "fromstate" {
		t.Errorf("db_password = %q, want %q", secrets["db_password"], "fromstate")
	}
	if secrets["api_key"] != "some-uuid" {
		t.Errorf("api_key = %q, want %q", secrets["api_key"], "some-uuid")
	}
	if _, ok := secrets["web_port"]; ok {
		t.Error("web_port should not be collected (no auto_gen)")
	}
}

func TestCollectAutoGenSecrets_SkipsEmpty(t *testing.T) {
	meta := &template.AppMeta{
		Values: []template.Value{
			{Name: "db_password", AutoGen: "password"},
		},
	}
	values := map[string]string{
		"db_password": "",
	}

	secrets := collectAutoGenSecrets(meta, values)
	if _, ok := secrets["db_password"]; ok {
		t.Error("empty secret values should not be collected")
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()

	meta := &template.AppMeta{
		Values: []template.Value{
			{Name: "db_password", AutoGen: "password"},
			{Name: "web_port"},
		},
	}
	values := map[string]string{
		"db_password": "roundtrip-secret",
		"web_port":    "8080",
	}

	secrets := collectAutoGenSecrets(meta, values)
	if err := saveSecrets(dir, secrets); err != nil {
		t.Fatalf("saveSecrets() error = %v", err)
	}

	loaded := loadSavedSecrets(dir)
	if loaded["db_password"] != "roundtrip-secret" {
		t.Errorf("db_password = %q, want %q", loaded["db_password"], "roundtrip-secret")
	}
	if _, ok := loaded["web_port"]; ok {
		t.Error("web_port should not survive round-trip")
	}
}
