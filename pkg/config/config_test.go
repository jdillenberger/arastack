package config

import (
	"os"
	"path/filepath"
	"testing"
)

type testServerCfg struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type testProviderCfg struct {
	Type     string `yaml:"type"`
	URL      string `yaml:"url"`
	Password string `yaml:"password"`
}

type testCfg struct {
	Name      string            `yaml:"name"`
	Debug     bool              `yaml:"debug"`
	Tags      []string          `yaml:"tags"`
	Server    testServerCfg     `yaml:"server"`
	Timeout   int               `yaml:"timeout"`
	Enabled   *bool             `yaml:"enabled"`
	Providers []testProviderCfg `yaml:"providers"`
}

func TestApplyEnvOverrides_String(t *testing.T) {
	cfg := &testCfg{Name: "default"}
	t.Setenv("TEST_NAME", "override")
	applyEnvOverrides(cfg, "TEST")
	if cfg.Name != "override" {
		t.Errorf("Name = %q, want %q", cfg.Name, "override")
	}
}

func TestApplyEnvOverrides_Int(t *testing.T) {
	cfg := &testCfg{Timeout: 30}
	t.Setenv("TEST_TIMEOUT", "60")
	applyEnvOverrides(cfg, "TEST")
	if cfg.Timeout != 60 {
		t.Errorf("Timeout = %d, want %d", cfg.Timeout, 60)
	}
}

func TestApplyEnvOverrides_Bool(t *testing.T) {
	cfg := &testCfg{Debug: false}
	t.Setenv("TEST_DEBUG", "true")
	applyEnvOverrides(cfg, "TEST")
	if !cfg.Debug {
		t.Error("Debug = false, want true")
	}
}

func TestApplyEnvOverrides_Slice(t *testing.T) {
	cfg := &testCfg{}
	t.Setenv("TEST_TAGS", "a, b, c")
	applyEnvOverrides(cfg, "TEST")
	if len(cfg.Tags) != 3 {
		t.Fatalf("Tags length = %d, want 3", len(cfg.Tags))
	}
	if cfg.Tags[0] != "a" || cfg.Tags[1] != "b" || cfg.Tags[2] != "c" {
		t.Errorf("Tags = %v, want [a b c]", cfg.Tags)
	}
}

func TestApplyEnvOverrides_NestedStruct(t *testing.T) {
	cfg := &testCfg{Server: testServerCfg{Host: "localhost", Port: 8080}}
	t.Setenv("TEST_SERVER_HOST", "0.0.0.0")
	t.Setenv("TEST_SERVER_PORT", "9090")
	applyEnvOverrides(cfg, "TEST")
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, "0.0.0.0")
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 9090)
	}
}

func TestApplyEnvOverrides_PtrBool(t *testing.T) {
	cfg := &testCfg{} // Enabled is nil
	t.Setenv("TEST_ENABLED", "false")
	applyEnvOverrides(cfg, "TEST")
	if cfg.Enabled == nil {
		t.Fatal("Enabled is nil, want *false")
	}
	if *cfg.Enabled {
		t.Error("Enabled = true, want false")
	}
}

func TestApplyEnvOverrides_PtrBoolTrue(t *testing.T) {
	f := false
	cfg := &testCfg{Enabled: &f}
	t.Setenv("TEST_ENABLED", "true")
	applyEnvOverrides(cfg, "TEST")
	if cfg.Enabled == nil {
		t.Fatal("Enabled is nil, want *true")
	}
	if !*cfg.Enabled {
		t.Error("Enabled = false, want true")
	}
}

func TestApplyEnvOverrides_SliceOfStructs(t *testing.T) {
	cfg := &testCfg{}
	t.Setenv("TEST_PROVIDERS_0_TYPE", "adguard")
	t.Setenv("TEST_PROVIDERS_0_URL", "http://192.168.1.2:3000")
	t.Setenv("TEST_PROVIDERS_0_PASSWORD", "secret")
	t.Setenv("TEST_PROVIDERS_1_TYPE", "pihole")
	t.Setenv("TEST_PROVIDERS_1_URL", "http://192.168.1.3")
	applyEnvOverrides(cfg, "TEST")
	if len(cfg.Providers) != 2 {
		t.Fatalf("Providers length = %d, want 2", len(cfg.Providers))
	}
	if cfg.Providers[0].Type != "adguard" || cfg.Providers[0].URL != "http://192.168.1.2:3000" || cfg.Providers[0].Password != "secret" {
		t.Errorf("Providers[0] = %+v, want adguard", cfg.Providers[0])
	}
	if cfg.Providers[1].Type != "pihole" || cfg.Providers[1].URL != "http://192.168.1.3" {
		t.Errorf("Providers[1] = %+v, want pihole", cfg.Providers[1])
	}
}

func TestApplyEnvOverrides_SliceOfStructsGap(t *testing.T) {
	// Indices must be contiguous — gap at index 1 stops scanning.
	cfg := &testCfg{}
	t.Setenv("TEST_PROVIDERS_0_TYPE", "adguard")
	t.Setenv("TEST_PROVIDERS_2_TYPE", "pihole") // gap: no index 1
	applyEnvOverrides(cfg, "TEST")
	if len(cfg.Providers) != 1 {
		t.Fatalf("Providers length = %d, want 1 (gap should stop scan)", len(cfg.Providers))
	}
}

func TestApplyEnvOverrides_EmptyPrefix(t *testing.T) {
	cfg := &testCfg{Name: "original"}
	t.Setenv("_NAME", "should-not-apply")
	applyEnvOverrides(cfg, "")
	if cfg.Name != "original" {
		t.Errorf("Name = %q, want %q (empty prefix should skip)", cfg.Name, "original")
	}
}

func TestLoad_OverridePath(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(cfgFile, []byte("name: from-file\ntimeout: 99\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := &testCfg{Name: "default", Timeout: 10}
	err := Load(cfg, Options{
		Name:         "test",
		OverridePath: cfgFile,
	})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Name != "from-file" {
		t.Errorf("Name = %q, want %q", cfg.Name, "from-file")
	}
	if cfg.Timeout != 99 {
		t.Errorf("Timeout = %d, want %d", cfg.Timeout, 99)
	}
}

func TestLoad_ExtraSearchDirs(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "myapp.yaml"), []byte("name: extra-dir\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := &testCfg{Name: "default"}
	err := Load(cfg, Options{
		Name:            "myapp",
		SystemDir:       filepath.Join(t.TempDir(), "nonexistent"),
		ExtraSearchDirs: []string{dir},
	})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Name != "extra-dir" {
		t.Errorf("Name = %q, want %q", cfg.Name, "extra-dir")
	}
}

func TestLoad_EnvOverrideAfterFile(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(cfgFile, []byte("name: from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("APP_NAME", "from-env")

	cfg := &testCfg{Name: "default"}
	err := Load(cfg, Options{
		Name:         "test",
		EnvPrefix:    "APP",
		OverridePath: cfgFile,
	})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Name != "from-env" {
		t.Errorf("Name = %q, want %q (env should override file)", cfg.Name, "from-env")
	}
}
