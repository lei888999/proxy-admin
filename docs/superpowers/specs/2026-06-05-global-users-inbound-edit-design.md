# M5: 全局用户 + 入站编辑 + Modal UX — 设计文档

- 日期：2026-06-05
- 状态：设计待用户确认
- 里程碑：M5（用户改为全局多对多、入站可编辑+重置密钥、新建/编辑走 modal）

## 1. 背景与目标

M4 的用户内嵌在入站下（has-many）。M5 把用户改成**全局实体**：一个用户可挂到多个入站（类 3x-ui client）；新增独立「用户管理」页。同时入站支持**编辑**（改标签/端口/协议参数，保留密钥）与**重置密钥**；入站新建/编辑改为 **modal** 弹窗，协议选择用 **shadcn Select** 组件。

关键决策（已确认）：
- 用户：全局 `User{Name,UUID,Password}` + `User⇄Inbound` 多对多；新建用户时多选入站。
- 用户列表**明文显示密码**（管理员自有面板）。
- 入站编辑：改 tag/port/协议参数，**type 不可改**，密钥默认保留；另给「重置密钥」按钮，**二次确认**。
- 入站卡片**不再显示**关联用户（用户在 /users 管）。
- 协议选择用 shadcn Select 组件（`npx shadcn add select`）。

### 范围内
- 数据模型：User 全局 + 多对多；config 生成按协议取凭证。
- 后端：User CRUD + reset；Inbound update + reset-keys；driver 增 `CredentialKind/UpdateSettings/ResetSecrets`。
- 前端：`/users` 页（modal 增删改 + 重置凭证 + 显密码）；`/inbounds` 改 modal 新建/编辑 + 重置密钥确认 + 去掉内联用户；侧边栏加「用户」；轻量 Modal 组件 + shadcn Select。

### 范围外（后续）
- nginx 反代 / 订阅链接 / 分享链接；流量统计；用户到期/限额。

## 2. 数据模型（再次重构 users）

```go
type Inbound struct { // M4 字段不变，关系改 m2m
	ID uint; Tag string `gorm:"uniqueIndex"`; Type, Network string; Port uint16
	Settings string `json:"-"`
	Users    []User `gorm:"many2many:user_inbounds;" json:"-"`
	timestamps
}
type User struct {
	ID       uint      `json:"id"`
	Name     string    `gorm:"not null" json:"name"`
	UUID     string    `gorm:"not null" json:"uuid"`     // 供 vless 入站
	Password string    `gorm:"not null" json:"password"` // 供 hy2 入站（前端明文显示）
	Inbounds []Inbound `gorm:"many2many:user_inbounds;" json:"-"`
	timestamps
}
```
- join 表 `user_inbounds`。删用户 → GORM 清关联 + 删行；删入站 → 解除该入站的关联（用户保留）。
- **迁移**：又一次结构性改表（User 去掉 InboundID/Credential，加 UUID/Password/m2m）。预发布期重建库。seed 仍只建入站不建用户。

## 3. driver 增量

```go
CredentialKind() string                              // "uuid"（vless）| "password"（hy2）
UpdateSettings(existing string, params map[string]any) (string, error) // 保留密钥，仅改可编辑参数
ResetSecrets(existing string) (string, error)        // 重新生成密钥/证书，保留参数
```
- vless：UpdateSettings 改 handshake（serverName=handshake），保留 reality 密钥/shortId/flow；ResetSecrets 新 reality 密钥+shortId，保留 handshake。CredentialKind="uuid"。
- hy2：UpdateSettings 改 serverName/up/down，保留证书；ResetSecrets 新自签证书，保留 serverName/up/down。CredentialKind="password"。

## 4. 配置生成（按协议取凭证）

每个入站收集其关联用户，按 `driver.CredentialKind()` 选字段：
```
for each inbound:
  d = Get(inbound.Type)
  creds = []Cred{}
  for u in inbound.Users:
      cred = if d.CredentialKind()=="password" { u.Password } else { u.UUID }
      creds += Cred{Name:u.Name, Credential:cred}
  piece = d.BuildInbound(tag,port,settings,creds)
```
其余生成逻辑不变。任何 user/inbound 增删改 → Regenerate。

