# tbmux：给 Codex 的完整中文开发 Prompt

你现在要帮助我从零开始构建一个小而稳、适合长期自用的 CLI 项目，项目名暂定为：**`tbmux`**。

请你在整个任务过程中遵循以下要求：

- **和我用中文交流**，包括计划、问题、阶段进展、实现说明、README 主要说明文字。
- 代码、命令名、配置字段、注释、提交信息可以按工程习惯使用英文，但**面对我的解释和交互优先使用中文**。
- 不要只给设计文档，要**实际产出可运行代码仓库**。
- 优先做 **MVP 可用版本**，然后再补测试、README、systemd 示例和易用性打磨。
- 整体目标是：**让我在服务器上几乎不用再手动敲 tensorboard 命令，也能把多个分散目录下的训练统一挂到一个网页端监看，并通过 Tailscale 在 tailnet 内访问。**

---

## 一、项目目标

我要一个面向 Linux 服务器的轻量 CLI 工具，叫 **`tbmux`**。

它的核心用途是：

1. 自动发现多个目录中的 TensorBoard event 日志；
2. 把这些日志聚合到一个统一的可读视图里；
3. 启动并维护一个固定端口的 TensorBoard 服务；
4. 方便通过 **Tailscale** 在 tailnet 内访问；
5. 不要求我修改现有训练代码；
6. 不要求我每次手动进入不同目录执行 tensorboard 命令；
7. 最好是一个 **CLI-first**、部署简单、长期能自用维护的小项目。

注意：
这**不是**要做成 MLflow 那种完整实验管理平台；
而是一个围绕 TensorBoard 的、偏运维侧和聚合侧的工具。

---

## 二、真实使用场景

### 场景 1：多个训练目录分散

我的训练可能散落在这些位置：

- `/data/trainings`
- `/mnt/experiments`
- `/home/ubuntu/jobs`
- 甚至其他任意目录

每个训练各自输出 TensorBoard event 文件。
我希望 `tbmux` 能递归扫描这些根目录，自动发现可监看的 run。

### 场景 2：新训练会不断出现

服务启动后，可能会有新的训练目录出现。
我希望可以通过 `sync` 或后台定期扫描，把新 run 自动纳入统一 TensorBoard。

### 场景 3：固定访问入口

我不想每次新增一个训练，都重新手敲一长串 tensorboard 命令。
我想要一个稳定入口，比如一直是：

- `http://127.0.0.1:6006`
- 或通过 tailnet 域名 / Tailscale Serve 暴露后的地址

### 场景 4：不是所有训练都要同时展示

虽然底层可能发现很多训练，但我并不总是希望网页里展示全部训练。
我希望工具支持“展示集合”的概念：

- 只展示今天产生/更新的训练
- 只展示正在运行中的训练
- 只展示某个指定子目录下的训练
- 只展示带某个标签、别名、路径前缀的训练
- 只展示我手动勾选/选择的一组训练

也就是说，需要区分：

1. **发现到的所有训练（discovered runs）**
2. **当前实际暴露给 TensorBoard 的训练（selected/exposed runs）**

我希望至少支持下面几类筛选方式：

- 按时间筛选（例如今天、最近 N 小时、最近 N 天有更新）
- 按状态筛选（例如推断“正在运行中”）
- 按路径筛选（指定某个子目录、根目录、路径模式）
- 按名称筛选（run name / alias / 模糊匹配）
- 按手动选择筛选（显式 include / exclude）

其中“运行中”的判断不要求绝对完美，但至少请给出一个工程上可用的默认策略，例如：

- 最近一段时间内 event 文件仍在增长或修改
- 最近 N 分钟内目录有新增 event 数据

并且，这个功能最好支持两种交互方式：

#### 方式 A：CLI 方式

例如：

```bash
tbmux list
tbmux list --today
tbmux list --running
tbmux list --under /data/trainings/project-a
tbmux select --today
tbmux select --running
tbmux select --match llama
tbmux select --under /data/trainings/project-a
tbmux select --add run_001
tbmux select --remove run_002
tbmux select --clear
tbmux select --apply
```

命令名可以调整，但核心是：

- 可以列出发现的训练
- 可以带筛选条件查看
- 可以更新“当前展示集合”
- 可以把选择结果应用到 TensorBoard 暴露集合

