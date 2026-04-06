# tbmux

`tbmux` 是一个面向 Linux 服务器的 TensorBoard 聚合 CLI：扫描多个训练目录，维护 `selected runs` 的 symlink 聚合目录，并用固定地址启动 TensorBoard，便于通过 Tailscale 访问。

## 适用场景

- 多个训练目录分散在不同路径
- 不想反复手敲 `tensorboard --logdir ...`
- 希望固定入口持续查看训练
- 需要把访问控制在 tailnet（Tailscale）里

## 安装（其他用户）

### 0. 前置条件

- Linux
- 已安装 TensorBoard（`tensorboard` 命令可用）
- 可选：已安装 Tailscale（用于 tailnet 暴露）

### 1. 安装 Go（非 root）

仓库已提供脚本：

```bash
bash scripts/install_go_user.sh
source ~/.bashrc
go version
```

说明：脚本会自动从 `go.dev` 拉取最新稳定版，安装到 `~/.local/go`，并写入 `~/.bashrc`。

### 2. 安装 tbmux

方式 A：在仓库内编译并放到 `~/.local/bin`

```bash
mkdir -p ~/.local/bin
$HOME/.local/go/bin/go build -o ~/.local/bin/tbmux ./cmd/tbmux
~/.local/bin/tbmux --help
```

方式 B：直接 `go install`

```bash
$HOME/.local/go/bin/go install ./cmd/tbmux
~/go/bin/tbmux --help
```

## 首次使用

### 1. 初始化配置

```bash
tbmux init
```

默认配置路径：`~/.config/tbmux/config.toml`

可先导出示例：

```bash
tbmux config example
```

### 2. 编辑 watched roots

编辑 `~/.config/tbmux/config.toml`，至少配置：

```toml
[[watched_roots]]
path = "/data/trainings"
alias = "trainings"

[[watched_roots]]
path = "/mnt/experiments"
alias = "experiments"
```

### 3. 发现 run 并选择展示集合

```bash
tbmux sync
tbmux list --running
tbmux select by-filter --running --set
tbmux select apply
```

### 4. 启动 TensorBoard

```bash
tbmux start
tbmux status
tbmux open
```

### 5. Tailscale 暴露（可选）

```bash
tbmux tailscale status
tbmux tailscale serve --dry-run
# 确认命令后再执行
tbmux tailscale serve
```

## 关键命令

- `tbmux init`
- `tbmux sync [--apply] [--json]`
- `tbmux list [--today|--hours N|--days N|--running|--not-running|--under PATH|--match Q] [--json]`
- `tbmux selected list [--json]`
- `tbmux select clear`
- `tbmux select add <id|name>...`
- `tbmux select remove <id|name>...`
- `tbmux select by-filter [--today|--hours N|--days N|--running|--not-running|--under PATH|--match Q] [--set|--remove]`
- `tbmux select apply`
- `tbmux start [--no-sync]`
- `tbmux stop`
- `tbmux restart`
- `tbmux status [--json]`
- `tbmux doctor [--json]`
- `tbmux open`
- `tbmux tailscale status [--json]`
- `tbmux tailscale serve [--dry-run] [--json]`
- `tbmux config path|example`

## 配置文件

默认路径：`~/.config/tbmux/config.toml`（可用 `--config` 覆盖）

关键字段：

- `watched_roots`: 扫描根目录（支持 alias）
- `exclude_patterns`: 跳过路径模式
- `tensorboard.binary/host/port/extra_args`
- `managed.run_dir/state_path/pid_path/log_path/cleanup_stale`
- `scan.running_window_minutes`: running 推断窗口
- `tailscale.binary`: 手动覆盖二进制路径
- `tailscale.serve_url`: `tailscale serve` 目标 URL

示例见 [examples/config.toml](/mnt/share/YH/openclaw-workspace/tbmux/examples/config.toml)。

## Tailscale 检测策略

检测顺序：

1. 配置或环境变量覆盖（`tailscale.binary` / `TBMUX_TAILSCALE_BIN`）
2. `PATH` 查找 `tailscale`
3. 常见全局路径（`/usr/bin`、`/usr/local/bin`）
4. 常见用户路径（`~/.local/bin`、`~/bin`）

可通过 `tbmux tailscale status` 查看当前使用的可执行文件路径与检测来源。

## systemd 示例

仓库提供：

- user service: [tbmux.service](/mnt/share/YH/openclaw-workspace/tbmux/systemd/tbmux.service)
- sync service: [tbmux-sync.service](/mnt/share/YH/openclaw-workspace/tbmux/systemd/tbmux-sync.service)
- sync timer: [tbmux-sync.timer](/mnt/share/YH/openclaw-workspace/tbmux/systemd/tbmux-sync.timer)
- system service（可选）: [tbmux-system.service](/mnt/share/YH/openclaw-workspace/tbmux/systemd/tbmux-system.service)

示例（user service）：

```bash
mkdir -p ~/.config/systemd/user
cp systemd/tbmux.service ~/.config/systemd/user/
cp systemd/tbmux-sync.service ~/.config/systemd/user/
cp systemd/tbmux-sync.timer ~/.config/systemd/user/

systemctl --user daemon-reload
systemctl --user enable --now tbmux.service
systemctl --user enable --now tbmux-sync.timer
```

## 自动化与 JSON

适合脚本/agent/hook：

- `tbmux sync --json`
- `tbmux list --json`
- `tbmux status --json`
- `tbmux doctor --json`
- `tbmux tailscale status --json`

## 测试

执行：

```bash
go test ./...
```

在受限环境下可指定缓存目录：

```bash
GOCACHE=/tmp/go-build GOPATH=/tmp/go GOMODCACHE=/tmp/go/pkg/mod GOPROXY=https://goproxy.cn,direct go test ./...
```

当前仓库已通过 `go test ./...`。

## 仓库结构

- `cmd/tbmux`: CLI 入口
- `internal/config`: TOML 配置加载与默认值
- `internal/discovery`: event 发现、run 命名、running 推断
- `internal/selection`: 过滤与 selected 集合管理
- `internal/runs`: symlink 聚合目录应用
- `internal/process`: TensorBoard 进程管理（PID/日志）
- `internal/tailscale`: tailscale 二进制检测与辅助命令
- `internal/doctor`: 健康检查
- `examples/config.toml`: 示例配置
- `docs/design.md`: 设计说明
- `scripts/install_go_user.sh`: 非 root Go 安装脚本
