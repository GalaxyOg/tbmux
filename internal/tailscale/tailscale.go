package tailscale

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
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
	commandContextFn   = exec.CommandContext
	statusTimeout      = 5 * time.Second
	serveTimeout       = 8 * time.Second
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
	return runWithTimeout(statusTimeout, bin, "status")
}

func ServeCommand(bin, url string) []string {
	if strings.TrimSpace(url) == "" {
		url = "http://127.0.0.1:6006"
	}
	return []string{bin, "serve", "--bg", url}
}

func RunServe(bin, url string) (string, error) {
	args := ServeCommand(bin, url)
	out, err, timedOut := runWithTimeoutState(serveTimeout, args[0], args[1:]...)
	if timedOut {
		statusOut, statErr := ServeStatus(bin)
		if statErr == nil && ParseServeURL(statusOut) != "" {
			msg := strings.TrimSpace(out)
			if msg != "" {
				msg += "\n"
			}
			msg += "tailscale serve 命令超时，但已检测到可用 tailnet 入口:\n" + statusOut
			return msg, nil
		}
		if statErr != nil {
			return out, fmt.Errorf("tailscale serve 超时，且读取 serve status 失败: %w", statErr)
		}
		return out, errors.New("tailscale serve 超时，未检测到可用 tailnet 入口")
	}
	return out, err
}

func ServeOffCommand(bin string) []string {
	return []string{bin, "serve", "--https=443", "off"}
}

func RunServeOff(bin string) (string, error) {
	args := ServeOffCommand(bin)
	out, err, _ := runWithTimeoutState(serveTimeout, args[0], args[1:]...)
	return out, err
}

func ServeStatus(bin string) (string, error) {
	return runWithTimeout(statusTimeout, bin, "serve", "status")
}

func ParseServeURL(output string) string {
	re := regexp.MustCompile(`https?://[^\s]+`)
	m := re.FindString(output)
	return strings.TrimSpace(m)
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

func runWithTimeout(timeout time.Duration, bin string, args ...string) (string, error) {
	out, err, _ := runWithTimeoutState(timeout, bin, args...)
	return out, err
}

func runWithTimeoutState(timeout time.Duration, bin string, args ...string) (string, error, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := commandContextFn(ctx, bin, args...)
	out, err := cmd.CombinedOutput()
	outStr := string(out)
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return outStr, fmt.Errorf("命令超时: %s %s", bin, strings.Join(args, " ")), true
	}
	if err != nil {
		return outStr, err, false
	}
	return outStr, nil, false
}