#### 方式 B：TUI 方式（最好有，但可作为增强项）

我希望最好还能提供一个轻量 TUI，让我在终端里：

- 浏览发现到的训练
- 搜索/筛选
- 多选/取消选择
- 应用当前选择到 Web 展示集合

如果你认为 TUI 不适合放进第一版 MVP，也可以把它作为第二阶段增强功能；
但请在架构上预留好，不要让后续很难补进去。

### 场景 5：Tailscale 安装方式不统一

这个要求非常重要。

不同机器上的 Tailscale 安装方式不同，必须兼容：

1. **管理员级 / 全局安装**
   - 比如 `tailscale` 在系统 PATH 中
   - 或位于 `/usr/bin/tailscale`、`/usr/local/bin/tailscale` 等位置

2. **用户级 / Home 目录安装**
   - 有些机器没有系统级 Tailscale
   - 只有用户目录下的版本，比如：
     - `~/.local/bin/tailscale`
     - `~/bin/tailscale`
     - 或其他常见用户级安装路径

所以项目里**绝对不能写死一个 tailscale 路径**，而是必须设计成：

- 支持显式配置覆盖
- 支持环境变量覆盖
- 支持 PATH 搜索
- 支持常见全局路径搜索
- 支持常见用户目录路径搜索
- 并且在状态或诊断输出中明确告诉我：**当前使用的是哪个 tailscale 可执行文件**

---

## 三、产品方向与推荐架构

请围绕下面这个方向实现：

- 一个 CLI 入口：`tbmux`
- 一个用户可编辑配置文件（TOML 或 YAML，优先 TOML）
- 一个由工具维护的统一 run 目录，例如：
  - `~/.local/share/tbmux/runs/`
- 在这个统一目录下，通过 **symlink 树** 聚合多个真实训练目录
- 一个长期运行的 TensorBoard 进程，指向这个统一目录
- 可选的 Tailscale 辅助命令
- 可选的 systemd user/system service 示例或生成逻辑

### 为什么倾向 symlink tree

我不希望过度依赖复杂的 `--logdir_spec` 行为，因为它在一些历史问题和插件兼容性上并不总是最稳。

更偏好的做法是：

- 工具扫描真实日志目录
- 决定哪些目录算一个 run
- 在统一目录中创建 symlink
- 最后让 TensorBoard 只面对一个统一根目录

也就是说，**TensorBoard 本身尽量保持简单，复杂性放在 `tbmux` 里处理**。

---

## 四、功能需求

### 1. 日志发现（discovery）

请实现递归扫描多个根目录，识别 TensorBoard event 文件，例如：

- `events.out.tfevents.*`
- 其他常见 TensorBoard event 命名模式（如有必要可补充）

需要设计一个可靠规则，用于判断：

- 哪个目录应该被视为一个可挂载 run
- 是否选择 event 文件所在目录作为 run 根目录
- 如何避免把过高层目录误判成一个 run

请优先考虑稳定、直观、易解释。

### 2. 统一 run 目录

维护一个工具自己的聚合目录，例如：

- `~/.local/share/tbmux/runs/`

要求：

- 对每个发现的 run 创建一个 symlink
- run 名称应尽量**可读、稳定**
- 避免不同源路径之间的名称冲突
- 支持在重扫时更新映射
- 如果源 run 消失，支持按配置清理 stale symlink
- 最好保留一份状态元数据，便于 `status` / `doctor` / `list` 使用

此外，请把“已发现训练”和“当前已选中展示训练”区分开：

- `discovered runs`：扫描出来的全部候选训练
- `selected runs`：当前要实际暴露到 TensorBoard Web 页面中的训练

也就是说，统一 run 目录里的 symlink 最好默认只对应 **selected runs**，而不是无脑把全部 discovered runs 都挂进去。

请设计一种清晰的数据模型来保存：

- run 的源路径
- run 的稳定 ID
- run 的可读显示名
- 最近更新时间
- 是否推断为 running
- 来自哪个 watched root
- 当前是否被 selected
- selection 来源（规则选中 / 手动选中 / 手动排除）

### 3. 训练筛选与展示集合管理

这是一个核心功能，不是边角需求。

请实现一种“展示集合管理”机制，让用户决定哪些训练真正显示在 TensorBoard 页面中。

