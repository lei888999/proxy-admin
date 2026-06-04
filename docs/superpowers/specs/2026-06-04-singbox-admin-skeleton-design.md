# sing-box-admin 最小可运行骨架 — 设计文档

- 日期：2026-06-04
- 状态：已批准设计，待写实现计划
- 里程碑：M1（骨架 + 登录 + 数据库）

## 1. 背景与目标

sing-box-admin 是一个部署在海外 VPS 上的 **sing-box 服务端管理面板**：后端运行并管理同机的 sing-box 进程、生成/修改服务端配置，前端是管理后台。形态类似 3x-ui / s-ui。

本设计只覆盖**第一个里程碑（M1）**：搭建可运行的全栈骨架，跑通"前端登录 → 调用受保护 API → 展示 sing-box 状态"的最小闭环。完整的用户管理、inbound/出站配置、订阅、流量统计等功能不在本里程碑，后续各自立项（每个子项目走独立的 spec → plan → 实现循环）。

### M1 范围内

- monorepo 目录结构（`backend/` Go + `frontend/` Next.js）
- 管理员登录鉴权（JWT，存 httpOnly cookie）
- SQLite 持久化（GORM），首启自动建默认管理员
- sing-box 状态探测接口（版本 / 是否安装 / 是否运行）
- 单二进制打包：前端静态导出嵌入 Go 二进制
- 顶层 Makefile：一键 dev / build / run

### M1 范围外（后续里程碑）

- sing-box 进程的 start/stop/restart 控制
- inbound / 用户 / 订阅 / 流量统计
- 强制改密、HTTPS、密钥持久化等生产加固
- 多管理员、权限分级

## 2. 技术栈

| 层 | 选型 |
|----|------|
| 后端语言 | Go |
| HTTP 框架 | Gin |
| ORM | GORM |
| 数据库 | SQLite |
| 鉴权 | JWT（HS256），httpOnly cookie |
| 前端框架 | Next.js（App Router）+ TypeScript |
| 样式/组件 | Tailwind CSS + shadcn/ui |
| 打包形态 | 单二进制（Next.js `output: 'export'` + Go `embed.FS`） |

## 3. 总体架构

```
┌─────────────────────────────────────────────┐
│  单 Go 二进制 (部署在 VPS)                     │
│                                               │
│  Gin HTTP Server (:8080)                      │
│   ├── POST /api/auth/login   → 校验 + 下发 cookie│
│   ├── POST /api/auth/logout  → 清 cookie       │
│   ├── GET  /api/status       → sing-box 状态   │ (需 JWT)
│   └── GET  /*                → embed.FS 前端静态 │
│                                               │
│  GORM + SQLite (sing-box-admin.db)            │
│   └── admins 表                               │
│                                               │
│  SingboxService → 探测 sing-box 版本/运行状态  │
└─────────────────────────────────────────────┘
        ▲ 开发期 :3000 Next.js dev
        │  next.config rewrites: /api/* → :8080 (同源, 无 CORS)
```

- **开发期**：前端 `next dev`（:3000），`next.config` 用 `rewrites` 把 `/api/*` 代理到 Go（:8080）。浏览器视角同源，cookie 与热更新均正常工作。
- **生产期**：`next build` 以 `output: 'export'` 导出到 `frontend/out/`，构建流程拷贝到 `backend/internal/web/dist/`，Go 用 `//go:embed` 嵌入，单二进制同时服务 `/api/*` 与前端页面。

## 4. 目录结构

```
sing-box-admin/
├── backend/
│   ├── cmd/server/main.go          # 入口：装配配置、DB、路由、中间件
│   ├── internal/
│   │   ├── config/config.go        # 读环境变量（端口、JWT 密钥、DB 路径、默认账号）
│   │   ├── database/database.go    # GORM 初始化 + AutoMigrate + 首启建默认管理员
│   │   ├── models/admin.go         # Admin 模型
│   │   ├── handlers/auth.go        # login / logout
│   │   ├── handlers/status.go      # status
│   │   ├── middleware/auth.go      # JWT 鉴权中间件（从 cookie 读）
│   │   ├── service/singbox.go      # sing-box 版本/状态探测（命令执行抽象为接口）
│   │   └── web/embed.go            # //go:embed dist 前端产物 + SPA fallback
│   ├── go.mod
│   └── Makefile
├── frontend/
│   ├── app/
│   │   ├── layout.tsx
│   │   ├── login/page.tsx          # 登录页
│   │   └── dashboard/page.tsx      # 仪表盘：展示 sing-box 状态
│   ├── lib/api.ts                  # fetch 封装（credentials: 'include'）+ 401 处理
│   ├── components/ui/              # shadcn/ui 组件
│   ├── next.config.ts              # output:'export' + dev rewrites
│   ├── tailwind.config.ts
│   └── package.json
├── docs/superpowers/specs/         # 设计文档
├── Makefile                        # 顶层：dev / build / run
├── .gitignore
└── CLAUDE.md                       # 骨架跑通后据真实代码生成（init 命令的最终产物）
```

