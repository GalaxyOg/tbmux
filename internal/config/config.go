package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	WatchedRoots    []WatchedRoot `toml:"watched_roots"`
	ExcludePatterns []string      `toml:"exclude_patterns"`
	TensorBoard     TensorBoard   `toml:"tensorboard"`
	Managed         Managed       `toml:"managed"`
	Scan            Scan          `toml:"scan"`
	Tailscale       Tailscale     `toml:"tailscale"`
}

type WatchedRoot struct {
	Path  string `toml:"path"`
	Alias string `toml:"alias"`
}

type TensorBoard struct {
	Binary    string   `toml:"binary"`
	Host      string   `toml:"host"`
	Port      int      `toml:"port"`
	ExtraArgs []string `toml:"extra_args"`
}

type Managed struct {
	RunDir       string `toml:"run_dir"`
	StatePath    string `toml:"state_path"`
	PidPath      string `toml:"pid_path"`
	LogPath      string `toml:"log_path"`
	CleanupStale bool   `toml:"cleanup_stale"`
}

type Scan struct {
	RunningWindowMinutes int `toml:"running_window_minutes"`
}

type Tailscale struct {
	Binary   string `toml:"binary"`
	ServeURL string `toml:"serve_url"`
}

func DefaultConfigPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "tbmux", "config.toml"), nil
}

func defaultPaths() (runDir, statePath, pidPath, logPath string, err error) {
	dataHome := os.Getenv("XDG_DATA_HOME")
	stateHome := os.Getenv("XDG_STATE_HOME")
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", "", "", err
	}
	if dataHome == "" {
		dataHome = filepath.Join(home, ".local", "share")
	}
	if stateHome == "" {
		stateHome = filepath.Join(home, ".local", "state")
	}
	runDir = filepath.Join(dataHome, "tbmux", "runs")
	statePath = filepath.Join(stateHome, "tbmux", "state.json")
	pidPath = filepath.Join(stateHome, "tbmux", "tensorboard.pid")
	logPath = filepath.Join(stateHome, "tbmux", "tensorboard.log")
	return runDir, statePath, pidPath, logPath, nil
}

func Default() (Config, error) {
	runDir, statePath, pidPath, logPath, err := defaultPaths()
	if err != nil {
		return Config{}, err
	}
	return Config{
		WatchedRoots:    []WatchedRoot{},
		ExcludePatterns: []string{"*/.git/*", "*/__pycache__/*"},
		TensorBoard: TensorBoard{
			Binary:    "",
			Host:      "127.0.0.1",
			Port:      6006,
			ExtraArgs: []string{},
		},
		Managed: Managed{
			RunDir:       runDir,
			StatePath:    statePath,
			PidPath:      pidPath,
			LogPath:      logPath,
			CleanupStale: true,
		},
		Scan: Scan{
			RunningWindowMinutes: 15,
		},
		Tailscale: Tailscale{
			Binary:   "",
			ServeURL: "http://127.0.0.1:6006",
		},
	}, nil
}

func ensureParent(path string) error {
	if path == "" {
		return errors.New("empty path")
	}
	return os.MkdirAll(filepath.Dir(path), 0o755)
}

func WriteDefault(path string) (Config, error) {
	cfg, err := Default()
	if err != nil {
		return Config{}, err
	}
	if err := ensureParent(path); err != nil {
		return Config{}, err
	}
	f, err := os.Create(path)
	if err != nil {
		return Config{}, err
	}
	defer f.Close()
	enc := toml.NewEncoder(f)
	if err := enc.Encode(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Load(path string) (Config, error) {
	cfg, err := Default()
	if err != nil {
		return Config{}, err
	}
	if _, err := os.Stat(path); err != nil {
		return Config{}, err
	}
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return Config{}, fmt.Errorf("解析配置失败: %w", err)
	}
	overrideFromEnv(&cfg)
	cfg.normalizePaths()
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func overrideFromEnv(cfg *Config) {
	if v := strings.TrimSpace(os.Getenv("TBMUX_TAILSCALE_BIN")); v != "" {
		cfg.Tailscale.Binary = v
	}
}

func (c *Config) normalizePaths() {
	for i := range c.WatchedRoots {
		c.WatchedRoots[i].Path = expandPath(c.WatchedRoots[i].Path)
	}
	c.Managed.RunDir = expandPath(c.Managed.RunDir)
	c.Managed.StatePath = expandPath(c.Managed.StatePath)
	c.Managed.PidPath = expandPath(c.Managed.PidPath)
	c.Managed.LogPath = expandPath(c.Managed.LogPath)
	c.Tailscale.Binary = expandPath(c.Tailscale.Binary)
}

func (c Config) Validate() error {
	if c.TensorBoard.Port <= 0 || c.TensorBoard.Port > 65535 {
		return fmt.Errorf("tensorboard.port 非法: %d", c.TensorBoard.Port)
	}
	if strings.TrimSpace(c.Managed.RunDir) == "" {
		return errors.New("managed.run_dir 不能为空")
	}
	if strings.TrimSpace(c.Managed.StatePath) == "" {
		return errors.New("managed.state_path 不能为空")
	}
	if strings.TrimSpace(c.Managed.PidPath) == "" {
		return errors.New("managed.pid_path 不能为空")
	}
	if c.Scan.RunningWindowMinutes <= 0 {
		return errors.New("scan.running_window_minutes 必须大于 0")
	}
	for i, root := range c.WatchedRoots {
		if strings.TrimSpace(root.Path) == "" {
			return fmt.Errorf("watched_roots[%d].path 不能为空", i)
		}
	}
	return nil
}

func ExampleTOML() (string, error) {
	cfg, err := Default()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := toml.NewEncoder(&b).Encode(cfg); err != nil {
		return "", err
	}
	return b.String(), nil
}

func expandPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return path
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
