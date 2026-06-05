# M3: 入站/用户管理（VLESS+Reality）— 设计文档

- 日期：2026-06-05
- 状态：设计待用户确认
- 里程碑：M3（可视化入站 + 用户管理 + 配置自动生成 + 应用重启）

## 1. 背景与目标

M2 让面板能启停 sing-box、编辑原始 config.json。M3 让管理员**可视化管理入站与用户**：面板维护 inbound + user 数据，自动生成 sing-box 配置，不再手写 JSON。第一版只支持 **VLESS + Reality**（当下最主流、抗封锁最好）。

关键决策（已确认）：
- **仅 VLESS+Reality**；其它协议后续里程碑。
- **入站/用户是配置真相源**：config.json 由 DB 的 inbounds+users 自动生成；M2 的原始 JSON 编辑器降级为**只读预览**。
- **密钥/标识全部在 Go 内生成**（curve25519 + crypto/rand），不调用 sing-box 二进制——纯函数、可测、无运行时依赖。

### 范围内
- 数据模型 Inbound / User（GORM）。
- 配置生成器：inbounds+users → 完整 sing-box config（含固定的 log/outbounds 脚手架）。
- Reality 密钥对 / UUID / short_id 生成。
- CRUD 端点 + 应用重启端点。
- 前端：入站页（增删 + 用户管理 + 只读连接参数展示）、概览加「应用并重启」、配置页改只读。

### 范围外（后续里程碑）
- 其它协议（Shadowsocks / Hysteria2 等）。
- 入站编辑（v1 仅新建/删除，改 = 删后重建）。
- vless:// 分享链接 / 订阅 URL（下一个里程碑）。
- 流量统计、用户限额/到期。

## 2. 总体架构

```
前端(深色 B 端 shell) ──HTTP(cookie 鉴权)──> Gin handlers
  ├ GET/POST /api/inbounds, DELETE /api/inbounds/:id
  ├ GET/POST /api/inbounds/:id/users, DELETE /api/users/:id
  └ POST /api/singbox/apply
        │
        v
  internal/inbound.Service ── GORM(SQLite: inbounds, users)
        ├─ KeyGen (Go): reality keypair / uuid / short_id
        ├─ Generate(inbounds) → config JSON (纯函数)
        └─ ConfigWriter.Save(json)  (复用 M2 singbox.ConfigStore)
  应用：handler → inbound.Service.Regenerate() → singbox.Service.Restart()(停+起)
```

**自动生成**：任何 inbound/user 增删后，Service 立即调用 `Regenerate()`（从 DB 重建 config.json）。因此「配置」预览与概览的 `hasConfig` 始终与数据一致。**应用并重启**才让运行中的 sing-box 重新加载。

## 3. 数据模型（`internal/models`）

```go
type Inbound struct {
    ID                uint   `gorm:"primaryKey"`
    Tag               string `gorm:"uniqueIndex;not null"` // 入站标签，配置 tag
    Port              uint16 `gorm:"not null"`             // listen_port
    Flow              string `gorm:"not null"`             // 默认 "xtls-rprx-vision"
    RealityPrivateKey string `gorm:"not null"`
    RealityPublicKey  string `gorm:"not null"`             // 供客户端使用
    RealityShortID    string `gorm:"not null"`             // 8 位 hex
    Handshake         string `gorm:"not null"`             // 握手目标域名，如 www.microsoft.com
    HandshakePort     uint16 `gorm:"not null"`             // 默认 443
    ServerName        string `gorm:"not null"`             // SNI，= Handshake
    Users             []User `gorm:"constraint:OnDelete:CASCADE"`
    CreatedAt, UpdatedAt time.Time
}

type User struct {
    ID        uint   `gorm:"primaryKey"`
    InboundID uint   `gorm:"index;not null"`
    Name      string `gorm:"not null"`
    UUID      string `gorm:"not null"`
    CreatedAt, UpdatedAt time.Time
}
```
`AutoMigrate(&Admin{}, &Inbound{}, &User{})`。删除 inbound 级联删除其 users。

## 4. 配置生成器（`internal/inbound/generate.go`，纯函数）

`Generate(inbounds []models.Inbound) (string, error)`，每个 inbound 产出一个 sing-box VLESS+Reality 入站：

```json
{
  "log": { "level": "info" },
  "inbounds": [
    {
      "type": "vless",
      "tag": "<Tag>",
      "listen": "::",
      "listen_port": <Port>,
      "users": [ { "name": "<User.Name>", "uuid": "<User.UUID>", "flow": "<Flow>" } ],
      "tls": {
        "enabled": true,
        "server_name": "<ServerName>",
        "reality": {
          "enabled": true,
          "handshake": { "server": "<Handshake>", "server_port": <HandshakePort> },
          "private_key": "<RealityPrivateKey>",
          "short_id": ["<RealityShortID>"]
        }
      }
    }
  ],
  "outbounds": [ { "type": "direct", "tag": "direct" } ]
}
```
- 用结构体 + `encoding/json` 构建（不用字符串拼接），保证合法 JSON。
- 无 inbound 时仍生成 `inbounds: []` 的合法骨架（但概览启动需有 inbound 才有意义；前端可提示）。
- 生成结果经 `ConfigWriter.Save`（M2 ConfigStore，做 JSON 校验 + 原子写）落盘。

