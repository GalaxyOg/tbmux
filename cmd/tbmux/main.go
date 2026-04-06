package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"tbmux/internal/app"
	"tbmux/internal/cliutil"
	"tbmux/internal/config"
	"tbmux/internal/doctor"
	"tbmux/internal/model"
	"tbmux/internal/process"
	"tbmux/internal/runs"
	"tbmux/internal/selection"
	"tbmux/internal/tailscale"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("tbmux", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	cfgPathFlag := fs.String("config", "", "配置文件路径")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	rest := fs.Args()
	if len(rest) == 0 {
		printRootUsage()
		return 0
	}
	configPath, err := app.ResolveConfigPath(*cfgPathFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	cmd := rest[0]
	sub := rest[1:]
	switch cmd {
	case "init":
		return cmdInit(configPath, sub)
	case "config":
		return cmdConfig(configPath, sub)
	case "sync":
		return cmdSync(configPath, sub)
	case "list":
		return cmdList(configPath, sub)
	case "selected":
		return cmdSelected(configPath, sub)
	case "select":
		return cmdSelect(configPath, sub)
	case "start":
		return cmdStart(configPath, sub)
	case "stop":
		return cmdStop(configPath)
	case "restart":
		if code := cmdStop(configPath); code != 0 {
			return code
		}
		return cmdStart(configPath, sub)
	case "status":
		return cmdStatus(configPath, sub)
	case "open":
		return cmdOpen(configPath)
	case "doctor":
		return cmdDoctor(configPath, sub)
	case "tailscale":
		return cmdTailscale(configPath, sub)
	case "help", "-h", "--help":
		printRootUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n", cmd)
		printRootUsage()
		return 2
	}
}

func printRootUsage() {
	fmt.Print(`tbmux - TensorBoard 多目录聚合管理 CLI

用法:
  tbmux [--config PATH] <command> [args]

核心命令:
  init
  sync [--apply]
  list [--today|--hours N|--days N|--running|--not-running|--under PATH|--match Q] [--json]
  selected list [--json]
  select clear|add|remove|by-filter|apply
  start|stop|restart|status
  doctor [--json]
  tailscale status|serve
  config path|example
`)
}

func loadCfgAndState(configPath string) (config.Config, model.State, error) {
	cfg, err := app.LoadConfig(configPath)
	if err != nil {
		return config.Config{}, model.State{}, err
	}
	if err := app.EnsureDirs(cfg); err != nil {
		return config.Config{}, model.State{}, err
	}
	st, err := app.LoadState(cfg)
	if err != nil {
		return config.Config{}, model.State{}, err
	}
	return cfg, st, nil
}

func cmdInit(configPath string, args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	force := fs.Bool("force", false, "覆盖已有配置")
	jsonOut := fs.Bool("json", false, "JSON 输出")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if _, err := os.Stat(configPath); err == nil && !*force {
		fmt.Fprintf(os.Stderr, "配置已存在: %s (使用 --force 覆盖)\n", configPath)
		return 1
	}
	cfg, err := config.WriteDefault(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := app.EnsureDirs(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	st := model.State{Version: 1, Discovered: []model.RunRecord{}, Selected: map[string]model.SelectionEntry{}, UpdatedAt: time.Now().UTC()}
	if err := app.SaveState(cfg, st); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if *jsonOut {
		_ = cliutil.PrintJSON(os.Stdout, map[string]any{"config_path": configPath, "state_path": cfg.Managed.StatePath, "run_dir": cfg.Managed.RunDir})
		return 0
	}
	fmt.Printf("初始化完成\nconfig: %s\nstate: %s\nrun_dir: %s\n", configPath, cfg.Managed.StatePath, cfg.Managed.RunDir)
	return 0
}

func cmdConfig(configPath string, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "用法: tbmux config path|example")
		return 2
	}
	switch args[0] {
	case "path":
		fmt.Println(configPath)
		return 0
	case "example":
		ex, err := config.ExampleTOML()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Print(ex)
		return 0
	default:
		fmt.Fprintln(os.Stderr, "用法: tbmux config path|example")
		return 2
	}
}

func cmdSync(configPath string, args []string) int {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	apply := fs.Bool("apply", false, "同步后应用 selected 到 symlink 目录")
	jsonOut := fs.Bool("json", false, "JSON 输出")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, st, err := loadCfgAndState(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	st, res, err := app.Sync(cfg, st)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	applied := 0
	if *apply {
		applied, err = app.ApplySelection(cfg, st)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	if err := app.SaveState(cfg, st); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if *jsonOut {
		payload := map[string]any{"discovered": res.Discovered, "selected": res.Selected, "pruned": res.Pruned, "applied": applied}
		_ = cliutil.PrintJSON(os.Stdout, payload)
		return 0
	}
	fmt.Printf("sync 完成: discovered=%d selected=%d pruned=%d", res.Discovered, res.Selected, res.Pruned)
	if *apply {
		fmt.Printf(" applied=%d", applied)
	}
	fmt.Println()
	return 0
}

func buildFilter(args []string) (model.Filter, bool, error) {
	fs := flag.NewFlagSet("filter", flag.ContinueOnError)
	today := fs.Bool("today", false, "仅当天更新")
	hours := fs.Int("hours", 0, "最近 N 小时")
	days := fs.Int("days", 0, "最近 N 天")
	running := fs.Bool("running", false, "仅 running")
	notRunning := fs.Bool("not-running", false, "仅 not-running")
	under := fs.String("under", "", "限定路径前缀")
	match := fs.String("match", "", "名称/id/path 模糊匹配")
	jsonOut := fs.Bool("json", false, "JSON 输出")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return model.Filter{}, false, err
	}
	if *running && *notRunning {
		return model.Filter{}, false, errors.New("--running 与 --not-running 不能同时使用")
	}
	var runningOnly *bool
	if *running {
		v := true
		runningOnly = &v
	}
	if *notRunning {
		v := false
		runningOnly = &v
	}
	f := model.Filter{Today: *today, Hours: *hours, Days: *days, RunningOnly: runningOnly, Under: *under, Match: *match}
	return f, *jsonOut, nil
}

func renderRunList(runs []model.RunRecord) {
	fmt.Printf("%-12s  %-8s  %-20s  %-40s  %s\n", "ID", "RUNNING", "UPDATED", "NAME", "SOURCE")
	for _, run := range runs {
		fmt.Printf("%-12s  %-8t  %-20s  %-40s  %s\n", run.ID, run.IsRunning, run.LastUpdatedAt.Format(time.RFC3339), run.Name, run.SourcePath)
	}
}

func cmdList(configPath string, args []string) int {
	filter, jsonOut, err := buildFilter(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	cfg, st, err := loadCfgAndState(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	_ = cfg
	runs := selection.ApplyFilter(st.Discovered, filter, time.Now())
	if jsonOut {
		_ = cliutil.PrintJSON(os.Stdout, runs)
		return 0
	}
	renderRunList(runs)
	fmt.Printf("总计: %d\n", len(runs))
	return 0
}

func cmdSelected(configPath string, args []string) int {
	if len(args) == 0 || args[0] != "list" {
		fmt.Fprintln(os.Stderr, "用法: tbmux selected list [--json]")
		return 2
	}
	fs := flag.NewFlagSet("selected list", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "JSON 输出")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	_, st, err := loadCfgAndState(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	runs := selection.SelectedRuns(st)
	if *jsonOut {
		_ = cliutil.PrintJSON(os.Stdout, runs)
		return 0
	}
	renderRunList(runs)
	fmt.Printf("selected: %d\n", len(runs))
	return 0
}

func cmdSelect(configPath string, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "用法: tbmux select clear|add|remove|by-filter|apply")
		return 2
	}
	cfg, st, err := loadCfgAndState(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	action := args[0]
	subArgs := args[1:]
	switch action {
	case "clear":
		selection.Clear(&st)
		if err := app.SaveState(cfg, st); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println("已清空 selected")
		return 0
	case "add":
		added, err := selection.AddByTokens(&st, subArgs, "manual_add")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if err := app.SaveState(cfg, st); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Printf("新增 selected: %d\n", added)
		return 0
	case "remove":
		removed, err := selection.RemoveByTokens(&st, subArgs)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if err := app.SaveState(cfg, st); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Printf("移除 selected: %d\n", removed)
		return 0
	case "by-filter":
		fs := flag.NewFlagSet("select by-filter", flag.ContinueOnError)
		today := fs.Bool("today", false, "仅当天更新")
		hours := fs.Int("hours", 0, "最近 N 小时")
		days := fs.Int("days", 0, "最近 N 天")
		running := fs.Bool("running", false, "仅 running")
		notRunning := fs.Bool("not-running", false, "仅 not-running")
		under := fs.String("under", "", "限定路径前缀")
		match := fs.String("match", "", "名称/id/path 模糊匹配")
		removeMode := fs.Bool("remove", false, "按筛选批量移除")
		setMode := fs.Bool("set", false, "按筛选覆盖 selected")
		fs.SetOutput(os.Stderr)
		if err := fs.Parse(subArgs); err != nil {
			return 2
		}
		if *running && *notRunning {
			fmt.Fprintln(os.Stderr, "--running 与 --not-running 不能同时使用")
			return 2
		}
		if *removeMode && *setMode {
			fmt.Fprintln(os.Stderr, "--remove 与 --set 不能同时使用")
			return 2
		}
		var runningOnly *bool
		if *running {
			v := true
			runningOnly = &v
		}
		if *notRunning {
			v := false
			runningOnly = &v
		}
		mode := "add"
		if *removeMode {
			mode = "remove"
		}
		if *setMode {
			mode = "set"
		}
		f := model.Filter{Today: *today, Hours: *hours, Days: *days, RunningOnly: runningOnly, Under: *under, Match: *match}
		changed := selection.SelectByFilter(&st, f, mode)
		if err := app.SaveState(cfg, st); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Printf("按筛选更新 selected: %d (mode=%s)\n", changed, mode)
		return 0
	case "apply":
		applied, err := app.ApplySelection(cfg, st)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Printf("已应用 selected 到 %s, symlink=%d\n", runs.SelectedDir(cfg.Managed.RunDir), applied)
		return 0
	default:
		fmt.Fprintln(os.Stderr, "用法: tbmux select clear|add|remove|by-filter|apply")
		return 2
	}
}

func cmdStart(configPath string, args []string) int {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	noSync := fs.Bool("no-sync", false, "启动前不执行 sync")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, st, err := loadCfgAndState(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !*noSync {
		st, _, err = app.Sync(cfg, st)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if err := app.SaveState(cfg, st); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	if _, err := app.ApplySelection(cfg, st); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	pid, err := process.Start(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("tensorboard 已启动 pid=%d\n", pid)
	fmt.Printf("URL: http://%s:%d\n", cfg.TensorBoard.Host, cfg.TensorBoard.Port)
	return 0
}

func cmdStop(configPath string) int {
	cfg, err := app.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := process.Stop(cfg.Managed.PidPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Println("tensorboard 未运行")
			return 0
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println("tensorboard 已停止")
	return 0
}

func cmdStatus(configPath string, args []string) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "JSON 输出")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, st, err := loadCfgAndState(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	running, pid, err := process.Status(cfg.Managed.PidPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	status := map[string]any{
		"running":      running,
		"pid":          pid,
		"url":          fmt.Sprintf("http://%s:%d", cfg.TensorBoard.Host, cfg.TensorBoard.Port),
		"discovered":   len(st.Discovered),
		"selected":     len(st.Selected),
		"selected_dir": runs.SelectedDir(cfg.Managed.RunDir),
		"pid_path":     cfg.Managed.PidPath,
		"log_path":     cfg.Managed.LogPath,
	}
	if *jsonOut {
		_ = cliutil.PrintJSON(os.Stdout, status)
		return 0
	}
	fmt.Printf("running: %t\n", running)
	fmt.Printf("pid: %d\n", pid)
	fmt.Printf("url: %s\n", status["url"])
	fmt.Printf("discovered: %d\n", len(st.Discovered))
	fmt.Printf("selected: %d\n", len(st.Selected))
	fmt.Printf("selected_dir: %s\n", status["selected_dir"])
	return 0
}

func cmdOpen(configPath string) int {
	cfg, err := app.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("http://%s:%d\n", cfg.TensorBoard.Host, cfg.TensorBoard.Port)
	return 0
}

func cmdDoctor(configPath string, args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "JSON 输出")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, err := app.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	report := doctor.Run(cfg)
	if *jsonOut {
		_ = cliutil.PrintJSON(os.Stdout, report)
		return 0
	}
	for _, it := range report.Items {
		fmt.Printf("[%s] %-28s %s\n", strings.ToUpper(it.Status), it.Name, it.Message)
	}
	return 0
}

func cmdTailscale(configPath string, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "用法: tbmux tailscale status|serve")
		return 2
	}
	cfg, err := app.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	switch args[0] {
	case "status":
		fs := flag.NewFlagSet("tailscale status", flag.ContinueOnError)
		jsonOut := fs.Bool("json", false, "JSON 输出")
		fs.SetOutput(os.Stderr)
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		det, err := tailscale.Detect(cfg.Tailscale.Binary)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		out, statErr := tailscale.Status(det.Path)
		if *jsonOut {
			payload := map[string]any{"binary": det.Path, "method": det.Method, "status_output": out}
			if statErr != nil {
				payload["status_error"] = statErr.Error()
			}
			_ = cliutil.PrintJSON(os.Stdout, payload)
			if statErr != nil {
				return 1
			}
			return 0
		}
		fmt.Printf("tailscale binary: %s (%s)\n", det.Path, det.Method)
		fmt.Print(out)
		if statErr != nil {
			fmt.Fprintln(os.Stderr, statErr)
			return 1
		}
		return 0
	case "serve":
		fs := flag.NewFlagSet("tailscale serve", flag.ContinueOnError)
		dryRun := fs.Bool("dry-run", false, "仅打印命令")
		jsonOut := fs.Bool("json", false, "JSON 输出")
		fs.SetOutput(os.Stderr)
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		det, err := tailscale.Detect(cfg.Tailscale.Binary)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		cmd := tailscale.ServeCommand(det.Path, cfg.Tailscale.ServeURL)
		if *dryRun {
			if *jsonOut {
				_ = cliutil.PrintJSON(os.Stdout, map[string]any{"dry_run": true, "command": cmd})
				return 0
			}
			fmt.Printf("dry-run: %s\n", strings.Join(cmd, " "))
			return 0
		}
		out, err := tailscale.RunServe(det.Path, cfg.Tailscale.ServeURL)
		if *jsonOut {
			payload := map[string]any{"command": cmd, "output": out}
			if err != nil {
				payload["error"] = err.Error()
				_ = cliutil.PrintJSON(os.Stdout, payload)
				return 1
			}
			_ = cliutil.PrintJSON(os.Stdout, payload)
			return 0
		}
		fmt.Print(out)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	default:
		fmt.Fprintln(os.Stderr, "用法: tbmux tailscale status|serve")
		return 2
	}
}
