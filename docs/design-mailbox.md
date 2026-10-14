# 邮箱（Gmail）模块设计方案

> 状态：**已决策，实施中**。2026-09-29 拍板：
> - **D1 深度**：L1 只读收件箱（列表/详情/搜索/附件下载，scope=gmail.readonly）
> - **D2 授权**：CLI 辅助授权（`--gmail-auth` 子命令：本地 loopback 回调换取 refresh token；
>   Google 已弃用 OOB 模式，采用 127.0.0.1 临时回调 server）
> - 附件转存（L2）与发件（L3）后置，架构已预留
> - **2026-09-29 追加**：模块定位为「Gmail 邮箱」；抽出 `internal/mail`
>   通用 Provider 注册表支持多供应商，**Outlook 后期接入**（页内已有占位 tab）

## 0. 关键前提（先读）

| 事项 | 说明 | 影响 |
|---|---|---|
| **仅支持 OAuth 2.0** | Gmail API 不支持账号密码；需要 Google Cloud 项目 + OAuth Client（Client ID/Secret）+ Refresh Token（离线授权） | 首次授权需要人工在浏览器走一次 Google 同意页；之后 refresh token 长期有效 |
| **网络可达性** | `google.googleapis.com` 国内不可直连（本机已实测超时） | 本地开发需 `HTTPS_PROXY` 代理；部署 Cloudflare/海外无碍 |
| **配额限制** | Gmail API 每用户 250 quota units/秒；`messages.list/get` 各 5 units | 正常浏览远够用；列表建议缓存 60s |
| **单账号站点** | 应用不开放注册，Gmail 即「站点自己的邮箱」 | 与网盘/书库一致：凭据配置式，不做多邮箱切换 |

## 1. 功能定位

「邮箱」作为资源中心的第 4 个上线模块：**收件 + 附件即资源**。

```
邮箱
├── 收件箱：列表（发件人/主题/摘要/时间/附件标识）、分页、Gmail 语法搜索（from: is:unread 等）
├── 邮件详情：正文（文本/HTML）、发件人/收件人、附件列表
├── 附件动作：下载 / ⭐ 转存到对象存储（与书库同构：邮件附件 → R2/本地书仓）
├── 通知/验证码提取（可选）：高亮正则提取的验证码，方便注册网盘账号场景
└── 发件（可选档位）：纯文本/HTML 发信、回复
```

> 与主线的契合点：网盘注册、资源站验证、分享链接通知大量走邮件；附件转存直接复用
> 书库已落地的 ObjectStore 链路（R2 未配置时本地兜底），是投入产出比最高的联动。

## 2. 对接深度（决策点 D1）

| 档位 | 能力 | 工作量 | 备注 |
|---|---|---|---|
| L1 只读收件箱 | 列表 + 详情 + 搜索 + 附件下载 | 小 | 只读 scope，授权心智最轻 |
| **L2 + 附件转存**（推荐） | L1 + 附件一键转存 R2/本地 | 中 | 复用 ObjectStore 与转存记录模式，主线联动 |
| L3 + 发件 | L2 + 发信/回复 | 大 | 需要 compose scope；发送语义要处理配额与草稿 |

## 3. OAuth 授权方式（决策点 D2）

| 方式 | 流程 | 评价 |
|---|---|---|
| **CLI 辅助授权（推荐）** | `./resource-center --gmail-auth` 打印 Google 授权 URL → 浏览器登录同意 → 回贴 authorization code → 程序换取 refresh token 写入配置 | 一次性成本 5 分钟；实现小（标准库可完成 code exchange） |
| OAuth Playground 手动 | Google 官方工具生成 refresh token 后填环境变量 | 零代码但步骤繁、易出错，对非开发用户不友好 |
| Web 引导页 | 应用内做完整 OAuth 回调流程 | 体验最好但需注册回调域名、处理 state/安全，重 |

> 无论哪种，`GMAIL_CLIENT_ID` / `GMAIL_CLIENT_SECRET` / `GMAIL_REFRESH_TOKEN` 三个
> 环境变量为最终形态；未配置时模块走 Mock（固定演示收件箱 + 附件，转存链路可完整演示）。

## 4. 架构设计

### 4.1 对接层 `internal/gmail/`

```go
type Client struct {          // gmail.go
    clientID, clientSecret, refreshToken string
    http *http.Client          // 支持 HTTPS_PROXY
    // token 缓存 + 惰性刷新（access_token ~1h）
}
// 核心方法（Gmail API v1，users/me 前缀）：
// ListMessages(q, pageToken)   GET /gmail/v1/users/me/messages?q=&maxResults=&pageToken=
// GetMessage(id)               GET .../messages/{id}?format=full（含附件元数据）
// GetAttachment(msgID, attID)  GET .../messages/{id}/attachments/{attId}（base64 数据）
// SendMessage(mime)            POST .../messages/send（L3）
```

- 邮件解析：RFC 2822 MIME 头（From/To/Subject 含中文编码）用标准库 `net/mail` +
  `mime/multipart` 解析；纯标准库，不引依赖
- `internal/book` 的 Mock 同款：`MockMailbox` 固定收件箱数据

### 4.2 API（登录墙内）

```
GET  /api/v1/mail/status                     授权状态 / Mock 标识
GET  /api/v1/mail/messages?q=&pageToken=     列表
GET  /api/v1/mail/messages/{id}              详情（含附件清单）
GET  /api/v1/mail/attachment/{msgID}/{attID} 附件下载（流式代理）
POST /api/v1/mail/transfer                   附件转存对象存储（复用 ObjectStore）
POST /api/v1/mail/send                       发件（L3）
```

### 4.3 数据层

不新增业务表：转存记录复用 `book_records` 的结构语义，新增轻表 `mail_records`
（用户隔离，字段：消息ID/附件名/大小/状态/R2Key），与书库记录页并列展示。

### 4.4 前端

- `MODULES` 登记「邮箱」+ `#/mailbox` 路由
- 布局：双栏（左列表 + 右详情），顶栏搜索框（支持 Gmail 搜索语法提示）
- 附件卡片：下载 / ⬆️ 转存（转存后进入「转存记录」统一查看）
- Mock 模式固定演示数据：2-3 封邮件（网盘注册验证码邮件 + 带附件的资源通知）

## 5. 风险与对策

| 风险 | 对策 |
|---|---|
| 国内网络不可达 | 开发用 `HTTPS_PROXY`；部署侧 Cloudflare/海外出口；文档明确标注 |
| refresh token 失效（撤销/过期） | `/mail/status` 常态探活，失效时页面顶部黄条提示重新授权 |
| MIME 解析边界（嵌套 multipart、奇怪编码） | 标准库 parser 为主 + 兜底原文展示；解析器纯函数可单测 |
| 邮件内容属敏感数据 | 应用自身不落库正文，仅内存转发；转存只针对用户点选的附件 |
| API 配额 | 列表响应缓存 60s；详情不缓存 |

## 6. 实施拆分（选定后）

1. `internal/gmail/`：Client（OAuth token 刷新 + List/Detail/Attachment）+ MIME 解析 + Mock
2. CLI 辅助授权：`--gmail-auth` 子命令（生成授权 URL / code 换 refresh token）
3. `/api/v1/mail/*` handlers + 路由 + 转存复用 ObjectStore
4. 前端 `#/mailbox` 双栏页 + 模块卡片
5. Mock 全链路联调（收件→附件转存→下载）+ 浏览器验证 + README
