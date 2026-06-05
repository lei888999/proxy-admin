# M2: sing-box 进程控制 + B 端后台 shell — 设计文档

- 日期：2026-06-05
- 状态：设计待用户确认
- 里程碑：M2（启动/停止 sing-box + 原始配置编辑 + B 端后台界面）

## 1. 背景与目标

M1 骨架只能探测 sing-box 状态。M2 让面板真正**管理同机的 sing-box 进程**：编辑原始 `config.json`、启动、停止；并把前端从两张居中裸卡片改造成正式的 **B 端后台**（侧边栏 + 顶栏 + 内容区），沿用 `docs/DESIGN.md` 的深色 xAI 调色板。

关键约束（用户确认）：
- **sing-box 随部署内置**，面板不做"安装/下载"。docker 镜像已从官方镜像内置 `/usr/local/bin/sing-box`；native 部署由 `deploy-native.sh` 下载并随二进制一起打包到 VPS。
- 配置只做**原始 `config.json` 文本编辑**，不做可视化节点编辑。
- 进程由**面板直接拉子进程**托管（PID 文件），不依赖 systemd 管 sing-box。

### 范围内
- 后端：`config.json` 读/写/校验、启动（含 `sing-box check`）、停止、状态扩展。
- 端点：`GET/PUT /api/singbox/config`、`POST /api/singbox/start`、`POST /api/singbox/stop`、扩展 `GET /api/status`。
- 前端：深色 B 端后台 shell（侧边栏/顶栏/内容区）+ 概览视图（状态 + 启停）+ 配置视图（编辑器）。
- 部署：`SINGBOX_DIR` 环境变量接入 compose + systemd；`deploy-native.sh` 打包 sing-box。

### 范围外（后续里程碑）
- 可视化 inbound/节点/订阅编辑、流量统计、多配置切换。
- 面板内安装/升级 sing-box（已由部署内置取代）。
- sing-box 热重载（M2 用停止+启动即可）。

## 2. 总体架构

```
前端 B 端 shell ──HTTP(cookie 鉴权)──> Gin handlers (RequireAuth)
  ├ GET    /api/status            → SingboxService.Status()  {installed,version,running,hasConfig}
  ├ GET    /api/singbox/config    → ConfigStore.Get()
  ├ PUT    /api/singbox/config    → ConfigStore.Save(content)   (json 语法校验)
  ├ POST   /api/singbox/start     → SingboxService.Start()      (check 校验→spawn)
  └ POST   /api/singbox/stop      → SingboxService.Stop()

托管目录 $SINGBOX_DIR/   (docker: /data/singbox; native: $DEPLOY_PATH/data/singbox)
  ├ bin/sing-box     native 部署打包的二进制（docker 用 PATH 上的 /usr/local/bin/sing-box）
  ├ config.json      面板编辑保存
  ├ sing-box.pid     子进程 PID
  └ sing-box.log     子进程 stdout/stderr（追加）
```

**二进制解析顺序**（`ResolveBin`）：`$SINGBOX_BIN`（显式覆盖）→ `$SINGBOX_DIR/bin/sing-box`（native 打包）→ `LookPath("sing-box")`（docker 内置在 PATH）→ 空（视为未安装）。docker 与 native 都能解析到，`installed` 恒为真。

## 3. 后端单元（新包 `internal/singbox/`）

各单元的外部 I/O（exec / 文件 / 信号）走可注入接口，编排逻辑用 fake 单测。

### 3.1 `ConfigStore`
- `Get() (string, error)`：读 `config.json`，不存在返回空串、`hasConfig=false`，不报错。
- `Save(content string) error`：先 `json.Valid([]byte(content))` 语法校验（失败 → `ErrInvalidJSON`），再原子写入（temp+rename）权限 `0600`。
- `Exists() bool`、`Path() string`。

### 3.2 `ProcessManager`
依赖小接口 `Commander`（`Run`/`Start` 命令）与 `Signaler`（按 PID 发信号、探活），便于 mock。
- `Start(bin, configPath string) error`：
  1. 若 `Running()` → `ErrAlreadyRunning`。
  2. `bin check -c configPath`：退出码非 0 → `ErrInvalidConfig{Output}`（不启动）。
  3. detached spawn `bin run -c configPath`：`SysProcAttr{Setpgid:true}`，stdout/stderr 追加到 `sing-box.log`，**只 Start 不 Wait**（面板重启不杀子进程）。
  4. 写 PID 文件。
