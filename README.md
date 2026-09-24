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
| `DB_PATH` | `sing-box-admin.db` | SQLite 数据库路径（启动时会被收紧为 0600） |
| `JWT_SECRET` | 随机生成 | 签名会话 Cookie 的密钥；**生产必须设置**，否则每次重启会话都会失效 |

## 启动与路由行为

- 面板启动时，如果已生成配置且面板管理的 sing-box 未运行，会自动启动它；配置错误、端口冲突或二进制缺失时，面板只记录错误并继续启动，方便从 UI 排查。
- 给用户分配出站后，路由顺序为：私网地址直连 → 中国大陆域名/IP 直连 → 其余目的地走该用户的出站。未分配出站的用户仍全部直连。
- 中国大陆识别使用官方 SagerNet 远程 `geosite-cn` 与 `geoip-cn` 二进制规则集，sing-box 每 7 天经直连更新一次；国内域名 DNS 也优先走本地/direct 解析。首次启动需要 VPS 能访问 GitHub Raw 下载规则集。
- 复制的订阅配置面向 Mihomo / Clash.Meta：配置显式使用 `mode: rule`，客户端先按 `GEOSITE,CN,DIRECT` 与 `GEOIP,CN,DIRECT` 直连国内目标，因此国内网站会看到客户端本机公网 IP；其余目标和外部 DNS 走「节点」到 VPS，失败时不会回退直连。订阅还启用加密 DNS、fake-ip、严格 TUN 路由并拒绝 IPv6，以减少应用绕过系统代理造成的泄露；客户端未授权 TUN、应用排除或关闭代理时无法由订阅单独兜底。
- 一个用户绑定多个入站时，订阅会生成「自动选择」和「故障切换」组；面板的「上游线路检测」只测 VPS 到出站服务器的 TCP 建连延迟，不等于客户端上传或 UDP 质量。
- 入站表单由后端协议 schema 驱动，目前支持 VLESS-Reality、Hysteria2、Trojan、AnyTLS；AnyTLS 需要 sing-box 1.12.0+。删除仍被用户使用的出站会被拒绝，必须先改派用户，防止意外回落到直连。

| `DEFAULT_ADMIN_USER` | `admin` | 初始管理员用户名（仅首次、表为空时写入） |
| `DEFAULT_ADMIN_PASS` | `mnice7082` | 初始管理员密码 |
| `COOKIE_SECURE` | `false` | 会话 Cookie 是否带 `Secure`；**面板走 HTTPS 时必须开** |
| `TRUSTED_PROXIES` | 空（谁都不信） | 可信反代列表（逗号分隔），影响登录限流所依据的客户端 IP |
| `SERVER_HOST` | 请求 Host | 生成订阅时用的公网域名/IP |

首次启动会自动创建默认管理员账号 `admin` / `mnice7082`。**这个默认密码写在本仓库里，等于公开**，登录后请立刻在右上角账户菜单 →「修改密码」改掉；改密会同时让其它设备上已登录的会话失效。

登录接口带失败限流：同一 IP 连续 5 次失败后锁定 15 分钟（此时即使密码正确也会被拒），以挡暴力破解和 bcrypt 造成的 CPU 耗尽。

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

### 需要开放的端口

在安全组/防火墙里放行以下端口（按实际用到的协议开）：

| 端口 | 协议 | 用途 |
| --- | --- | --- |
| `8080`（或你设的 `PORT`） | TCP | 面板 Web 界面 |
| `4443` | TCP | 默认 vless-reality 入站 |
| `8443` | UDP | 默认 hysteria2 入站 |

其中 `4443` / `8443` 是首次启动自动创建的默认入站端口；如果你在面板里改了端口或新增了入站，请按面板里实际的端口和协议放行。

> **不要**对外开放 `9090`——那是 sing-box 的 Clash API（`external_controller`），只绑定在 `127.0.0.1`，供面板本机读取流量统计，暴露出去有安全风险。

> 注意：host 网络在 macOS/Windows 的 Docker Desktop 上不生效，端口不会按预期暴露。本地验证请用 `make run` 直接跑单二进制。

---

## 常见问题

**日志反复出现 `traffic poll: Get "http://127.0.0.1:9090/connections": connection refused`**

流量统计轮询器会去读 sing-box 的 Clash API（`127.0.0.1:9090`）。只有**面板自己启动**的 sing-box 才会被轮询，所以这条报错意味着面板启动的那个进程没有在 9090 上提供 Clash API——通常是配置还没重新生成/应用。到面板里点「应用并重启」即可。

**概览页提示「检测到一个不由面板管理的 sing-box 进程」**

说明机器上另有一个不是面板启动的 sing-box。面板**不会**去启停它（早先版本会用 `pkill` 杀掉本机所有 sing-box，现已移除），但它占着代理端口，面板启动自己的实例时会因端口被占而立刻退出——这种情况现在会直接报错并附上 sing-box 日志尾部，而不是假装启动成功。先手动停掉那个进程，再用面板启动。

---

## 固定 sing-box 版本

- Docker：在 `.env` 里设置 `SING_BOX_IMAGE`（如 `ghcr.io/sagernet/sing-box:v1.11.4`）。
- 原生部署：设置环境变量 `SINGBOX_VERSION`（如 `1.13.13`）后再执行 `make deploy-native`。

**用户页的「采样流量」为什么不是精确计费流量？**

sing-box 的 Clash API 会实时推送整机吞吐，因此概览页「实时吞吐」是精确的当前全局速率。按用户维度时，该 API 只给出**当前仍在线的连接**的累计字节，没有已关闭连接的事件或用户总计数器；面板每秒采样一次在线连接并每 5 秒批量写入数据库，所以极短连接（在两次采样之间建立并关闭）可能未被观测到。该数值适合运维观察，不应用于精确计费或强制流量配额。
