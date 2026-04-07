package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteDefaultAndLoad(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	cfg, err := WriteDefault(configPath)
	if err != nil {
		t.Fatalf("WriteDefault failed: %v", err)
	}
	if cfg.TensorBoard.Port != 6006 {
		t.Fatalf("unexpected default port: %d", cfg.TensorBoard.Port)
	}
	if !cfg.Tailscale.AutoServe {
		t.Fatalf("expected tailscale.auto_serve default to true")
	}
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Managed.RunDir == "" || loaded.Managed.StatePath == "" {
		t.Fatalf("default managed path should not be empty")
	}
}

func TestEnvOverrideTailscaleBinary(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	if _, err := WriteDefault(configPath); err != nil {
		t.Fatalf("WriteDefault failed: %v", err)
	}
	t.Setenv("TBMUX_TAILSCALE_BIN", "/tmp/custom-tailscale")
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Tailscale.Binary != "/tmp/custom-tailscale" {
		t.Fatalf("env override not applied: %q", loaded.Tailscale.Binary)
	}
}

func TestValidatePort(t *testing.T) {
	cfg, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	cfg.TensorBoard.Port = 70000
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for invalid port")
	}
}

func TestDefaultConfigPathXDG(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	p, err := DefaultConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(dir, "tbmux", "config.toml")
	if p != expected {
		t.Fatalf("expected %s got %s", expected, p)
	}
}

func TestLoadRequiresExistingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("expected not-exist error, got %v", err)
	}
}

func TestSaveAndReload(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.toml")
	cfg, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	cfg.WatchedRoots = []WatchedRoot{{Path: "~/exp", Alias: "exp"}}
	if err := Save(p, cfg); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	got, err := Load(p)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(got.WatchedRoots) != 1 {
		t.Fatalf("expected 1 watched root, got %d", len(got.WatchedRoots))
	}
	if got.WatchedRoots[0].Alias != "exp" {
		t.Fatalf("unexpected alias: %s", got.WatchedRoots[0].Alias)
	}
}

func TestExpandHomePath(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	configPath := filepath.Join(dir, "config.toml")
	content := `
[managed]
run_dir = "~/data/runs"
state_path = "~/state/state.json"
pid_path = "~/state/tb.pid"
log_path = "~/state/tb.log"
cleanup_stale = true

[scan]
running_window_minutes = 10
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Managed.RunDir != filepath.Join(home, "data", "runs") {
		t.Fatalf("run_dir not expanded: %s", cfg.Managed.RunDir)
	}
}
