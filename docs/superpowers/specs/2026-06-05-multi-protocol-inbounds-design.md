# M4: 多协议入站（协议注册表）+ 默认 seed — 设计文档

- 日期：2026-06-05
- 状态：设计待用户确认
- 里程碑：M4（把单协议入站改造成多协议；新增 Hysteria2；部署默认 seed 两个入站）

## 1. 背景与目标

M3 的入站只支持 VLESS+Reality，且 reality 字段直接做成了表列。M4 把入站改造成**多协议、可扩展**：用「协议注册表（driver）」承载每种协议的默认生成、配置生成、展示信息；新增 **Hysteria2**；部署后**默认 seed** 一个 VLESS-Reality 和一个 Hysteria2 入站（不带用户）。加新协议 = 写一个 driver，不改表结构、不改生成器主流程。

关键决策（已确认）：
- 模型：`Inbound{type, tag, port, network, settings(JSON)}` + 协议注册表。
- Hysteria2 TLS：面板生成**自签 ECDSA 证书**（零依赖；客户端需允许 insecure）。
- 默认 seed：两个入站，**不建默认用户**。
- 限速：Hysteria2 带 `up_mbps`/`down_mbps`（创建可填，有默认）。
- 默认端口：**vless-reality = TCP 8443**，**hysteria2 = UDP 443**；端口冲突按 **(port, network)** 判断（TCP/UDP 同号可共存）。443 预留给后续 nginx 里程碑代理面板/订阅，本里程碑不锁死、端口可改。

### 范围内
- 协议 driver 抽象 + 注册表；vless-reality 与 hysteria2 两个 driver。
- 重构 Inbound/User 模型（type+settings JSON；User.Credential）。
- 自签证书生成；配置生成器走注册表；端口冲突按 (port, network)。
- 默认 seed（幂等）。
- 端点：创建 body 改为 `{type, tag, port, params}`；新增 `GET /api/inbound-types`。
- 前端：协议下拉 + type-aware 表单 + 按类型展示 publicInfo / 用户凭证标签。

### 范围外（后续里程碑）
- nginx 反代面板 + 订阅链接（独立里程碑；届时 nginx 取 TCP 443）。
- vless:// / hysteria2:// 分享链接、订阅 URL。
- 更多协议（shadowsocks/tuic 等）——但本里程碑后加它们只需写 driver。
- 入站编辑（仍是新建/删除）；ACME 证书。

## 2. 协议注册表（`internal/inbound`）

```go
type Driver interface {
    Type() string                                    // "vless-reality" | "hysteria2"
    Label() string                                   // 下拉显示名
    Network() string                                 // "tcp" | "udp"（端口冲突维度）
    BuildSettings(params map[string]any) (string, error) // 生成密钥/证书 + 应用 params/默认 → settings JSON
    NewCredential() string                           // 新用户凭证：vless→uuid，hy2→随机密码
    BuildInbound(in models.Inbound) (map[string]any, error) // 产出 sing-box 入站片段（读 settings + users）
    PublicInfo(in models.Inbound) map[string]any     // 仅可公开展示字段（无私钥/证书私钥）
}

// 注册表：map[type]Driver；init 注册 vlessReality、hysteria2
func Get(t string) (Driver, bool)
func Types() []TypeInfo // [{Type,Label,Network}]，给前端下拉 + 默认端口提示
```
`BuildSettings(nil)` = 纯默认（用于 seed）。

## 3. 数据模型（重构）

```go
type Inbound struct {
    ID        uint      `json:"id"`
    Tag       string    `gorm:"uniqueIndex" json:"tag"`
    Type      string    `json:"type"`
    Network   string    `json:"network"`   // tcp|udp，由 driver 决定
    Port      uint16    `json:"port"`
    Settings  string    `json:"-"`         // 协议私有 JSON（含 reality 私钥 / 证书私钥），不下发
    Users     []User    `gorm:"constraint:OnDelete:CASCADE" json:"-"` // 不直接序列化 model；见 InboundView
    CreatedAt, UpdatedAt time.Time
}
type User struct {
    ID         uint   `json:"id"`
    InboundID  uint   `json:"inboundId"`
    Name       string `json:"name"`
    Credential string `json:"credential"` // uuid 或密码
    CreatedAt, UpdatedAt time.Time
}
```
- **私密不下发**：`Settings` `json:"-"`。列表接口返回 `InboundView{id,type,tag,port,network,publicInfo,users}`（model 不直接序列化；DTO 显式带 publicInfo + 预加载的 users，前端按 M3 优化从中直接渲染、不再每卡再拉）。publicInfo 由 driver 给，无私钥；用户凭证客户端要用，照常返回。
- **迁移**：预发布期结构性改表（去 reality 专用列、User.UUID→Credential）。AutoMigrate 加新列但不删旧列/不迁数据；建议重建数据库或全新部署。seed 仅 inbounds 为空时跑。

## 4. 两个 driver