## 5. 数据模型

### Admin

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint | 主键 |
| username | string | 唯一索引 |
| password_hash | string | bcrypt |
| created_at | time | GORM 自动 |
| updated_at | time | GORM 自动 |

**首次启动**：`database` 初始化时若 admins 表为空，自动创建默认管理员：
- 用户名：`admin`
- 密码：`mnice7082`（bcrypt 后入库）

默认账号来自配置（环境变量可覆盖），骨架阶段不强制改密（留 TODO，后续里程碑加）。

## 6. 认证流程

- **登录**：`POST /api/auth/login {username, password}` → bcrypt 校验 → 签发 JWT（HS256，claims 含 `sub`=用户名、`exp`=7 天）→ 通过 `Set-Cookie` 下发：
  - Cookie 名：`token`
  - 属性：`HttpOnly`、`SameSite=Lax`、`Path=/`、`Max-Age=7d`（生产再加 `Secure`）
  - 响应体返回 `{username}`（不含 token，token 仅在 cookie）
- **登出**：`POST /api/auth/logout` → 下发过期 cookie 清除。
- **鉴权中间件**：受保护路由从 cookie 读 `token` → 校验签名与过期 → 注入用户名到 context；失败返回 401。
- **JWT 密钥**：从环境变量 `JWT_SECRET` 读；缺省时随机生成并打 WARN 日志（重启会失效，骨架可接受；持久化留后续）。
- **前端**：
  - `lib/api.ts` 所有请求带 `credentials: 'include'`。
  - 因 cookie 为 httpOnly，前端无法读 token；通过调用 `/api/status` 是否返回 401 判断登录态。
  - dashboard 加载时若 `/api/status` 返回 401 → 跳 `/login`。
  - 登录成功后跳 `/dashboard`；登出调用 logout 后跳 `/login`。

## 7. sing-box 状态服务

`SingboxService.Status()` 骨架阶段最小实现，返回：

```json
{ "installed": true, "version": "1.x.x", "running": false }
```

- `installed` + `version`：执行 `sing-box version` 解析输出；命令不存在 → `installed:false`，不报错（VPS 可能尚未安装 sing-box）。
- `running`：检查是否存在运行中的 sing-box 进程（按进程名）。
- 命令执行通过接口抽象（如 `CommandRunner`），便于测试中 mock，不直接在 handler 里调 `exec`。

进程控制（start/stop/restart）不在 M1。

## 8. 测试策略

### 后端
- `handlers/auth`：用 `httptest` 测登录成功、密码错误、缺字段、登出清 cookie。
- `handlers/status`：带合法 cookie → 200 且字段正确；无 cookie / 过期 → 401。
- `middleware/auth`：合法/非法/过期 token 的放行与拦截。
- `service/singbox`：通过 mock `CommandRunner` 测「已安装」「未安装」「running/not running」分支。

### 前端
- `lib/api.ts`：请求封装与 401 处理的单元测试。
- 登录页：能渲染、能提交表单、提交后按响应跳转。
- 重交互/端到端测试留后续里程碑。

## 9. 构建与运行

顶层 `Makefile` 目标（具体命令在实现计划中确定）：
- `make dev`：并行起 Go（:8080）与 Next.js dev（:3000）。
- `make build`：`next build` 导出 → 拷入 backend embed 目录 → `go build` 出单二进制。
- `make run`：运行构建出的二进制。

## 10. 风险与待办（后续里程碑）

- JWT 密钥持久化（当前随机生成，重启失效）。
- 强制首次改密。
- HTTPS / 反代 / `Secure` cookie。
- sing-box 进程生命周期管理与配置生成（核心功能，下一个里程碑）。
- CLAUDE.md 在骨架跑通后据真实代码与命令生成。