至少应支持：

#### 3.1 筛选能力

- 按更新时间筛选
  - 今天
  - 最近 N 小时
  - 最近 N 天
- 按状态筛选
  - running
  - not running
- 按路径筛选
  - 指定根目录
  - 指定子目录
  - 路径前缀 / glob / regex（任选一种或多种，但请控制复杂度）
- 按名称筛选
  - 精确匹配
  - 模糊匹配
- 按标签/别名筛选（如果你认为值得做）

#### 3.2 选择能力

至少支持：

- 清空当前选择
- 根据筛选条件批量选中
- 根据筛选条件批量取消选中
- 手动按 run ID / 名称增删
- 查看当前 selected runs
- 把当前选择应用到实际暴露集合

#### 3.3 交互方式

优先做 CLI，可选做 TUI。

CLI 至少需要有一套清晰命令来完成：

- 查看 discovered runs
- 查看 selected runs
- 按条件筛选
- 更新 selected runs
- 应用选择结果到 TensorBoard 暴露目录

如果你觉得命令设计更合理，可以不完全照抄下面这些，但能力上要覆盖：

```bash
tbmux list
tbmux list --today
tbmux list --running
tbmux list --under /data/trainings/project-a

tbmux selected list

tbmux select clear
tbmux select add run_001
tbmux select remove run_002
tbmux select by-filter --today
tbmux select by-filter --running
tbmux select by-filter --under /data/trainings/project-a
tbmux select apply
```

#### 3.4 TUI（增强项）

如果实现成本合适，希望提供一个轻量 TUI，例如：

```bash
tbmux tui
```

目标体验：

- 左侧或列表中显示 discovered runs
- 支持搜索和筛选
- 支持多选
- 支持切换 selected 状态
- 最终一键 apply 到 TensorBoard 暴露集合

如果 TUI 不放进 MVP，请：

- 在第一轮设计里明确标注它是 Phase 2
- 但从架构上预留独立的数据层 / 选择层，避免以后重构很痛苦

### 4. TensorBoard 进程管理

请提供实用的 CLI 子命令，初步建议如下：

- `tbmux init`
- `tbmux sync`
- `tbmux start`
- `tbmux stop`
- `tbmux restart`
- `tbmux status`
- `tbmux list`
- `tbmux selected list`
- `tbmux select ...`
- `tbmux doctor`
- `tbmux open`
- `tbmux tailscale status`
- `tbmux tailscale serve`

命令名可以微调，但要保持：

- 直观
- 偏运维风格
- 易记
- 真正可用

进程管理方面，可接受的实现方式包括：

- 直接子进程管理
- PID 文件
- systemd user service
- systemd system service
- 或组合方案

但请记住：
**优先可靠性，不要做过度炫技的复杂守护逻辑。**

如果你认为最佳实践是：

- CLI 负责 init/sync/doctor
- systemd 负责常驻进程

也完全可以，请把边界设计清楚。

### 4. Tailscale 集成

必须支持与 Tailscale 配合使用，但不能假设所有场景都有 root 权限。

建议支持：

- 检测 `tailscale` 可执行文件
- 检测 `tailscale status`
- 检测当前 tailnet 状态
- 输出或执行用于暴露 TensorBoard 的命令

例如：

- `tailscale serve 6006`
- 或更明确的变体（视版本而定）

但这里要注意：

- 不同 Tailscale 版本或权限模型可能不同
- 如果自动执行 `tailscale serve` 风险较高，请提供：
  - dry-run
  - 明确提示
  - 或仅打印推荐命令

重点不是“强行自动化”，而是“**在不同机器上都尽量稳地帮助我完成暴露**”。

### 5. 配置文件

请设计一个人类可编辑、结构清晰的配置文件。

优先选择：
- TOML

建议配置项包括但不限于：

- watched roots
- root alias / label
- exclude patterns
- tensorboard host
- tensorboard port
- managed run dir
- scan interval
- tailscale binary override
- tailscale exposure mode
- naming rules
- stale entry cleanup
- 默认选择规则（例如今天 / running / 某路径前缀）
- 手动 include / exclude 持久化
- 是否把 selection 结果单独存成 state 文件
- 日志级别

请给出：

1. 配置 schema 设计
2. 默认配置样例
3. README 中的说明

### 6. 运维友好性

