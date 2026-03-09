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

type testCfg struct {
	Name    string        `yaml:"name"`
	Debug   bool          `yaml:"debug"`
	Tags    []string      `yaml:"tags"`
	Server  testServerCfg `yaml:"server"`
	Timeout int           `yaml:"timeout"`
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
	if err := os.WriteFile(cfgFile, []byte("name: from-file\ntimeout: 99\n"), 0o644); err != nil {
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
	if err := os.WriteFile(filepath.Join(dir, "myapp.yaml"), []byte("name: extra-dir\n"), 0o644); err != nil {
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
	if err := os.WriteFile(cfgFile, []byte("name: from-file\n"), 0o644); err != nil {
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