- `Stop() error`：读 PID；不存在/已死 → `ErrNotRunning`；否则 SIGTERM → 轮询最多 3s → 仍存活则 SIGKILL → 删 PID 文件。
- `Running() bool`：PID 文件存在且进程存活（`Signal(0)`）；兜底 `pgrep -x sing-box`（接管外部启动的进程）。

### 3.3 `SingboxService`（编排 + 状态）
- 持有 `sync.Mutex`，`Start`/`Stop` 串行化，避免并发拉起两个进程。
- `Status() Status{Installed,Version,Running,HasConfig}`：`Installed`=ResolveBin 非空；`Version`=`bin version` 解析（沿用 M1 正则）；`Running`=ProcessManager.Running()；`HasConfig`=ConfigStore.Exists()。
- `Start()`：ResolveBin 空 → `ErrNotInstalled`；无配置 → `ErrNoConfig`；否则 `ProcessManager.Start`。
- `Stop()`：透传 `ProcessManager.Stop`。
- `Config()/SaveConfig()`：透传 ConfigStore。

> M1 的 `internal/service/singbox.go`（`Runner`/`Status`）合并进新 `internal/singbox` 包并扩展；`Status` 增加 `HasConfig`。`status` handler 改用新 service。这是本里程碑触及范围内的合并，不做无关重构。

## 4. HTTP 端点与错误映射

全部挂在 `RequireAuth` 后。错误 → 状态码：

| 场景 | 状态码 | 响应体 |
|---|---|---|
| 启动：未安装 | 400 | `{"error":"sing-box not installed"}` |
| 启动：无配置 | 400 | `{"error":"no config"}` |
| 启动：`check` 不通过 | 400 | `{"error":"invalid config","detail": <check 输出>}` |
| 启动：已在运行 | 409 | `{"error":"already running"}` |
| 停止：未运行 | 409 | `{"error":"not running"}` |
| 保存配置：JSON 语法错 | 400 | `{"error":"invalid json"}` |
| 其它 I/O | 500 | `{"error": ...}` |
| 成功 | 200 | 启停返回最新 `Status`；GET config 返回 `{"content":...}` |

启停成功后返回最新 `Status`，前端据此刷新按钮态（少一次往返）。

## 5. 配置与部署改动

### 环境变量（`internal/config`）
- `SINGBOX_DIR`（默认 `./singbox`）：托管目录。
- `SINGBOX_BIN`（默认空）：显式覆盖二进制路径。
- `SINGBOX_VERSION`（默认 `1.14.0`）：仅 `deploy-native.sh` 打包、`docker-compose.yml` 的 `SING_BOX_IMAGE` 钉版时用，运行时不读。docker 端同步把 `SING_BOX_IMAGE` 默认钉到 `ghcr.io/sagernet/sing-box:v1.14.0`。

### docker
- `docker-compose.yml` 增 `SINGBOX_DIR: /data/singbox`（落在已有数据卷，持久化）。镜像已内置 sing-box 于 PATH，`ResolveBin` 兜底命中。

### native（`deploy-native.sh`）
- 探测 VPS 架构后，下载 `sing-box-${SINGBOX_VERSION}-linux-${arch}.tar.gz`，解出 `sing-box`，`scp` 到 `$DEPLOY_PATH/data/singbox/bin/sing-box` 并 chmod。
- systemd unit 增 `Environment=SINGBOX_DIR=$DEPLOY_PATH/data/singbox`。
- 进程托管仍是面板拉子进程（不是 sing-box 的 systemd service）；面板自身仍由 systemd 托管。

## 6. 前端：深色 B 端后台 shell

整体从两张居中卡片改为后台 shell。沿用 `docs/DESIGN.md` 深色 token（近黑画布、`#191919` 面、发丝边框、白药丸、Geist/Geist Mono）。实现时用 `frontend-design` 技能提升质感。

