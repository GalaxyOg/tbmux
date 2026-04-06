package process

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"tbmux/internal/config"
	"tbmux/internal/runs"
)

type TBDetection struct {
	Path   string `json:"path"`
	Method string `json:"method"`
}

func DetectTensorBoard(override string) (TBDetection, error) {
	if strings.TrimSpace(override) != "" {
		if isExecutable(override) {
			return TBDetection{Path: override, Method: "config_override"}, nil
		}
		return TBDetection{}, fmt.Errorf("tensorboard binary 不可执行: %s", override)
	}
	if p, err := exec.LookPath("tensorboard"); err == nil {
		return TBDetection{Path: p, Method: "path_lookup"}, nil
	}
	for _, p := range []string{"/usr/bin/tensorboard", "/usr/local/bin/tensorboard"} {
		if isExecutable(p) {
			return TBDetection{Path: p, Method: "common_global_path"}, nil
		}
	}
	return TBDetection{}, errors.New("未找到 tensorboard 可执行文件")
}

func Start(cfg config.Config) (int, error) {
	running, pid, _ := Status(cfg.Managed.PidPath)
	if running {
		return pid, fmt.Errorf("tensorboard 已在运行, pid=%d", pid)
	}
	tb, err := DetectTensorBoard(cfg.TensorBoard.Binary)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Managed.PidPath), 0o755); err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Managed.LogPath), 0o755); err != nil {
		return 0, err
	}
	logf, err := os.OpenFile(cfg.Managed.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, err
	}
	defer logf.Close()

	args := []string{
		"--logdir", runs.SelectedDir(cfg.Managed.RunDir),
		"--host", cfg.TensorBoard.Host,
		"--port", strconv.Itoa(cfg.TensorBoard.Port),
	}
	args = append(args, cfg.TensorBoard.ExtraArgs...)

	cmd := exec.Command(tb.Path, args...)
	cmd.Stdout = logf
	cmd.Stderr = logf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid = cmd.Process.Pid
	if err := os.WriteFile(cfg.Managed.PidPath, []byte(strconv.Itoa(pid)), 0o644); err != nil {
		_ = cmd.Process.Kill()
		return 0, err
	}
	return pid, nil
}

func Stop(pidPath string) error {
	pid, err := readPID(pidPath)
	if err != nil {
		return err
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	_ = os.Remove(pidPath)
	return nil
}

func Status(pidPath string) (running bool, pid int, err error) {
	pid, err = readPID(pidPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, 0, nil
		}
		return false, 0, err
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, pid, err
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		_ = os.Remove(pidPath)
		return false, pid, nil
	}
	return true, pid, nil
}

func readPID(pidPath string) (int, error) {
	b, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0, fmt.Errorf("pid 文件损坏: %w", err)
	}
	return pid, nil
}

func isExecutable(path string) bool {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return false
	}
	return st.Mode()&0o111 != 0
}