### vless-reality（network=tcp）
- `BuildSettings`：生成 reality 密钥对（复用 M3 `KeyGen`，curve25519）+ shortId；默认 `handshake=www.microsoft.com, handshakePort=443, serverName=handshake, flow=xtls-rprx-vision`；params 可覆盖 `handshake`。settings={realityPrivateKey,realityPublicKey,shortId,handshake,handshakePort,serverName,flow}。
- `NewCredential`=uuid。`BuildInbound`=M3 的 vless+reality 片段。`PublicInfo`={realityPublicKey,shortId,serverName,flow}。

### hysteria2（network=udp）
- `BuildSettings`：用 Go 生成自签 ECDSA 证书(certPEM,keyPEM)；默认 `serverName=bing.com, upMbps=100, downMbps=100`；params 可覆盖 `serverName,upMbps,downMbps`。settings={serverName,certPEM,keyPEM,upMbps,downMbps}。
- `NewCredential`=随机密码（16 字节 base64url）。
- `BuildInbound`：`{type:"hysteria2",tag,listen:"::",listen_port,up_mbps,down_mbps,users:[{name,password}],tls:{enabled:true,server_name,alpn:["h3"],certificate:[certPEM],key:[keyPEM]}}`（up/down 为 0 时省略）。
- `PublicInfo`={serverName,upMbps,downMbps,insecure:true}（提示客户端需允许 insecure / 固定指纹）。

证书生成放 `internal/inbound/cert.go`：`selfSignedCert(sni) (certPEM, keyPEM string, err error)`，ECDSA P-256，有效期长（如 10 年），CN/SAN=sni。

## 5. 配置生成器

`Generate(inbounds []models.Inbound) (string, error)`：`log` + 遍历 inbounds → `Get(in.Type).BuildInbound(in)` 拼 `inbounds[]` + `direct` outbound → `json.MarshalIndent`。未知 type 返回错误。任何入站/用户增删仍 `Regenerate()` 自动落盘。

## 6. 端点（均 authed，沿用 M3 风格）

| 方法/路径 | 变化 |
|---|---|
| `GET /api/inbound-types` | **新增**：返回 `[{type,label,network,defaultPort}]` 给前端下拉 |
| `GET /api/inbounds` | 返回 `InboundView[]`（含 publicInfo + 预加载 users，无 settings） |
| `POST /api/inbounds` | body 改 `{type, tag, port, params}`；服务端 driver.BuildSettings(params) 生成 settings |
| `DELETE /api/inbounds/:id` | 不变 |
| `GET/POST /api/inbounds/:id/users`、`DELETE /api/users/:id` | 不变；用户凭证由对应 driver.NewCredential() 生成 |
| `POST /api/singbox/apply` | 不变 |

创建校验：type 必须在注册表；tag 非空且唯一（trim）；端口冲突按 (port, network)。错误码：未知 type→400，tag 空→400，tag 重复→409，端口占用→409。

## 7. 默认 seed

启动 AutoMigrate 后调用 `SeedDefaults(svc)`：若 inbounds 为空，创建：
- `vless-reality`（tag=`vless-reality`，TCP 8443，BuildSettings(nil)）。
- `hysteria2`（tag=`hysteria2`，UDP 443，BuildSettings(nil)）。
均不带用户。幂等：非空则跳过。seed 走 service.CreateInbound（同样触发 Regenerate）。

## 8. 前端

- **新建入站**：协议下拉（拉 `/api/inbound-types`）；选 vless-reality 显示 `握手域名`（默认 www.microsoft.com）、端口默认 8443；选 hysteria2 显示 `SNI`（默认 bing.com）、`上行Mbps`、`下行Mbps`、端口默认 443。提交 `{type,tag,port,params}`。
- **入站卡片**：标题显示 `tag · TYPE · :port/network`；按 type 展示 publicInfo（vless：公钥/shortId/SNI/flow；hy2：SNI/上下行/“insecure”提示）；用户区凭证标签按类型（UUID / 密码）。
- 文案全中文；沿用深色 B 端 shell。

## 9. 测试

- 后端：
  - `cert`：自签证书可被 `x509` 解析、SAN 含 sni。
  - vless driver：BuildSettings 生成可解码密钥；BuildInbound 结构断言；PublicInfo 不含私钥。
  - hysteria2 driver：BuildSettings 生成证书 + 默认/覆盖 up/down；BuildInbound 断言 type/tls/alpn/up_mbps/users[].password；PublicInfo 不含 keyPEM。
  - registry：Get/Types。
  - Service：CreateInbound 按 type 建并 Regenerate；未知 type→错误；端口冲突按 (port,network)（tcp/udp 同号可建，同 network 同号报错）；CreateUser 凭证按类型；ListInboundViews 不含 settings。
  - SeedDefaults：空表建两个、非空跳过（幂等）。
  - handlers：inbound-types、create(type/params)、错误码。
- 前端：类型下拉渲染、按类型字段切换、按类型建入站调用、卡片按类型展示、用户凭证标签。

## 10. 风险与说明
- 自签证书 → 客户端需 insecure/固定指纹，publicInfo 明示。
- 端口冲突仅按 (port, network)；同一 network 内仍需手动避免撞 nginx（443 已留给 nginx）。
- 结构性改表，预发布期建议重建库。
- nginx 反代面板+订阅为后续里程碑；本设计已让 443 可空出（vless 默认 8443、hy2 用 UDP 443）。