## 5. HTTP 端点（authed）

用户（新）：
| 方法/路径 | 说明 | 错误码 |
|---|---|---|
| `GET /api/users` | 列出用户：`{id,name,uuid,password,inboundIds[],inboundTags[]}` | — |
| `POST /api/users` | 建用户 `{name, inboundIds[]}`；服务端生成 uuid+password | 400 名称空 |
| `PUT /api/users/:id` | 改 `{name, inboundIds[]}`（替换关联） | 404 |
| `DELETE /api/users/:id` | 删用户 | 404 |
| `POST /api/users/:id/reset` | 重生成 uuid+password | 404 |

入站（改/增）：
| `PUT /api/inbounds/:id` | `{tag, port, params}`；type 不可改；保留密钥 | 400 标签空/未知；409 tag 重复/端口冲突；404 |
| `POST /api/inbounds/:id/reset-keys` | 重置 reality 密钥/证书 | 404 |

移除 M3/M4 的内联用户端点：`GET/POST /api/inbounds/:id/users`、`DELETE /api/users/:id`(旧语义) → 由 `/api/users` 取代。`GET /api/inbounds` 的 `InboundView` 去掉 `users` 字段（入站卡片不再展示用户）。

## 6. 后端 service

- `ListUserViews() []UserView`（含 inboundIds/inboundTags）。
- `CreateUser(name string, inboundIDs []uint)`：gen uuid+password，create，`Association("Inbounds").Replace(...)`，Regenerate。
- `UpdateUser(id, name, inboundIDs)`：更新名 + Replace 关联，Regenerate。
- `DeleteUser(id)`：删（清关联），Regenerate。
- `ResetUserCreds(id)`：新 uuid+password，Regenerate。
- `UpdateInbound(id, tag, port, params)`：取 driver，`UpdateSettings(existing,params)`，校验 tag 唯一(排除自身)、(port,network) 冲突(排除自身)，保存，Regenerate。
- `ResetInboundKeys(id)`：`driver.ResetSecrets(existing)`，保存，Regenerate。
- 关联预加载用 `Preload("Inbounds")`（用户侧）与 `Preload("Users")`（入站生成侧）。

## 7. 前端

- 侧边栏导航：`概览 / 入站 / 用户 / 配置`。
- **轻量 Modal 组件** `components/modal.tsx`（深色、点遮罩/Esc 关闭），自写不引第三方。
- **shadcn Select**：`npx shadcn@latest add select`，协议选择用之；编辑入站时禁用（type 不可改）。
- **入站页**：新建/编辑都开 modal（协议 Select + type-aware 字段：vless 握手域名 / hy2 SNI+上下行）。卡片每项有「编辑」「重置密钥」「删除」；重置密钥弹**二次确认** modal。卡片只展示 publicInfo（不展示用户）。
- **用户页** `/users`：表格（名称、UUID、**明文密码**、所属入站标签）；「新建用户」modal（名称 + 入站多选 checkbox 列表）；行操作「编辑」(同 modal)、「重置凭证」、「删除」。
- `lib/api.ts` 增：users CRUD + reset、inbound update + resetKeys、`InboundType` 复用；`Inbound` 去掉 users。

## 8. 测试

- 后端：User CRUD + 多对多关联（建用户挂 2 入站、改关联、删用户清 join、删入站保用户）；生成器对 vless 取 uuid、对 hy2 取 password；`UpdateSettings` 改参数保密钥、`ResetSecrets` 换密钥保参数；`UpdateInbound` 校验(改 tag 重复→409、改端口冲突→409、排除自身可改回原值)；handlers 各状态码。
- 前端：用户页（列表显密码、新建多选入站调用、删除、重置）、入站 modal（新建/编辑 type 字段切换、编辑时协议只读、重置密钥二次确认后才调用）。

## 9. 风险与说明
- 又一次改表，预发布期重建库。
- 密码明文展示与下发是管理员面板的预期信任模型（reality 私钥/证书私钥仍 `json:"-"` 不下发）。
- 重置密钥/凭证会使已分发的旧客户端失效，前端二次确认提示。