输出风格应该适合真正运维使用：

- 默认输出简洁
- 错误信息明确
- 不要过度冗长
- `doctor` 命令要真正有价值

`doctor` 至少应检查：

- TensorBoard 是否已安装 / 是否可执行
- Tailscale 是否已安装 / 检测到哪个路径
- 配置文件是否合法
- 监看根目录是否存在、是否可读
- 聚合目录是否可写
- 端口是否被占用
- symlink 创建是否受限
- selection state 是否可读写、是否和 discovered runs 一致
- 当前运行状态是否正常

---

## 五、交付物要求

请最终交付一个完整仓库，至少包含：

1. **可运行 CLI 实现**
2. **README.md**（中文为主，必要时英文术语保留）
3. **示例配置文件**
4. **安装与使用说明**
5. **systemd user service 示例**
6. **可选：systemd system service 示例**
7. **测试**（尤其是 discovery、命名冲突、symlink 映射逻辑）
8. **简短设计说明**（解释关键取舍）

不要只停留在“能跑通一个 happy path”。
至少把最关键的边界情况处理好。

---

## 六、硬性约束

### 必须满足

1. 依赖尽量克制，不要为了小项目引入过重框架。
2. 优先考虑 Linux 服务器上的长期可维护性。
3. 不要求 Docker。
4. 不要求 Kubernetes。
5. 不要求修改现有训练脚本。
6. 不依赖 TensorBoard.dev 或云服务。
7. 默认安全模型应当是：**私有访问，默认面向 Tailscale tailnet，而不是公网直接暴露。**
8. **Tailscale 检测必须同时兼容：全局安装 + 用户目录安装。**

### 不希望出现

- 一堆为了“架构优雅”而增加的复杂度
- 强绑定某种单一部署方式
- 过度脆弱的 shell 拼接逻辑
- 只适配一种 Tailscale 安装路径
- 需要用户每次手工重新指定所有目录

---

## 七、建议的命令设计

你可以先提出更优方案，但我期望接近下面这种：

```bash
tbmux init
tbmux sync
tbmux list
tbmux list --today
tbmux list --running
tbmux list --under /data/trainings/project-a

tbmux selected list

tbmux select clear
tbmux select add run_001
tbmux select remove run_002
tbmux select by-filter --today
tbmux select by-filter --running
tbmux select by-filter --under /data/trainings/project-a
tbmux select apply

tbmux start
tbmux stop
tbmux restart
tbmux status
tbmux open
tbmux doctor

tbmux tailscale status
tbmux tailscale serve --dry-run
tbmux tailscale serve
```

也可以考虑支持：

```bash
tbmux config path
tbmux config example
tbmux runs prune
tbmux tui
```

如果你觉得命令需要调整，请在开始实现前说明理由。

---

## 八、建议的实现顺序

请按这个顺序推进：

### 第一步：先给设计，不要一上来瞎写
先明确提出：

1. 仓库目录结构
2. CLI 命令布局
3. 配置文件 schema
4. 运行模型（CLI 管理进程、还是 systemd 为主）
5. Tailscale 可执行文件检测策略
6. run 命名与冲突规避策略

### 第二步：实现 MVP
完成最小可用版本，至少包括：

- init
- sync
- list（支持基础筛选）
- selected list
- select clear / add / remove / by-filter / apply
- start
- stop
- status
- doctor
- tailscale status

如果你认为 TUI 不适合进 MVP，请明确标成 Phase 2，但 selection 的数据模型和 CLI 必须在 MVP 就成立。

### 第三步：完善文档与部署体验
补上：

- README
- 示例配置
- systemd user service
- 可选 systemd system service
- 典型使用流程

### 第四步：补测试与打磨
重点补：

- discovery 测试
- selection/filter 测试
- symlink tree 测试
- naming collision 测试
- running 状态判断测试
- tailscale 检测逻辑测试
- 配置解析测试

---

## 九、实现语言选择

请你自行选择最适合这个项目的语言。

候选：
- Go
- Rust
- Python

选择标准：

- 部署门槛低
- 运行稳定
- 依赖少
- 对 CLI 项目友好
- 维护成本低

如果你选 Python，请把打包、venv、systemd 使用体验设计清楚，不要留下一个“只能在开发机跑”的半成品。

