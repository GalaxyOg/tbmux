# tbmux 设计说明（MVP）

## 关键取舍

1. TensorBoard 进程管理：MVP 内置 `start/stop/status`，并提供 systemd 示例。日常建议 systemd 托管，CLI 负责运维动作。
2. Tailscale serve：保留 `tailscale serve` 执行能力，但推荐先 `--dry-run` 明确命令。
3. run 命名策略：`可读名 + 冲突后缀`。基础名来自 `alias + 相对路径`，冲突追加短 ID，兼顾可读与稳定。
4. discovery 缓存：MVP 直接全量扫描并写入 state，不引入复杂增量索引；后续可在 state 增量缓存目录 mtime。
5. 大目录性能：通过 `exclude_patterns` 限制扫描范围，建议搭配 systemd timer 周期 sync，避免每次交互都重扫。

## 数据模型

- `discovered runs`：扫描得到的全部 run 列表
- `selected runs`：要暴露给 TensorBoard 的集合（按 run id 保存）
- `state.json`：持久化 discovered/selected

## 自动化接口

面向脚本/agent 的稳定输出：

- `tbmux sync --json`
- `tbmux list --json`
- `tbmux status --json`
- `tbmux doctor --json`
- `tbmux tailscale status --json`

## TUI 设计（v0.2）

- 框架：Bubble Tea（事件驱动，键盘交互稳定，后续可扩展实时刷新）
- 入口：交互终端直接执行 `tbmux` 默认进入 TUI，CLI 子命令继续保留为底层接口
- 命令：`tbmux tui [--today|--hours|--days|--running|--not-running|--under|--match]`
- 模型分层：
  - discovered runs：扫描全集（state）
  - filtered view：当前筛选视图（TUI 状态）
  - draft selected：TUI 内存草稿集合
  - apply：落盘 state + 更新 symlink 暴露目录
- 交互键位：
  - `j/k`、方向键：移动
  - `space`：切换 selected
  - `x`：一键清空 draft selected
  - `/`：搜索
  - `r`：running 筛选轮换
  - `t`：today 开关
  - `c`：清空筛选
  - `s`：手动 sync
  - `a`：apply
  - `q`：退出（dirty 时二次确认）
  - `?`：帮助
- 视图策略：
  - 长列表支持光标跟随滚动（viewport）
  - 终端宽度不足时自动退化为单栏，避免右侧 pane 挤压错位