## 5. 密钥/标识生成（`internal/inbound/keygen.go`，纯 Go）

定义 `KeyGen` 接口（便于 mock）：
```go
type KeyGen interface {
    RealityKeypair() (privateKey, publicKey string, err error) // curve25519, base64
    UUID() string       // v4
    ShortID() string    // 8 位 hex
}
```
- 真实现 `goKeyGen`：私钥 = 32 随机字节（curve25519 clamp），公钥 = `curve25519.X25519(priv, basepoint)`；按 sing-box reality 约定 base64 编码（与 `sing-box generate reality-keypair` 输出格式一致）。UUID 用 crypto/rand 拼 v4。ShortID = 4 随机字节 hex。

## 6. HTTP 端点（均挂 RequireAuth）

| 方法/路径 | 说明 | 错误码 |
|---|---|---|
| `GET /api/inbounds` | 列出入站（含 users 概要/计数） | — |
| `POST /api/inbounds` | 建入站；body: `{tag,port,handshake}`（缺省 flow/handshakePort/serverName/keys 服务端补全） | 400 校验失败；409 tag 重复 |
| `DELETE /api/inbounds/:id` | 删入站（级联删 user） | 404 不存在 |
| `GET /api/inbounds/:id/users` | 列出某入站用户 | 404 |
| `POST /api/inbounds/:id/users` | 加用户；body: `{name}`，uuid 服务端生成 | 400；404 入站不存在 |
| `DELETE /api/users/:id` | 删用户 | 404 |
| `POST /api/singbox/apply` | 重新生成配置 + 重启 sing-box，返回最新 status | 透传 singbox 错误码（未安装 400 等） |

每次成功的增删都会触发 `Regenerate()`。`apply` 重启用 `singbox.Service.Restart()`（停-忽略未运行-再起）；需新增 `Restart()`。

`POST /api/inbounds` 创建时：生成 reality 密钥对+short_id+（uuid 在加用户时生成）；`Flow` 默认 `xtls-rprx-vision`，`HandshakePort` 默认 443，`ServerName` = `Handshake`。

## 7. 前端（深色 B 端 shell，全中文）

- 侧边栏导航加「入站」：`概览 / 入站 / 配置`。
- **入站页** `/inbounds`：
  - 入站列表（标签、端口、用户数、删除按钮）。
  - 「新建入站」表单：标签、端口、握手域名（默认值 `www.microsoft.com`）。提交后服务端生成密钥。
  - 选中入站 → 用户区：用户列表（名称、UUID、删除）、「添加用户」（输入名称）。
  - 入站详情展示**只读连接参数**（端口、Reality 公钥、short_id、SNI、flow）与每个用户的 UUID，供管理员手配客户端。
- **概览页**：加「应用并重启」按钮（调 `/api/singbox/apply`），操作后刷新状态。
- **配置页** `/config`：改为只读预览（移除保存按钮与编辑），展示生成的 config.json。
- `lib/api.ts` 新增：`listInbounds/createInbound/deleteInbound/listUsers/createUser/deleteUser/applySingbox`。

## 8. 错误处理
- 端口/标签校验：tag 非空唯一、port 1–65535；重复 tag → 409。
- 删除不存在 → 404。
- 生成失败/写盘失败 → 500。
- apply：透传 singbox 启动错误（未安装/无 inbound 导致空配置可启动但无意义 → 由 sing-box check 决定；若 check 失败返回 400+detail）。

## 9. 测试
- **后端**
  - `Generate`：给定 inbounds（含多用户）→ 解析结果 JSON 断言 type=vless、listen_port、users[].uuid/flow、tls.reality.private_key/short_id/handshake；空 inbounds → 合法骨架。
  - `KeyGen`：UUID 形如 v4、ShortID 8 hex、Reality 公钥由固定私钥确定性派生（注入随机源或断言长度/base64 可解）。
  - `inbound.Service`（内存 sqlite + fake ConfigWriter + fake KeyGen）：建入站写入并触发 Regenerate、tag 重复报错、加/删用户后 Regenerate、删入站级联删用户。
  - handlers（httptest + fake service）：覆盖第 6 节状态码。
- **前端**（Vitest）：api 新方法；入站页列表渲染/新建调用/删除、用户增删、应用按钮调用 applySingbox；配置页只读（无保存按钮）。

## 10. 风险与说明
- 以 root 运行管理员自定义入站属预期信任模型。
- v1 不校验端口冲突（多个入站撞端口由 sing-box check/启动暴露）；可后续加。
- Reality 公钥需正确对应私钥编码，单测以确定性派生保证；上线首个入站后建议真机连一次验证。