如果你选 Go，我会倾向认为它很适合这个项目，因为：
- 单文件二进制方便部署
- CLI 生态成熟
- 运维味比较重

但你可以自行判断，只要理由充分。

---

## 十、关于 Codex 最终使用便利性的额外要求

除了把项目本身做好，我还希望这个项目在后续使用上，对 **Codex / coding agent** 也尽量友好。

请在设计里考虑：

### 1. 方便被 Codex 理解和调用
也就是说，项目结构和命令尽量清晰，让后续我可以很容易让 Codex 做这些事：

- 新增一个命令
- 调整 run 命名规则
- 增加一种发现策略
- 优化 doctor 输出
- 添加一个 Tailscale 辅助动作

所以请做到：

- 模块边界清楚
- CLI 子命令组织清楚
- 配置 schema 不要混乱
- README 和设计说明要让后续 agent 容易读懂

### 2. 尽量提供“可被 skill / hook / agent workflow 复用”的接口
这里不一定要真的集成某个特定平台，但最好在项目中预留一种简单、稳定、适合自动化调用的方式。

优先考虑下面这些能力：

- `tbmux status --json`
- `tbmux list --json`
- `tbmux doctor --json`
- `tbmux tailscale status --json`

也就是说：

- 默认给人看的输出保持简洁
- 同时提供 JSON 输出，方便未来 skill、hook、脚本、agent 自动消费

如果实现成本不高，也可以考虑：

- 明确的退出码设计
- `--quiet`
- `--verbose`
- `--config <path>`

### 3. 对自动化友好的原则
项目最好天然适合以后接到：

- shell hook
- systemd timer
- OpenClaw / Codex / Claude Code 之类的 coding agent
- 其他运维脚本

所以请尽量避免：

- 只有人类交互才能使用的流程
- 无法脚本化的模糊输出
- 高耦合的全局状态

换句话说：
**这个项目既要适合人直接用，也要适合 agent / skill / hook 方便调用。**

请在 README 或 design note 中专门留一小节，说明：

- 哪些命令适合自动化调用
- JSON 输出如何使用
- 推荐的 hook / timer / agent 调用方式是什么

---

## 十一、希望你主动给出的工程判断

在实现过程中，请主动做这些判断并说明：

1. TensorBoard 进程更适合由 `tbmux` 自己直接托管，还是更适合交给 systemd
2. Tailscale `serve` 是否应该默认只给出建议命令，而不是直接执行
3. run 命名到底应该更偏“可读”还是更偏“绝对稳定”
4. 是否需要缓存 discovery 结果
5. 对于大型目录树，如何避免每次全量扫描过慢

不要把这些都丢回给我做产品经理式裁决；
你可以先给默认决策，再说明如果以后要扩展，结构上怎么留余地。

---

## 十二、第一轮输出要求

请你开始工作时，**先不要直接埋头写代码**。

你的第一轮输出请按下面格式给我：

1. **你选择的实现语言及理由**
2. **仓库目录结构提案**
3. **CLI 设计提案**
4. **配置文件 schema 提案**
5. **运行模型提案**（进程管理、systemd 关系）
6. **Tailscale 检测策略提案**
7. **MVP 范围说明**
8. **然后再开始实现**

---

## 十三、验收标准

我会重点看这些：

- 是否真的解决“多个训练统一监看”的问题
- 是否真的减少手动 tensorboard 操作
- 是否真的兼容不同安装方式的 Tailscale
- 是否部署简单
- 是否适合 Linux 服务器长期使用
- 是否让后续 Codex / skill / hook 调用方便
- 是否文档清楚
- 是否代码结构干净

如果你在实现过程中发现某个需求不合理，可以直接指出并给出替代设计。
我接受工程上更合理的调整，但不接受偷懒式删需求。

---

## 十四、补充偏好

请记住以下偏好：

- 与我沟通时使用中文
- 优先可用、稳、易部署
- 不喜欢花哨但不实用的架构
- 希望 CLI 味足一点
- 希望后续容易被 agent / skill / hook 调用
- 如果某功能存在权限/安全风险，优先做成显式 opt-in，而不是默认自动执行

---

现在请开始：

先给出实现语言、目录结构、CLI 设计、配置 schema、运行模型、Tailscale 检测策略和 MVP 范围，然后进入实现。
