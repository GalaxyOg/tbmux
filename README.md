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

### 1. 无需 Go 的安装方式（推荐）

直接安装预编译二进制（不需要 Go 工具链）：

```bash
bash scripts/install_tbmux_binary.sh
tbmux version
```

也可以远程执行（适合没有 git 的机器）：

```bash
curl -fsSL https://raw.githubusercontent.com/GalaxyOg/tbmux/master/scripts/install_tbmux_binary.sh | bash
```

可选参数：

- `TBMUX_REPO=GalaxyOg/tbmux`（默认）
- `TBMUX_VERSION=latest` 或固定版本（例如 `v0.2.0`）
- `TBMUX_PREFIX=$HOME/.local`（安装前缀）

### 2. 需要源码构建时，再安装 Go（非 root）

仓库已提供脚本：

```bash
bash scripts/install_go_user.sh
source ~/.bashrc
go version
```

说明：脚本会自动从 `go.dev` 拉取最新稳定版，安装到 `~/.local/go`，并写入 `~/.bashrc`。

### 3. 源码构建安装 tbmux

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

### 2.1 设置 TensorBoard 可执行文件路径（重点）

如果 `tbmux start` 报 `未找到 tensorboard 可执行文件`，请手动配置绝对路径：

1. 查找路径

```bash
which tensorboard
# 如果在 conda 环境里：
conda run -n <env_name> which tensorboard
```

2. 写入配置

```toml
[tensorboard]
binary = "/home/xxx/anaconda3/envs/<env_name>/bin/tensorboard"
host = "127.0.0.1"
port = 6006
extra_args = []
```

说明：

- `binary=""` 时按 PATH/常见系统路径自动查找
- conda 场景建议显式写绝对路径，最稳定

### 3. 发现 run 并选择展示集合

```bash
tbmux sync
tbmux list --running
tbmux select by-filter --running --set
tbmux select apply
```

常见过滤示例：

```bash
# 仅最近 12 小时
tbmux list --hours 12

# 仅 running 且位于某路径
tbmux list --running --under /home/yh/Algo_test/ReinFlow

# 按关键词筛选并设置为当前展示集合
tbmux select by-filter --match ppo --set

# 按路径筛选加入展示集合
tbmux select by-filter --under /home/yh/Algo_test/ReinFlow
```

### 4. 启动 TensorBoard

```bash
tbmux start
tbmux status
tbmux open
```

说明：`tbmux start` 和 `tbmux open` 现在会同时输出：

- 本机访问地址（127.0.0.1）
- 局域网候选地址（本机网卡 IP）
- 若已开启 `tailscale serve`，还会输出 tailnet URL

### 5. Tailscale 暴露（可选）

```bash
tbmux tailscale status
tbmux tailscale serve --dry-run
# 确认命令后再执行
tbmux tailscale serve
```

说明：

- `tbmux tailscale serve` 带超时保护，避免异常情况下长时间卡住
- 如果命令超时但 `serve status` 已显示 tailnet 入口，`tbmux` 会按成功处理并给出地址提示

## 关键命令

- `tbmux`（交互终端中默认进入 TUI）
- `tbmux init`
- `tbmux sync [--apply] [--json]`
- `tbmux tui`（交互式：筛选/浏览/选择/apply）
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

`status/list/doctor` 等人类可读输出默认带颜色；`--json` 输出不带颜色，适合脚本。

## TUI 交互

执行：

```bash
tbmux tui
```

快捷键：

- `j/k` 或方向键：上下移动
- `←/→`：左栏中对当前长名称做水平滚动查看
- `space`：切换当前项 selected
- `x`：清空全部 draft selected
- `/`：搜索
- `r`：running 筛选轮换
- `t`：today 开关
- `c`：清空筛选
- `s`：sync discovered
- `a`：apply
- `g`：手动开启/关闭 tailscale serve
- `m`：切换 `tailscale.auto_serve`（会写回配置文件）
- `q`：退出（dirty 时二次确认）
- `?`：帮助

当前 TUI 可直接完成：

- 设置过滤条件（today/hours/days/running/under/match）
- 查看过滤后的 discovered runs
- 查看当前 selected（正在展示集合）
- 光标滚动跟随（长列表下移时窗口自动跟随）
- 始终双栏（左列表 + 右详情）
- 左栏长名称可用左右键查看完整内容
- 一键清空全部 draft selected（`x`）
- apply selected 到 symlink 暴露目录（`a`）
- 右栏显示 TensorBoard/Tailscale 状态与 tailnet 地址

TUI 与 CLI 的关系：

- TUI 中 `space` 只修改内存里的 draft selected（不立即落盘）
- TUI 中 `x` 可一键清空所有 draft selected
- 按 `a` 才会等价执行“保存 selected + apply 到 symlink”
- 等价 CLI 流程：`tbmux select ...` + `tbmux select apply`
- 若有未 apply 变更，`q` 会先提示，再次 `q` 才退出并丢弃草稿

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
- `tailscale.auto_serve`: TUI 启动时自动确保 `tailscale serve` 可用（默认 `true`）

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

## 维护者发布（二进制）

仓库已提供自动发布流程：`.github/workflows/release.yml`

发布方式：

1. 打 tag 并推送（例如 `v0.2.0`）
2. GitHub Actions 自动构建并发布 release 资产
3. 用户可用 `scripts/install_tbmux_binary.sh` 直接安装对应版本

本地手工打包（可选）：

```bash
VERSION=v0.2.0 bash scripts/build_release.sh
```

输出：

- `dist/tbmux_<version>_linux_amd64.tar.gz`
- `dist/tbmux_<version>_linux_arm64.tar.gz`
- `dist/sha256sums.txt`

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
- `scripts/install_tbmux_binary.sh`: 预编译二进制安装脚本（无需 Go）
- `scripts/build_release.sh`: release 打包脚本