### 文案语言：统一中文
所有功能文案（导航、按钮、标签、状态、提示、错误）一律中文，**不再中英混用**。`sing-box` / `Geist` 等专有名词保留原文，mono eyebrow 可用产品名 `sing-box admin`。统一用词：用户名、密码、登录、退出登录、仪表盘、概览、配置、启动、停止、保存配置、运行中、已停止、已安装、未安装、加载中。
> 现有 `login`/`dashboard` 页面与其测试当前用英文（Username/Password/Sign in/Logout/Dashboard）；本里程碑改为上述中文，并相应更新测试断言（`getByLabelText(/用户名/)`、按钮名 `/登录/` 等）。

### 登录页（做"大气"）
登录页不套后台 shell，单独设计为更有气场的入口，仍是深色 xAI 调色板：
- **左右分屏**（桌面）：左侧品牌区——大号 Geist 展示字体标题（字重 400、负字距）+ mono 大写 eyebrow + 一句副标题，占据视觉重心；右侧居中登录表单卡。窄屏回退为单列（品牌区压缩到表单上方）。
- 表单：用户名/密码/登录按钮（白药丸），错误内联红字；留白充足（`gap`/`px` 加大），发丝边框卡。
- 目标是"打开就有产品感"，而非一张小卡片漂在中间。实现时用 `frontend-design` 技能。

### 结构
- `components/app-shell.tsx`：侧边栏（LOGO + 导航：概览 / 配置）+ 顶栏（标题 + 用户菜单含登出）+ 内容区 `{children}`。鉴权由 shell 统一处理：挂载即 `getStatus()`，401 → `/login`。
- 路由（Next App Router，静态导出）：
  - `/login`：保持现状（深色，不套 shell）。
  - `/dashboard`（概览，套 shell）：状态卡（运行中●/已停止、版本、已安装、有无配置）+ `启动`/`停止` 按钮（按状态启用/禁用，操作中 loading，失败显示 error）。
  - `/config`（套 shell）：mono 文本框，`GET config` 载入，`保存配置` 调 `PUT`；保存/校验错误内联显示。
  - `/` → 重定向 `/dashboard`（沿用）。
- `lib/api.ts` 新增：`getConfig()`、`saveConfig(content)`、`startSingbox()`、`stopSingbox()`；`SingboxStatus` 加 `hasConfig`。

### 交互
- 概览按钮启用规则：`启动` 需 `installed && hasConfig && !running`；`停止` 需 `running`。
- 启停/保存后用返回的 `Status` 或重新 `getStatus()` 刷新。
- `check`/保存失败时把后端 `detail`/`error` 展示给用户。

## 7. 测试

### 后端
- `ConfigStore`：保存非法 JSON → 错误且不落盘；保存合法 → 读回一致；不存在 → Get 空、Exists 假。
- `ProcessManager`（fake Commander/Signaler）：已运行→Start 报错；check 失败→不 spawn 且返回输出；正常→spawn 且写 PID；Stop 未运行→报错；Stop 正常→发 SIGTERM、删 PID。
- `SingboxService`：未安装/无配置/已运行各分支；Status 四字段正确。
- handlers：httptest + fake service，覆盖第 4 节每个状态码。

### 前端（Vitest）
- `lib/api.ts` 新方法（mock fetch）。
- `app-shell` 渲染导航项（概览/配置）、登出调用 logout。
- `/dashboard`：按 status 启用/禁用启停按钮、点击调用对应 api 并刷新。
- `/config`：载入既有内容、保存调用 saveConfig、显示校验错误。
- 现有 `login`/`dashboard` 测试断言改为中文文案（`getByLabelText(/用户名/)`、`getByLabelText(/密码/)`、按钮 `/登录/`、状态文本 `/运行中|已停止/` 等）；登录成功仍跳 `/dashboard`、401 仍跳 `/login`。

## 8. 风险与说明
- **以 root 运行任意配置**：这是管理员自有的代理面板，属预期；文档提示。
- **子进程跨面板重启**：Setpgid + 不 Wait + PID/pgrep 重新接管，面板重启不影响 sing-box 运行。
- **并发启动**：service mutex 串行化。
- **端口占用 / 启动失败**：`check` 拦住语法/结构错误；运行期失败写入 `sing-box.log`，`Running()` 会反映真实存活态（后续里程碑可加日志查看端点）。
