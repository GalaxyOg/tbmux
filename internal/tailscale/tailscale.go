package tailscale

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type Detection struct {
	Path   string `json:"path"`
	Method string `json:"method"`
}

var (
	lookPathFn         = exec.LookPath
	userHomeDirFn      = os.UserHomeDir
	isExecutableFn     = isExecutablePath
	commonCandidatesFn = commonCandidates
)

func Detect(override string) (Detection, error) {
	if strings.TrimSpace(override) != "" {
		p := expandHome(override)
		if isExecutableFn(p) {
			return Detection{Path: p, Method: "config_or_env_override"}, nil
		}
		return Detection{}, fmt.Errorf("tailscale override 不可执行: %s", p)
	}
	if p, err := lookPathFn("tailscale"); err == nil {
		return Detection{Path: p, Method: "path_lookup"}, nil
	}
	for _, p := range commonCandidatesFn() {
		if isExecutableFn(p) {
			m := "common_global_path"
			if strings.Contains(p, "/.local/") || strings.Contains(p, "/bin/") && strings.Contains(p, homePrefix()) {
				m = "common_user_path"
			}
			return Detection{Path: p, Method: m}, nil
		}
	}
	return Detection{}, errors.New("未找到 tailscale 可执行文件")
}

func Status(bin string) (string, error) {
	cmd := exec.Command(bin, "status")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), err
	}
	return string(out), nil
}

func ServeCommand(bin, url string) []string {
	if strings.TrimSpace(url) == "" {
		url = "http://127.0.0.1:6006"
	}
	return []string{bin, "serve", "--bg", url}
}

func RunServe(bin, url string) (string, error) {
	args := ServeCommand(bin, url)
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), err
	}
	return string(out), nil
}

func commonCandidates() []string {
	paths := []string{"/usr/bin/tailscale", "/usr/local/bin/tailscale", "/opt/homebrew/bin/tailscale"}
	home, err := userHomeDirFn()
	if err == nil {
		paths = append(paths,
			filepath.Join(home, ".local", "bin", "tailscale"),
			filepath.Join(home, "bin", "tailscale"),
		)
	}
	if runtime.GOOS == "windows" {
		return []string{"tailscale.exe"}
	}
	return paths
}

func isExecutablePath(path string) bool {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return false
	}
	return st.Mode()&0o111 != 0
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := userHomeDirFn()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func homePrefix() string {
	home, err := userHomeDirFn()
	if err != nil {
		return ""
	}
	return home
}
