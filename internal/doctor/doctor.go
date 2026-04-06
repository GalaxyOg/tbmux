package doctor

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"tbmux/internal/config"
	"tbmux/internal/model"
	"tbmux/internal/process"
	"tbmux/internal/selection"
	"tbmux/internal/state"
	"tbmux/internal/tailscale"
)

func Run(cfg config.Config) model.DoctorReport {
	report := model.DoctorReport{CheckedAt: time.Now().UTC(), Items: []model.DoctorItem{}}

	appendItem := func(name, status, msg string) {
		report.Items = append(report.Items, model.DoctorItem{Name: name, Status: status, Message: msg})
	}

	if tb, err := process.DetectTensorBoard(cfg.TensorBoard.Binary); err != nil {
		appendItem("tensorboard.binary", "error", err.Error())
	} else {
		appendItem("tensorboard.binary", "ok", fmt.Sprintf("%s (%s)", tb.Path, tb.Method))
	}

	if ts, err := tailscale.Detect(cfg.Tailscale.Binary); err != nil {
		appendItem("tailscale.binary", "warn", err.Error())
	} else {
		appendItem("tailscale.binary", "ok", fmt.Sprintf("%s (%s)", ts.Path, ts.Method))
	}

	if len(cfg.WatchedRoots) == 0 {
		appendItem("watched_roots", "warn", "未配置 watched_roots")
	}
	for _, root := range cfg.WatchedRoots {
		if st, err := os.Stat(root.Path); err != nil || !st.IsDir() {
			appendItem("watch_root", "error", fmt.Sprintf("不可用: %s", root.Path))
		} else {
			appendItem("watch_root", "ok", root.Path)
		}
	}

	if err := os.MkdirAll(cfg.Managed.RunDir, 0o755); err != nil {
		appendItem("managed.run_dir", "error", err.Error())
	} else {
		tf := filepath.Join(cfg.Managed.RunDir, ".write_test")
		if err := os.WriteFile(tf, []byte("ok"), 0o644); err != nil {
			appendItem("managed.run_dir", "error", "目录不可写")
		} else {
			_ = os.Remove(tf)
			appendItem("managed.run_dir", "ok", cfg.Managed.RunDir)
		}
	}

	addr := fmt.Sprintf("%s:%d", cfg.TensorBoard.Host, cfg.TensorBoard.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		appendItem("tensorboard.port", "warn", fmt.Sprintf("端口可能已占用: %s", addr))
	} else {
		_ = ln.Close()
		appendItem("tensorboard.port", "ok", fmt.Sprintf("端口可用: %s", addr))
	}

	if err := checkSymlink(cfg.Managed.RunDir); err != nil {
		appendItem("symlink", "warn", err.Error())
	} else {
		appendItem("symlink", "ok", "symlink 创建正常")
	}

	st, err := state.Load(cfg.Managed.StatePath)
	if err != nil {
		appendItem("state.read", "error", err.Error())
	} else {
		appendItem("state.read", "ok", fmt.Sprintf("discovered=%d selected=%d", len(st.Discovered), len(st.Selected)))
		pruned := selection.PruneSelected(&st)
		if pruned > 0 {
			appendItem("state.selection_consistency", "warn", fmt.Sprintf("存在 %d 个失效 selected id", pruned))
		} else {
			appendItem("state.selection_consistency", "ok", "selected 与 discovered 一致")
		}
	}

	if running, pid, err := process.Status(cfg.Managed.PidPath); err != nil {
		appendItem("tensorboard.process", "error", err.Error())
	} else if running {
		appendItem("tensorboard.process", "ok", fmt.Sprintf("运行中 pid=%d", pid))
	} else {
		appendItem("tensorboard.process", "warn", "未运行")
	}

	return report
}

func checkSymlink(runDir string) error {
	tmpDir := filepath.Join(runDir, ".doctor_tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	src := filepath.Join(tmpDir, "src")
	ln := filepath.Join(tmpDir, "ln")
	if err := os.WriteFile(src, []byte("ok"), 0o644); err != nil {
		return err
	}
	if err := os.Symlink(src, ln); err != nil {
		return fmt.Errorf("symlink 创建失败: %w", err)
	}
	return nil
}
