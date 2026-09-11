# sing-box-admin

sing-box 的服务端管理面板：Go 后端管理本机的 sing-box 进程与配置，Next.js 前端做管理界面。生产环境打包成**单个无 CGO 的二进制**，同时对外提供 API 和内嵌的前端页面。

---

## 环境要求

按你选择的运行方式，各有不同：

- **本地构建 / 开发**：Go、Node.js（npm）。
- **Docker 部署**：VPS 上装好 Docker 和 Docker Compose，仅支持 **Linux 主机**（用到了 host 网络）。
- **原生部署（systemd）**：VPS 什么都不用装，只要能跑二进制并有 root 权限；面板会自动下载 sing-box。

---

## 安装与本地运行

```bash
# 1. 开发模式：后端 :8080 + 前端 :3000 一起跑
make dev

# 2. 构建单二进制（前端静态导出 → 内嵌 → go build）
make build          # 产物在 backend/bin/sing-box-admin

# 3. 构建并运行
make run            # 打开 http://localhost:8080
```

二进制通过环境变量读取配置：

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `PORT` | `8080` | 面板监听端口 |
| `DB_PATH` | `sing-box-admin.db` | SQLite 数据库路径 |
| `JWT_SECRET` | 随机生成 | 签名会话 Cookie 的密钥；**生产必须设置**，否则每次重启会话都会失效 |
| `DEFAULT_ADMIN_USER` | `admin` | 初始管理员用户名（仅首次、表为空时写入） |
| `DEFAULT_ADMIN_PASS` | `mnice7082` | 初始管理员密码 |
| `SERVER_HOST` | 请求 Host | 生成订阅时用的公网域名/IP |

首次启动会自动创建默认管理员账号 `admin` / `mnice7082`，登录后请尽快修改。

---

## 部署到 VPS

两种方式，都通过 `.deploy.env` 配置 SSH 目标（从 `.deploy.env.example` 复制，假设已配好密钥登录）：

```bash
cp .deploy.env.example .deploy.env
# 填写：
#   DEPLOY_HOST=root@your.vps.ip
#   DEPLOY_PATH=/opt/sing-box-admin
#   DEPLOY_PORT=22        # 可选，默认 22
```

> 两种方式都**不会覆盖** VPS 上已有的 `.env` 和数据库。

### 方式一：原生部署（推荐用于测试/调试，不需要 Docker）

```bash
make deploy-native          # 单次部署
make watch-deploy-native    # 保存即重新部署（需 brew install fswatch）
```

流程：本地构建前端 + 交叉编译**单个静态二进制**（架构自动探测），只上传二进制，在 VPS 上以 **systemd** 服务运行（`sing-box-admin.service`，自动安装）。sing-box 会在 VPS 上按需下载。数据库位于 `$DEPLOY_PATH/data/`。

部署前需在 VPS 的 `$DEPLOY_PATH/.env` 里写好 `JWT_SECRET`：

```bash
ssh root@your.vps.ip 'mkdir -p /opt/sing-box-admin && \
  echo "JWT_SECRET=$(openssl rand -hex 32)" > /opt/sing-box-admin/.env'
```

查看日志：

```bash
ssh root@your.vps.ip 'journalctl -u sing-box-admin -f'
```

### 方式二：Docker 部署（与生产一致）

```bash
make deploy          # rsync 源码 + 远程 docker compose up -d --build
make watch-deploy    # 保存即重新部署
```

流程：rsync 整个工程（排除 `node_modules`/`.git`/`.env`/数据），在 VPS 上执行 `docker compose up -d --build`。VPS 需要 Docker + Compose，数据持久化在命名卷 `singbox_admin_data`。

也可以直接在 VPS 上手动跑：

```bash
cp .env.example .env
# 编辑 .env，至少设置 JWT_SECRET（openssl rand -hex 32）
docker compose up -d --build

# 快捷命令
make docker-up      # = docker compose up -d --build
make docker-down
make docker-logs
```

---

## 网络与端口

Docker Compose 使用 `network_mode: host`（**仅 Linux 生效**），所以：

- **面板端口**由 `.env` 里的 `PORT` 控制（默认 8080），直接绑定在 VPS 上。
- **代理端口**由面板托管的 sing-box 动态监听，同样直接绑定在 VPS 上。

因为用的是 host 网络，无需在 compose 里写端口映射；对外访问只需在**云厂商安全组/防火墙**里放行面板端口和各代理端口即可。

> 注意：host 网络在 macOS/Windows 的 Docker Desktop 上不生效，端口不会按预期暴露。本地验证请用 `make run` 直接跑单二进制。

---

## 固定 sing-box 版本

- Docker：在 `.env` 里设置 `SING_BOX_IMAGE`（如 `ghcr.io/sagernet/sing-box:v1.11.4`）。
- 原生部署：设置环境变量 `SINGBOX_VERSION`（如 `1.13.13`）后再执行 `make deploy-native`。
